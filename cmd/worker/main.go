// cmd/worker/main.go
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os/exec"
	"syscall"

	// Import the generated gRPC code.
	pb "github.com/gnsalok/chrono-flow/internal/proto"
	"google.golang.org/grpc"
)

// server struct will implement the TaskServiceServer interface generated from our .proto file.
type server struct {
	// UnimplementedTaskServiceServer must be embedded to have forward compatible implementations.
	pb.UnimplementedTaskServiceServer
}

// Execute is the implementation of the RPC method defined in our tasks.proto.
// It receives a task request from the controller, executes it, and returns the result.
func (s *server) Execute(ctx context.Context, in *pb.TaskRequest) (*pb.TaskResponse, error) {
	log.Printf("Received task execution request for ID: %s, Command: %s", in.Id, in.Command)

	// Use exec.Command to prepare the command. We use "sh -c" to properly handle
	// complex shell commands with pipes, redirections, etc.
	cmd := exec.Command("sh", "-c", in.Command)

	// Execute the command and capture its combined standard output and standard error.
	output, err := cmd.CombinedOutput()

	// Prepare the response message.
	response := &pb.TaskResponse{
		Id:     in.Id,
		Stdout: string(output), // CombinedOutput includes both stdout and stderr.
		Stderr: "",             // We'll populate this based on the exit code.
	}

	// Check the result of the execution.
	if err != nil {
		// If there was an error, it could be an execution error (non-zero exit code)
		// or an error starting the command.
		if exitError, ok := err.(*exec.ExitError); ok {
			// The command ran but returned a non-zero exit code.
			log.Printf("Command finished with a non-zero exit code: %v", err)
			response.ExitCode = int32(exitError.ExitCode())
			// We can separate the stderr if we want, but for now, CombinedOutput is fine.
			response.Stderr = string(exitError.Stderr)
		} else {
			// An error occurred trying to run the command (e.g., command not found).
			log.Printf("Failed to execute command: %v", err)
			response.ExitCode = -1 // Use a special exit code for execution failures.
			response.Stderr = err.Error()
		}
	} else {
		// The command executed successfully (exit code 0).
		ws := cmd.ProcessState.Sys().(syscall.WaitStatus)
		response.ExitCode = int32(ws.ExitStatus())
		log.Printf("Command executed successfully for ID: %s", in.Id)
	}

	return response, nil
}

func main() {
	// Set up command-line flags to configure the worker's port.
	// This allows us to run multiple workers on different ports.
	port := flag.Int("port", 50051, "The server port")
	flag.Parse()

	// Create a TCP listener on the specified port.
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Create a new gRPC server instance.
	s := grpc.NewServer()

	// Register our server implementation with the gRPC server.
	pb.RegisterTaskServiceServer(s, &server{})

	log.Printf("Worker server listening at %v", lis.Addr())

	// Start serving incoming gRPC requests.
	// This is a blocking call, so the program will run until it's terminated.
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
