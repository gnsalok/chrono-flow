// internal/store/store.go
package store

import (
	"database/sql"
	"time"

	// We are using the pq driver for PostgreSQL.
	// The blank import is used because the driver registers itself with the database/sql package.
	_ "github.com/lib/pq"
)

// Job represents the structure of a job in our database.
// It maps directly to the 'jobs' table schema.
type Job struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Schedule  string    `json:"schedule"`
	Command   string    `json:"command"`
	IsEnabled bool      `json:"is_enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Store provides an interface for all database operations.
type Store struct {
	db *sql.DB
}

// NewStore creates a new Store with a database connection.
func NewStore(dataSourceName string) (*Store, error) {
	db, err := sql.Open("postgres", dataSourceName)
	if err != nil {
		return nil, err
	}
	// Ping the database to verify the connection is alive.
	if err := db.Ping(); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// CreateJob inserts a new job into the database.
func (s *Store) CreateJob(name, schedule, command string) (int64, error) {
	var id int64
	query := `INSERT INTO jobs (name, schedule, command) VALUES ($1, $2, $3) RETURNING id`
	err := s.db.QueryRow(query, name, schedule, command).Scan(&id)
	return id, err
}

// GetJob retrieves a single job by its ID.
func (s *Store) GetJob(id int64) (*Job, error) {
	job := &Job{}
	query := `SELECT id, name, schedule, command, is_enabled, created_at, updated_at FROM jobs WHERE id = $1`
	err := s.db.QueryRow(query, id).Scan(&job.ID, &job.Name, &job.Schedule, &job.Command, &job.IsEnabled, &job.CreatedAt, &job.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil // Return nil, nil if no job is found.
	}
	return job, err
}

// GetJobs retrieves all jobs from the database.
func (s *Store) GetJobs() ([]*Job, error) {
	query := `SELECT id, name, schedule, command, is_enabled, created_at, updated_at FROM jobs`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		job := &Job{}
		err := rows.Scan(&job.ID, &job.Name, &job.Schedule, &job.Command, &job.IsEnabled, &job.CreatedAt, &job.UpdatedAt)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// GetEnabledJobs retrieves all jobs that are currently enabled.
func (s *Store) GetEnabledJobs() ([]*Job, error) {
	query := `SELECT id, name, schedule, command, is_enabled, created_at, updated_at FROM jobs WHERE is_enabled = TRUE`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		job := &Job{}
		err := rows.Scan(&job.ID, &job.Name, &job.Schedule, &job.Command, &job.IsEnabled, &job.CreatedAt, &job.UpdatedAt)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, nil
}

// UpdateJob updates an existing job in the database.
func (s *Store) UpdateJob(id int64, name, schedule, command string, isEnabled bool) error {
	query := `UPDATE jobs SET name = $1, schedule = $2, command = $3, is_enabled = $4, updated_at = NOW() WHERE id = $5`
	_, err := s.db.Exec(query, name, schedule, command, isEnabled, id)
	return err
}

// DeleteJob removes a job from the database.
func (s *Store) DeleteJob(id int64) error {
	query := `DELETE FROM jobs WHERE id = $1`
	_, err := s.db.Exec(query, id)
	return err
}
