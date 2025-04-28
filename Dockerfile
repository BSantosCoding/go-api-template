# ---- Builder Stage ----
# Use a specific Go version with Alpine base for smaller size
FROM golang:1.23-alpine AS builder

# Install build dependencies for Go tools (like migrate, air, swag)
# git is needed for go install
# build-base is needed for CGO if any dependencies require it
RUN apk add --no-cache git build-base

# Set the working directory inside the container
WORKDIR /app

# Copy go module files first to leverage Docker cache
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application source code
COPY . .

# Mark the directory as safe for git operations
RUN git config --global --add safe.directory /app

# Install development tools (air, swag, migrate)
# These are installed in the builder stage, migrate CLI will be copied to final stage
RUN go install -buildvcs=false github.com/air-verse/air@latest
RUN go install -buildvcs=false github.com/swaggo/swag/cmd/swag@latest
RUN go install -buildvcs=false -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

# Generate Swagger docs before building
# Ensure swag annotations are correct before this step
RUN swag init -g main.go

# Build the Go application
# -ldflags="-w -s" strips debug information and symbols for a smaller binary
# CGO_ENABLED=0 disables CGO for static linking (usually good for Alpine)
RUN CGO_ENABLED=0 go build -buildvcs=false -ldflags="-w -s" -o /app/main .


# ---- Runtime Stage ----
# Use a minimal Alpine image for the final stage
FROM alpine:latest

# Install runtime dependencies
# ca-certificates for HTTPS requests
# tzdata for timezone support (if needed by your app)
# postgresql-client provides psql and is needed by migrate CLI
RUN apk add --no-cache ca-certificates tzdata postgresql-client

# Set the working directory
WORKDIR /app

# Copy the compiled application binary from the builder stage
COPY --from=builder /app/main /app/main

# Copy the migrate CLI binary from the builder stage
# Assumes GOPATH is default /go in builder stage
COPY --from=builder /go/bin/migrate /usr/local/bin/migrate
COPY --from=builder /go/bin/air /usr/local/bin/air

# Copy configuration files (if needed, though env vars are preferred)
COPY config.yaml ./config.yaml

# Copy migration files (needed if running migrations from within the container)
COPY internal/database/migrations ./internal/database/migrations

# Expose the port the application runs on (ensure this matches your config/env var)
EXPOSE 8080

# Command to run the application
# This will be overridden by docker-compose for development (to use Air)
CMD ["/app/main"]

