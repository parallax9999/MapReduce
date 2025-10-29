# design.md

## Client interface
- Client implementations of Map and Reduce will be done in python.
- User uploads files to the frontend (React) which will talk to the client, the client will send to the boss, who uploads directly to the shared volume.
- Client.py will have a submitJob() function that takes these arguments:
    - Path to Python file containing user’s map() and reduce() functions
    - Path to input data files
    - Output format

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
- Boss writes to shared volume at /uploads/&lt;job-id&gt; and /code/&lt;job-id&gt;
## Input/Output Data
- Client's choice of the output format
- The protobuffs for each task tells the workers what format file format to output
## Testing 
- Spawning workers dynamically using a set variable, in this case, 4, since there are 4 CPUs
- Worker recovery for 1, 2, 3, and 4 failures.
- Task assignment to open worker
- Long task reassignment/completion
- Individual tests for the client, boss, and workers
- Classic word count on sample text files, sum, average, max/min
- Verify output correctness matches expected results
- Multiple input files handling
- Test parsing and output of different file formats
- Run same job with and without combiner enabled
- Compare intermediate data size and execution time
- Submit multiple jobs simultaneously
- Fair scheduling across jobs
- Empty input files
- Very large files (multi-GB)
- Files with very long lines/records
- Malformed data handling
## Performance/Special Feature
- As our special feature, we will implement a task queue that allows for requeuing when for straggling tasks and failed workers.
- special feature performance will be measured by determining key timings (time to detect failiures, time to recover, CPU time)
    - Plot: Time to detect worker failure vs number of workers
    - Plot: Job completion time with 0, 1, 2, 3, 4 worker failures
    - Plot: Task requeue latency when lease expires
- For throughput:
    - Records processed per second
    - Bytes processed per second
    - Speedup with varying numbers of workers (1, 2, 3, 4)


---

### Additional details:
[Google doc for protobuffs and structs](https://docs.google.com/document/d/1YcSvGkeu3RkAzqwNxshYmHVy1qNCkKK0jGcbzysP-RA/edit?usp=sharing)

