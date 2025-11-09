FROM python:3.11-slim

# Set environment variables for deterministic behavior
ENV PYTHONHASHSEED=0
ENV VOLUME_PATH=/volume

# Install dependencies
RUN pip install grpcio psutil aiofiles

# Set working directory
WORKDIR /app

# Copy worker code and generated protobuf files
COPY worker.py .
COPY worker_pb2.py .
COPY worker_pb2_grpc.py .

# Default boss address (can be overridden)
ENV BOSS_ADDR=localhost:8080

# Run the worker
CMD ["python", "worker.py"]