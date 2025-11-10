# MapReduce Worker (Python)

Python implementation of a MapReduce worker that connects to the boss server and executes map/reduce tasks.

## Setup

1. **Install dependencies:**
   ```bash
   pip install -r requirements.txt
   ```

2. **Generate protobuf files:**
   ```bash
   python3 -m grpc_tools.protoc \
     -I../boss/proto \
     --python_out=. \
     --grpc_python_out=. \
     ../boss/proto/worker.proto
   ```

   This will generate:
   - `worker_pb2.py` - Protobuf message definitions
   - `worker_pb2_grpc.py` - gRPC service stubs

## Running the Worker

```bash
# Run with default settings (localhost:8080, 1 slot)
python worker.py

# Run with custom boss address
BOSS_ADDR="192.168.1.100:8080" python worker.py

# Run with multiple task slots
WORKER_SLOTS=4 python worker.py

# Run with both custom settings
BOSS_ADDR="boss.example.com:8080" WORKER_SLOTS=2 python worker.py
```

## Environment Variables

- `BOSS_ADDR` - Address of the boss server (default: `localhost:8080`)
- `WORKER_SLOTS` - Number of concurrent tasks this worker can handle (default: `1`)

## Architecture

The worker follows the design laid out in `map_reduce_context.claude`:

1. **Connection Phase:**
   - Connects to boss via gRPC bidirectional stream
   - Sends `WorkerHello` message with worker ID and slot count

2. **Operation Phase:**
   - Receives messages from boss:
     - `AssignTask` - Execute a map or reduce task
     - `Ping` - Respond with pong/heartbeat
     - `Kill` - Shutdown gracefully
   - Sends messages to boss:
     - `TaskProgress` - Update task progress to extend lease
     - `TaskResult` - Report task completion
     - `Heartbeat` - Pong response with system stats

3. **Task Execution:**
   - Currently simulates task execution (10 seconds)
   - Sends progress updates every 2 seconds
   - In a real implementation, would:
     - Load user code from `code_uri`
     - Execute map/reduce functions
     - Read input files and write output files
     - Handle partitioning (for map tasks)

## Implementation Notes

This implementation mirrors the structure of the Go `dummy-worker`:
- Uses async/await for concurrent task handling
- Supports multiple concurrent tasks based on `slots`
- Implements proper lease extension via progress updates
- Handles graceful shutdown on kill signal

## Next Steps

To make this a fully functional worker:
1. Implement actual map function execution
2. Implement actual reduce function execution
3. Add file I/O for reading inputs and writing outputs
4. Add partitioning logic for map tasks
5. Add combiner function support
6. Support different data formats (TEXT, PARQUET, JSON)
7. Load and execute user-provided code from `code_uri`
