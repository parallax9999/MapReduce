# Dockerfile for MapReduce Worker (Python)
FROM python:3.11-slim

# Install system dependencies
RUN apt-get update && apt-get install -y \
    gcc \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy requirements first for better caching
COPY requirements.txt .

# Install Python dependencies
RUN pip install --no-cache-dir -r requirements.txt

# Copy ALL Python files (including worker.py and generated protobuf files)
COPY *.py ./

# Set environment variables
ENV BOSS_ADDR=boss:8080
ENV WORKER_SLOTS=1
ENV VOLUME_PATH=/volume

# Run the worker
CMD ["python", "worker.py"]
