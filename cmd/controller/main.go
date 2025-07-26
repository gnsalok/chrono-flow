// cmd/controller/main.go
package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"strconv"

	"github.com/gnsalok/chrono-flow/internal/store"
	"github.com/gorilla/mux"
)

// APIServer represents the API server for the controller.
type APIServer struct {
	listenAddr string
	store      *store.Store
}

// NewAPIServer creates a new APIServer instance.
func NewAPIServer(listenAddr string, store *store.Store) *APIServer {
	return &APIServer{
		listenAddr: listenAddr,
		store:      store,
	}
}

// Run starts the HTTP server and sets up the API routes.
func (s *APIServer) Run() {
	router := mux.NewRouter()

	// API routes for managing jobs
	router.HandleFunc("/jobs", s.handleGetJobs).Methods("GET")
	router.HandleFunc("/jobs", s.handleCreateJob).Methods("POST")
	router.HandleFunc("/jobs/{id}", s.handleGetJob).Methods("GET")
	router.HandleFunc("/jobs/{id}", s.handleUpdateJob).Methods("PUT")
	router.HandleFunc("/jobs/{id}", s.handleDeleteJob).Methods("DELETE")

	log.Println("Controller API server listening on", s.listenAddr)
	http.ListenAndServe(s.listenAddr, router)
}

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
	// Command-line flags for configuration
	listenAddr := flag.String("listenaddr", ":8080", "The API server listen address")
	dbConnStr := flag.String("dbconn", "user=postgres password=yourpassword dbname=chrono_flow sslmode=disable", "PostgreSQL connection string")
	flag.Parse()

	// Initialize the data store
	dbStore, err := store.NewStore(*dbConnStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	log.Println("Successfully connected to the database")

	// Create and run the API server
	server := NewAPIServer(*listenAddr, dbStore)
	server.Run()
}
