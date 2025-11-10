# MapReduce Containerization Guide

## Overview
This guide provides complete instructions for containerizing the MapReduce system with Docker Compose. The setup includes:
- 1 Boss/Client/Frontend container (complete control plane with web UI)
- 3 Worker containers (compute nodes)
- Access the web UI at `http://localhost:3000`

## Architecture

```
┌─────────────────────────────────────────────┐
│              Host Machine                    │
│         Browser → localhost:3000             │
└─────────────────────────────────────────────┘
                    │
           ┌────────▼──────────┐
           │ Boss + Client +   │
           │   Web Frontend    │
           │ :8080 :8081 :3000 │
           └────────┬──────────┘
                    │
    ┌───────────────┼───────────────────┐
    │               │                   │
┌───▼──────┐ ┌─────▼──────┐ ┌─────────▼────┐
│ Worker-1 │ │  Worker-2  │ │   Worker-3   │
│  :50051  │ │   :50052   │ │    :50053    │
└──────────┘ └────────────┘ └──────────────┘
```

## Project Structure

```
MapReduce/
├── docker-compose.yml
├── boss/
│   ├── Dockerfile
│   ├── main.go
│   └── (boss source files)
├── client/
│   ├── main.go
│   └── (client source files)
├── worker/
│   ├── Dockerfile
│   ├── main.go
│   └── (worker source files)
├── pb/
│   └── (protobuf definitions)
├── frontend/
│   ├── package.json
│   ├── src/
│   └── (React app - runs on host)
└── volume/
    └── (shared data directory)
```

## Dockerfiles

### 1. Boss/Client/Frontend Dockerfile (`Dockerfile.controller`)

```dockerfile
# Stage 1: Build Go binaries
FROM golang:1.21-alpine AS go-builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy protobuf files (already generated)
COPY pb/ ./pb/

# Copy source code
COPY boss/ ./boss/
COPY client/ ./client/

# Build both binaries
RUN CGO_ENABLED=0 GOOS=linux go build -o /boss ./boss
RUN CGO_ENABLED=0 GOOS=linux go build -o /client ./client

# Stage 2: Build React frontend
FROM node:18-alpine AS frontend-builder

WORKDIR /app

# Copy package files
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

# Copy frontend source
COPY frontend/ ./

# Build production bundle
RUN npm run build

# Stage 3: Final runtime image
FROM alpine:latest

RUN apk --no-cache add ca-certificates nginx supervisor

# Copy binaries from go-builder
COPY --from=go-builder /boss /usr/local/bin/boss
COPY --from=go-builder /client /usr/local/bin/client

# Copy frontend build from frontend-builder
COPY --from=frontend-builder /app/dist /usr/share/nginx/html

# Create nginx config for React app
RUN cat > /etc/nginx/nginx.conf << 'EOF'
events {
    worker_connections 1024;
}

http {
    include /etc/nginx/mime.types;
    default_type application/octet-stream;

    server {
        listen 3000;
        server_name localhost;
        
        root /usr/share/nginx/html;
        index index.html;
        
        # Serve static files
        location / {
            try_files $uri $uri/ /index.html;
        }
        
        # Proxy WebSocket connections to client
        location /ws {
            proxy_pass http://localhost:8081;
            proxy_http_version 1.1;
            proxy_set_header Upgrade $http_upgrade;
            proxy_set_header Connection "upgrade";
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
        }
        
        # Proxy API requests to client
        location /api/ {
            proxy_pass http://localhost:8081;
            proxy_set_header Host $host;
            proxy_set_header X-Real-IP $remote_addr;
        }
    }
}
EOF

# Create supervisor config to run all services
RUN cat > /etc/supervisord.conf << 'EOF'
[supervisord]
nodaemon=true
logfile=/var/log/supervisord.log

[program:boss]
command=/usr/local/bin/boss
autostart=true
autorestart=true
stderr_logfile=/var/log/boss.err.log
stdout_logfile=/var/log/boss.out.log

[program:client]
command=/usr/local/bin/client
autostart=true
autorestart=true
environment=BOSS_ADDR="localhost:8080",VOLUME_PATH="/volume"
stderr_logfile=/var/log/client.err.log
stdout_logfile=/var/log/client.out.log

[program:nginx]
command=nginx -g 'daemon off;'
autostart=true
autorestart=true
stderr_logfile=/var/log/nginx.err.log
stdout_logfile=/var/log/nginx.out.log
EOF

# Create volume directory
RUN mkdir -p /volume

# Expose ports
EXPOSE 8080 8081 3000

# Start supervisor to manage all processes
CMD ["/usr/bin/supervisord", "-c", "/etc/supervisord.conf"]
```

