// cmd/controller/main.go
package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	raft_node "github.com/gnsalok/chrono-flow/internal/raft" // Aliased to avoid conflict
	"github.com/gnsalok/chrono-flow/internal/scheduler"
	"github.com/gnsalok/chrono-flow/internal/store"
	"github.com/gorilla/mux"
)

// APIServer represents the API server for the controller.
type APIServer struct {
	listenAddr string
	store      *store.Store
	raftNode   *raft_node.Node // Add a reference to the Raft node
}

// NewAPIServer creates a new APIServer instance.
func NewAPIServer(listenAddr string, store *store.Store, raftNode *raft_node.Node) *APIServer {
	return &APIServer{
		listenAddr: listenAddr,
		store:      store,
		raftNode:   raftNode,
	}
}

// Run starts the HTTP server. Note that this is a blocking call.
func (s *APIServer) Run() {
	router := mux.NewRouter()

	// API routes for managing jobs
	router.HandleFunc("/jobs", s.handleGetJobs).Methods("GET")
	router.HandleFunc("/jobs", s.handleCreateJob).Methods("POST")
	router.HandleFunc("/jobs/{id}", s.handleGetJob).Methods("GET")
	router.HandleFunc("/jobs/{id}", s.handleUpdateJob).Methods("PUT")
	router.HandleFunc("/jobs/{id}", s.handleDeleteJob).Methods("DELETE")

	// Add the new endpoint for joining the cluster
	router.HandleFunc("/join", s.handleJoin).Methods("POST")

	log.Println("Controller API server starting on", s.listenAddr)
	if err := http.ListenAndServe(s.listenAddr, router); err != nil {
		log.Fatalf("Failed to start API server: %v", err)
	}
}

// handleJoin processes a request from a new node to join the cluster.
func (s *APIServer) handleJoin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NodeID   string `json:"nodeId"`
		RaftAddr string `json:"raftAddr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if err := s.raftNode.Join(req.NodeID, req.RaftAddr); err != nil {
		log.Printf("Failed to handle join request: %v", err)
		// This could be because this node is not the leader.
		// The client is expected to retry.
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully handled join request for node %s at %s", req.NodeID, req.RaftAddr)
	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// ... (keep all the handle... functions exactly as they were) ...
// handleGetJobs retrieves all jobs.
func (s *APIServer) handleGetJobs(w http.ResponseWriter, r *http.Request) {
	jobs, err := s.store.GetJobs()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, jobs)
}

// handleCreateJob creates a new job.
func (s *APIServer) handleCreateJob(w http.ResponseWriter, r *http.Request) {
	var job store.Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
		return
	}

	id, err := s.store.CreateJob(job.Name, job.Schedule, job.Command)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	job.ID = id
	writeJSON(w, http.StatusCreated, job)
}

// handleGetJob retrieves a single job by its ID.
func (s *APIServer) handleGetJob(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid job ID"})
		return
	}

	job, err := s.store.GetJob(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if job == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Job not found"})
		return
	}
	writeJSON(w, http.StatusOK, job)
}

// handleUpdateJob updates an existing job.
func (s *APIServer) handleUpdateJob(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid job ID"})
		return
	}

	var job store.Job
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid request payload"})
		return
	}

	err = s.store.UpdateJob(id, job.Name, job.Schedule, job.Command, job.IsEnabled)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

// handleDeleteJob deletes a job.
func (s *APIServer) handleDeleteJob(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid job ID"})
		return
	}

	err = s.store.DeleteJob(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// writeJSON is a helper function to write JSON responses.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func main() {
	// --- Configuration Flags ---
	listenAddr := flag.String("listenaddr", ":8080", "The API server listen address")
	dbConnStr := flag.String("dbconn", "user=postgres password=yourpassword dbname=chrono_flow sslmode=disable", "PostgreSQL connection string")
	workerAddr := flag.String("workeraddr", "localhost:50051", "The address of a worker node")

	raftNodeID := flag.String("raft-id", "", "Node ID for Raft. Must be unique in the cluster.")
	raftAddr := flag.String("raft-addr", "localhost:12000", "Address for Raft communication.")
	raftDir := flag.String("raft-dir", "/tmp/chrono-flow-raft", "Directory for Raft's log and snapshot storage.")
	raftJoinAddr := flag.String("raft-join-addr", "", "HTTP address of a peer to join an existing cluster (e.g. http://localhost:8080).")
	bootstrap := flag.Bool("bootstrap", false, "Bootstrap the cluster. Use for the very first node only.")
	flag.Parse()

	if *raftNodeID == "" {
		log.Fatal("-raft-id is required")
	}

	// --- Database Connection ---
	dbStore, err := store.NewStore(*dbConnStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("Successfully connected to the database")

	// --- Raft Node Initialization ---
	raftNode, err := raft_node.NewNode(*raftNodeID, *raftAddr, *raftDir)
	if err != nil {
		log.Fatalf("Failed to create raft node: %v", err)
	}

	if *bootstrap {
		log.Println("Bootstrapping cluster...")
		raftNode.BootstrapCluster()
	} else if *raftJoinAddr != "" {
		// Start a goroutine that will repeatedly try to join the cluster
		// via the specified peer's HTTP API.
		go func() {
			for {
				log.Printf("Attempting to join cluster via peer at %s", *raftJoinAddr)
				err := raftNode.JoinCluster(*raftJoinAddr)
				if err == nil {
					log.Println("Successfully joined cluster.")
					break // Exit the loop on success
				}
				log.Printf("Failed to join cluster: %v. Retrying in 5 seconds...", err)
				time.Sleep(5 * time.Second)
			}
		}()
	}

	// --- Scheduler and API Server ---
	var sched *scheduler.Scheduler

	// Pass the raft node to the API server
	apiServer := NewAPIServer(*listenAddr, dbStore, raftNode)
	go apiServer.Run()

	// This loop listens for leadership changes and starts/stops the scheduler accordingly.
	for isLeader := range raftNode.LeaderCh() {
		if isLeader {
			log.Println("This node became the LEADER. Starting scheduler...")
			sched = scheduler.NewScheduler(dbStore, *workerAddr)
			sched.Start()
		} else {
			if sched != nil {
				log.Println("This node is now a FOLLOWER. Stopping scheduler...")
				sched.Stop()
				sched = nil
			}
		}
	}

	// Wait for a shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
}
