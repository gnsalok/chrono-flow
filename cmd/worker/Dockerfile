# worker/Dockerfile

# --- Build Stage ---
FROM golang:1.19-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the worker binary.
RUN CGO_ENABLED=0 go build -o /bin/worker ./cmd/worker

# --- Final Stage ---
FROM alpine:latest

# Copy the compiled binary from the builder stage.
COPY --from=builder /bin/worker /bin/worker

# Expose the gRPC port the worker listens on.
EXPOSE 50051

# The command to run when the container starts.
ENTRYPOINT ["/bin/worker"]
