# Dockerfile for MapReduce Boss (Go)
FROM golang:1.25-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git

# Set working directory
WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY *.go ./
COPY pb/ ./pb/

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o boss .

# Final stage - minimal image
FROM alpine:latest

# Install ca-certificates for HTTPS
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/boss .

# Expose gRPC port
EXPOSE 8080

# Set environment variables
ENV VOLUME_PATH=/volume

# Run the boss
CMD ["./boss"]
