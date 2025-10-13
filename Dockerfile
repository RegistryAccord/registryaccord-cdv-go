# Build stage
FROM golang:1.25-alpine AS builder

# Install git (needed for go mod download with private repos)
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o cdvd cmd/cdvd/main.go

# Final stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Create non-root user
RUN adduser -D -s /bin/sh cdv

# Set working directory
WORKDIR /app

# Copy the binary from builder stage
COPY --from=builder /app/cdvd .

# Copy config files if they exist
COPY --from=builder /app/.env.example .env.example

# Change ownership to non-root user
RUN chown -R cdv:cdv /app

# Switch to non-root user
USER cdv

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8080/healthz || exit 1

# Run the binary
ENTRYPOINT ["./cdvd"]
