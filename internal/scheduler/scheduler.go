// internal/scheduler/scheduler.go
package scheduler

import (
	"context"
	"log"
	"time"

	pb "github.com/gnsalok/chrono-flow/internal/proto"
	"github.com/gnsalok/chrono-flow/internal/store"
	"github.com/robfig/cron/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Scheduler manages all the scheduled jobs.
type Scheduler struct {
	store *store.Store
	cron  *cron.Cron
	// In a real-world scenario, this would be a dynamic list of workers.
	// For now, we'll hardcode one for simplicity.
	workerAddr string
}

// NewScheduler creates and initializes a new Scheduler.
func NewScheduler(store *store.Store, workerAddr string) *Scheduler {
	return &Scheduler{
		store: store,
		// Create a new cron scheduler that uses seconds-level precision.
		cron:       cron.New(cron.WithSeconds()),
		workerAddr: workerAddr,
	}
}

// Start begins the scheduler's operations. It loads jobs from the database
// and starts the cron daemon.
func (s *Scheduler) Start() {
	log.Println("Starting scheduler...")
	if err := s.loadAndScheduleJobs(); err != nil {
		log.Fatalf("Failed to load jobs on startup: %v", err)
	}
	s.cron.Start()
	log.Println("Scheduler started successfully.")
}

// Stop gracefully stops the cron scheduler.
func (s *Scheduler) Stop() {
	log.Println("Stopping scheduler...")
	s.cron.Stop()
	log.Println("Scheduler stopped.")
}

// loadAndScheduleJobs fetches all enabled jobs from the database and adds them
// to the cron scheduler.
func (s *Scheduler) loadAndScheduleJobs() error {
	jobs, err := s.store.GetEnabledJobs()
	if err != nil {
		return err
	}

	for _, job := range jobs {
		// Using a closure to capture the job variable correctly for each iteration.
		// If we didn't do this, all cron jobs would run with the reference to the last job in the loop.
		j := job
		_, err := s.cron.AddFunc(j.Schedule, func() {
			s.runJob(j)
		})
		if err != nil {
			log.Printf("Error scheduling job ID %d (%s): %v", j.ID, j.Name, err)
		} else {
			log.Printf("Successfully scheduled job ID %d (%s) with schedule: %s", j.ID, j.Name, j.Schedule)
		}
	}
	return nil
}

// runJob is the function that gets executed when a cron schedule is met.
func (s *Scheduler) runJob(job *store.Job) {
	log.Printf("Executing job ID %d (%s): %s", job.ID, job.Name, job.Command)

	// In a real system, you'd have logic to select a worker.
	// For now, we connect to our single, hardcoded worker.
	conn, err := grpc.Dial(s.workerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("Failed to connect to worker for job ID %d: %v", job.ID, err)
		// Here you would log the execution failure to the database.
		return
	}
	defer conn.Close()

	client := pb.NewTaskServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute) // 1-minute timeout for the task
	defer cancel()

	// Create and send the task request.
	req := &pb.TaskRequest{
		Id:      "some-unique-execution-id", // In a real system, generate a unique ID and save it.
		Command: job.Command,
	}

	res, err := client.Execute(ctx, req)
	if err != nil {
		log.Printf("Failed to execute task for job ID %d: %v", job.ID, err)
		// Log failure to job_executions table
		return
	}

	log.Printf("Job ID %d finished. Exit Code: %d, Stdout: %s", job.ID, res.ExitCode, res.Stdout)
	// Log success to job_executions table
}
