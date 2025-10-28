# design.md

## Client interface
- Client implementations of Map and Reduce will be done in python.
- User uploads files to the frontend (React) which will talk to the client, the client will send to the boss, who uploads directly to the shared volume.
## Language choices
- **Go** for the boss
- **Python** for the worker
- **Python** for the client
- **React** for the frontend
## System Communication
- boss <-> client and boss <-> workers, we use gRPC / protobufs
- for frontend-client communication, we use websockets/HTTP
## Shared Storage
- We use a shared volume that the docker containers (and boss) mount to
## Input/Output Data
- Client's choice of the output format
- The protobuffs for each task tells the workers what format file format to output
## Testing 
- Spawning workers dynamically using a set variable, in this case, 4, since there are 4 CPUs
- Worker recovery for 1, 2, 3, and 4 failures.
- Task assignment to open worker
- Long task reassignment/completion
- Individual tests for the client, boss, and workers
## Performance/Special Feature
- As our special feature, we will implement a task queue that allows for requeuing when for straggling tasks and failed workers.
- special feature performance will be measured by determining key timings (time to detect failiures, time to recover, CPU time)


