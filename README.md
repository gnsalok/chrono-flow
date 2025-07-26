# Chrono-Flow: A Distributed, Fault-Tolerant Job Scheduler

Chrono-Flow is a lightweight, distributed, and fault-tolerant task scheduler built with Go. It is designed to run scheduled jobs (like cron jobs) in a cloud-native environment, providing high availability through the Raft consensus algorithm.

## Features

- **Distributed & Fault-Tolerant:** The controller runs as a cluster. If the leader node fails, a new leader is automatically elected, ensuring no downtime.
- **REST API:** A simple and clean RESTful API for managing jobs (Create, Read, Update, Delete).
- **Flexible Scheduling:** Supports standard cron expressions (e.g., `*/5 * * * *` for every 5 minutes).
- **Multiple Job Types:** Currently supports executing shell commands on worker nodes.
- **Scalable Workers:** You can add more worker nodes to scale out job execution capacity.
- **Containerized:** The entire system can be deployed and run with a single `docker-compose` command.

## Tech Stack

- **Language:** Go/Golang
- **Backend Framework:** Gorilla/Mux for the REST API
- **RPC:** gRPC for Controller-Worker communication
- **Distributed Consensus:** HashiCorp `Raft` for leader election
- **Database:** `PostgreSQL` for storing job definitions and execution logs
- **Containerization:** Docker & Docker Compose

## Getting Started

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Docker Compose](https://docs.docker.com/compose/)

### Running the System

Clone the repository:

```bash
git clone https://github.com/gnsalok/chrono-flow.git
cd chrono-flow
```

**Important:** Open `docker-compose.yml` and change the `POSTGRES_PASSWORD` from `yourpassword` to a secure password. Make sure to update it for all controller services as well.

Build and run the services:

```bash
docker-compose up --build
```

This command will build the Docker images for the controller and worker, and then start all the services: a 3-node controller cluster, one worker, and a PostgreSQL database.

You will see logs from all services. Look for one of the controllers to announce `This node became the LEADER`.

## API Usage

You can interact with the cluster via the REST API on port 8080.

### Create a New Job

Create a job that prints the date every 10 seconds:

```bash
curl -X POST http://localhost:8080/jobs \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Date Printer",
    "schedule": "*/10 * * * * *",
    "command": "echo \"The current date is $(date)\""
}'
```

You should see the output of this job in the `chrono-worker-1` logs every 10 seconds.

### List All Jobs

```bash
curl http://localhost:8080/jobs
```

### Test Fault Tolerance

1. Find the container ID of the leader node (check the logs for the "became the LEADER" message). Let's assume it's `chrono-controller-1`.
2. Stop the leader node:

   ```bash
   docker stop chrono-controller-1
   ```

3. Watch the logs of the other controllers (`chrono-controller-2` or `chrono-controller-3`). You will see one of them announce that it has become the new leader.

The scheduled job will continue to execute without interruption!

### Clean Up
To stop and remove all containers, run:

```bash
docker-compose down
```


## Lead Maintainer
- [Alok Tripathi](https://github.com/gnsalok)