### 2. Worker Dockerfile (`Dockerfile.worker`)

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy protobuf files (already generated)
COPY pb/ ./pb/

# Copy worker source
COPY worker/ ./worker/

# Build worker binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /worker ./worker

# Final stage with Python support
FROM python:3.11-alpine

RUN apk --no-cache add ca-certificates

# Copy worker binary from builder
COPY --from=builder /worker /worker

# Install Python packages for MapReduce jobs
RUN pip install --no-cache-dir numpy pandas

# Create volume directory
RUN mkdir -p /volume

# Set environment variables
ENV WORKER_ID=""
ENV BOSS_ADDR="boss:8080"
ENV VOLUME_PATH="/volume"

# Run worker
CMD /worker
```

## Docker Compose Configuration

### `docker-compose.yml`

```yaml
version: '3.8'

services:
  boss-client:
    build:
      context: .
      dockerfile: Dockerfile.controller
    container_name: mapreduce-controller
    ports:
      - "3000:3000"  # React Frontend
      - "8080:8080"  # Boss gRPC server (optional, for debugging)
      - "8081:8081"  # Client WebSocket/REST API (optional, for debugging)
    volumes:
      - ./volume:/volume
    environment:
      - VOLUME_PATH=/volume
    networks:
      - mapreduce-net
    healthcheck:
      test: ["CMD", "wget", "--spider", "-q", "http://localhost:3000"]
      interval: 10s
      timeout: 5s
      retries: 3

  worker-1:
    build:
      context: .
      dockerfile: Dockerfile.worker
    container_name: mapreduce-worker-1
    environment:
      - WORKER_ID=worker-1
      - BOSS_ADDR=boss-client:8080
      - VOLUME_PATH=/volume
    volumes:
      - ./volume:/volume
    networks:
      - mapreduce-net
    depends_on:
      - boss-client
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G

  worker-2:
    build:
      context: .
      dockerfile: Dockerfile.worker
    container_name: mapreduce-worker-2
    environment:
      - WORKER_ID=worker-2
      - BOSS_ADDR=boss-client:8080
      - VOLUME_PATH=/volume
    volumes:
      - ./volume:/volume
    networks:
      - mapreduce-net
    depends_on:
      - boss-client
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G

  worker-3:
    build:
      context: .
      dockerfile: Dockerfile.worker
    container_name: mapreduce-worker-3
    environment:
      - WORKER_ID=worker-3
      - BOSS_ADDR=boss-client:8080
      - VOLUME_PATH=/volume
    volumes:
      - ./volume:/volume
    networks:
      - mapreduce-net
    depends_on:
      - boss-client
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 2G

networks:
  mapreduce-net:
    driver: bridge

volumes:
  mapreduce-data:
```

## Code Modifications Required

### 1. Boss (`boss/main.go`)

Current code listens on hardcoded `localhost:8080`. Update to:

```go
func main() {
    addr := os.Getenv("BOSS_ADDR")
    if addr == "" {
        addr = "0.0.0.0:8080"  // Listen on all interfaces in container
    }
    
    lis, err := net.Listen("tcp", addr)
    // ... rest of the code
}
```

### 2. Client (`client/main.go`)

Current code has hardcoded addresses. Update to:

```go
func main() {
    clientID := fmt.Sprintf("client-%d", time.Now().UnixNano())
    
    bossAddr := os.Getenv("BOSS_ADDR")
    if bossAddr == "" {
        bossAddr = "localhost:8080"  // Default for local development
    }
    
    // For WebSocket server, bind to all interfaces
    log.Println("Starting WebSocket server on :8081")
    if err := http.ListenAndServe("0.0.0.0:8081", nil); err != nil {
        log.Printf("WebSocket server failed: %v", err)
    }
}
```

### 3. Worker (`worker/main.go`)

Update worker to use environment variables:

```go
func main() {
    workerID := os.Getenv("WORKER_ID")
    if workerID == "" {
        workerID = fmt.Sprintf("worker-%d", time.Now().UnixNano())
    }
    
    bossAddr := os.Getenv("BOSS_ADDR")
    if bossAddr == "" {
        bossAddr = "localhost:8080"
    }
    
    volumePath := os.Getenv("VOLUME_PATH")
    if volumePath == "" {
        volumePath = "/volume"
    }
    
    // Connect to boss
    conn, err := grpc.Dial(bossAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
    // ... rest of the code
}
```

### 4. Frontend Updates

Since the frontend is now served from within the container via nginx, we need to update the WebSocket and API URLs to use relative paths:

```typescript
// src/hooks/useWebSocket.ts
// Use relative path so it works through nginx proxy
const WS_URL = `ws://${window.location.host}/ws`;

// src/components/StartJob.tsx
// Use relative path for API calls
const API_URL = '/api';

// Update the fetch calls to use relative paths:
const response = await fetch('/api/submit-job', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({...})
});

