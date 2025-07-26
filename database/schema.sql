-- database/schema.sql

-- This table stores the definitions of the jobs that need to be run.
CREATE TABLE IF NOT EXISTS jobs (
    -- A unique identifier for the job, generated automatically.
    id SERIAL PRIMARY KEY,
    -- A user-friendly name for the job.
    name VARCHAR(255) NOT NULL,
    -- The cron expression that defines the job's schedule (e.g., '*/5 * * * *').
    schedule VARCHAR(100) NOT NULL,
    -- The shell command that the job will execute.
    command TEXT NOT NULL,
    -- A flag to enable or disable the job without deleting it.
    is_enabled BOOLEAN DEFAULT TRUE,
    -- Timestamp for when the job was created.
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Timestamp for when the job was last updated.
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- This table stores the history of every execution for every job.
CREATE TABLE IF NOT EXISTS job_executions (
    -- A unique identifier for this specific execution.
    id SERIAL PRIMARY KEY,
    -- A foreign key linking this execution to a job in the 'jobs' table.
    -- If a job is deleted, all its execution records are also deleted.
    job_id INTEGER NOT NULL REFERENCES jobs(id) ON DELETE CASCADE,
    -- The standard output captured from the command.
    output_stdout TEXT,
    -- The standard error captured from the command.
    output_stderr TEXT,
    -- The exit code of the command. 0 usually indicates success.
    exit_code INTEGER,
    -- The status of the execution (e.g., 'success', 'failed').
    status VARCHAR(50),
    -- The timestamp for when the execution was scheduled to start.
    scheduled_at TIMESTAMPTZ NOT NULL,
    -- The timestamp for when the execution actually started.
    started_at TIMESTAMPTZ,
    -- The timestamp for when the execution finished.
    finished_at TIMESTAMPTZ
);

-- Create an index on job_id for faster lookups of a job's execution history.
CREATE INDEX IF NOT EXISTS idx_job_executions_job_id ON job_executions(job_id);
-- Create an index on status for faster querying of executions by their status.
CREATE INDEX IF NOT EXISTS idx_job_executions_status ON job_executions(status);
