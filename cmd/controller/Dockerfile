# controller/Dockerfile

# --- Build Stage ---
# Use the official Golang image as a builder.
FROM golang:1.23-alpine AS builder

# Set the working directory inside the container.
WORKDIR /app

# Copy go.mod and go.sum files to download dependencies first.
# This leverages Docker's layer caching.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code.
COPY . .

# Build the controller binary.
# CGO_ENABLED=0 is important for creating a static binary that works in a minimal Alpine image.
# -o /bin/controller specifies the output path for the compiled binary.
RUN CGO_ENABLED=0 go build -o /bin/controller ./cmd/controller

# --- Final Stage ---
# Use a minimal Alpine image for the final container to keep it small and secure.
FROM alpine:latest

# Copy the compiled binary from the builder stage.
COPY --from=builder /bin/controller /bin/controller

# Expose the port the API server will run on.
EXPOSE 8080
# Expose the port for Raft communication.
EXPOSE 12000

# The command to run when the container starts.
ENTRYPOINT ["/bin/controller"]