const response = await fetch('/api/upload-file', {
  method: 'POST',
  body: formData,
});
```

## Running the System

### 1. Start the containers:

```bash
# Build and start all containers
docker-compose up --build

# Or run in background
docker-compose up -d --build

# Scale workers if needed (note: you'll need to adjust docker-compose.yml)
docker-compose up -d --scale worker=5
```

### 2. Access the web interface:

Open your browser and navigate to: **`http://localhost:3000`**

The complete MapReduce dashboard will be available with:
- System overview with worker status
- Job submission interface
- File upload capability
- Real-time monitoring of jobs and tasks

### 3. Verify the system:

```bash
# Check container status
docker-compose ps

# View logs
docker-compose logs -f boss-client
docker-compose logs -f worker-1

# Check network connectivity
docker exec mapreduce-worker-1 ping boss-client
```

## Volume Structure

The shared volume (`./volume` on host, `/volume` in containers) should contain:

```
volume/
├── input/          # Input files for MapReduce jobs
│   ├── data1.csv
│   └── data2.json
├── output/         # Job output files
│   └── job-xxx/
├── code/           # MapReduce Python scripts
│   ├── wordcount.py
│   └── custom_job.py
└── temp/           # Temporary files
```

## Sample MapReduce Job

Place this in `volume/code/wordcount.py`:

```python
import sys
import json

def map_function(key, value):
    """Map function for word count"""
    for word in value.split():
        yield (word.lower(), 1)

def reduce_function(key, values):
    """Reduce function for word count"""
    return (key, sum(values))

# MapReduce runner will call these functions
```

## Troubleshooting

### Common Issues:

1. **Workers can't connect to boss**:
   - Check if boss-client container is running: `docker-compose ps`
   - Verify network: `docker network inspect mapreduce_mapreduce-net`

2. **Frontend shows blank page**:
   - Check nginx is running: `docker exec mapreduce-controller ps aux | grep nginx`
   - View nginx logs: `docker exec mapreduce-controller cat /var/log/nginx.err.log`
   - Verify build succeeded: `docker logs mapreduce-controller`

3. **WebSocket connection fails**:
   - Check nginx proxy config: `docker exec mapreduce-controller cat /etc/nginx/nginx.conf`
   - Ensure client is running: `docker exec mapreduce-controller ps aux | grep client`

4. **Volume permissions**:
   - Ensure volume directory is writable: `chmod -R 777 ./volume`

4. **Container resource limits**:
   - Adjust CPU/memory in docker-compose.yml based on your system

### Debug Commands:

```bash
# Enter container shell
docker exec -it mapreduce-controller /bin/sh

# Check network from within container
docker exec mapreduce-worker-1 nc -zv boss-client 8080

# View real-time logs
docker-compose logs -f --tail=100

# Monitor resource usage
docker stats
```

## Production Considerations

1. **Security**:
   - Use proper authentication for gRPC connections
   - Implement TLS for all communications
   - Restrict CORS origins in production

2. **Persistence**:
   - Use named volumes for data persistence
   - Consider external storage solutions (NFS, S3)

3. **Monitoring**:
   - Add Prometheus metrics endpoints
   - Use Grafana for visualization
   - Implement distributed tracing

4. **Scaling**:
   - Use Kubernetes for production deployments
   - Implement auto-scaling based on job queue
   - Consider using a service mesh (Istio)

## Next Steps

1. Update source code with environment variable support
2. Create the Dockerfiles as specified
3. Test with sample MapReduce jobs
4. Add health check endpoints to all services
5. Implement graceful shutdown handling