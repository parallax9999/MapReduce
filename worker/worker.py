#!/usr/bin/env python3
"""
MapReduce Worker Implementation

This worker connects to the boss via gRPC and processes map/reduce tasks.
It follows the design laid out in map_reduce_context.claude and mirrors
the structure of the Go dummy-worker implementation.
"""

import asyncio
import logging
import random
import time
import sys
import os
from typing import Optional

import grpc

try:
    import worker_pb2
    import worker_pb2_grpc
except ImportError:
    print("Error: Protobuf files not generated. Run:")
    print("  python -m grpc_tools.protoc -I../boss/proto --python_out=. --grpc_python_out=. ../boss/proto/worker.proto")
    sys.exit(1)


logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class Worker:
    """
    MapReduce Worker that connects to the boss and executes tasks.

    Attributes:
        id: Unique worker identifier
        boss_addr: Address of the boss server (e.g., "localhost:8080")
        slots: Number of concurrent tasks this worker can handle
        current_tasks: Dictionary of currently running tasks
        stream: Bidirectional gRPC stream to the boss
        shutdown: Flag to signal shutdown
    """

    def __init__(self, worker_id: Optional[str] = None, boss_addr: str = "localhost:8080", slots: int = 1):
        """
        Initialize a new worker.

        Args:
            worker_id: Unique worker ID (generated if not provided)
            boss_addr: Address of the boss server
            slots: Number of concurrent tasks this worker can handle
        """
        self.id = worker_id or f"worker-{random.randint(1000, 9999)}-{int(time.time() * 1000)}"
        self.boss_addr = boss_addr
        self.slots = slots
        self.current_tasks = {}
        self.stream = None
        self.shutdown = False

        logger.info(f"Initializing worker {self.id}")

    async def connect(self) -> bool:
        """
        Connect to the boss server and send hello message.

        Returns:
            True if connection successful, False otherwise
        """
        try:
            # Create insecure channel (use secure credentials in production)
            channel = grpc.aio.insecure_channel(self.boss_addr)
            stub = worker_pb2_grpc.WorkerServiceStub(channel)

            # Create bidirectional stream
            self.stream = stub.Control()

            # Send hello message
            hello = worker_pb2.WorkerToBoss(
                hello=worker_pb2.WorkerHello(
                    worker_id=worker_pb2.WorkerId(id=self.id),
                    slots=self.slots
                )
            )

            await self.stream.write(hello)
            logger.info(f"Connected to boss at {self.boss_addr} and sent hello")

            return True

        except Exception as e:
            logger.error(f"Failed to connect to boss: {e}")
            return False

    async def run(self):
        """
        Main worker loop: handle messages from boss.
        """
        if not await self.connect():
            return

        # Start message handling
        try:
            async for msg in self.stream:
                if self.shutdown:
                    break

                await self.handle_message(msg)

        except grpc.aio.AioRpcError as e:
            logger.error(f"Stream error: {e}")
        except Exception as e:
            logger.error(f"Unexpected error in run loop: {e}")
        finally:
            logger.info("Worker shutting down")

    async def handle_message(self, msg: worker_pb2.BossToWorker):
        """
        Handle a message from the boss.

        Args:
            msg: Message from boss
        """
        msg_type = msg.WhichOneof('msg')

        if msg_type == 'assign_task':
            task = msg.assign_task
            logger.info(f"Received task assignment: {task.task_id.id} (type: {worker_pb2.TaskType.Name(task.type)})")

            # Start task in background
            asyncio.create_task(self.handle_task(task))

        elif msg_type == 'ping':
            logger.info(f"Received ping: {msg.ping}")
            await self.send_pong()

        elif msg_type == 'kill':
            logger.info("Received kill signal")
            self.shutdown = True

        else:
            logger.warning(f"Unknown message type: {msg_type}")

    async def send_pong(self):
        """
        Send pong response to boss with heartbeat information.
        """
        try:
            # Send heartbeat with system stats
            pong = worker_pb2.WorkerToBoss(
                pong=worker_pb2.Heartbeat(
                    cpu_usage=random.random() * 100,  # Mock CPU usage
                    memory_usage=random.randint(100000, 1000000)  # Mock memory usage
                )
            )

            await self.stream.write(pong)
            logger.info("Sent pong response")

        except Exception as e:
            logger.error(f"Failed to send pong: {e}")

    async def handle_task(self, task: worker_pb2.AssignTask):
        """
        Handle a task assignment from the boss.

        This executes the actual MapReduce task:
        - Load user code from code_uri
        - Execute map or reduce function on input data
        - Write output files
        - Send progress updates

        Args:
            task: Task assignment from boss
        """
        task_id = task.task_id.id
        self.current_tasks[task_id] = task

        # Print detailed task information
        logger.info(f"=== RECEIVED TASK {task_id} ===")
        logger.info(f"Task Type: {worker_pb2.TaskType.Name(task.type)}")
        logger.info(f"Input URI: {task.input_uri}")
        logger.info(f"Code URI: {task.code_uri}")
        logger.info(f"Byte Range: {task.byte_start} - {task.byte_end}")
        logger.info(f"Input Type: {worker_pb2.DataFormat.Name(task.input_type)}")
        logger.info(f"Output Type: {worker_pb2.DataFormat.Name(task.output_type)}")
        logger.info(f"Reducer Count: {task.reducer_count}")
        logger.info(f"Enable Combiner: {task.enable_combiner}")
        logger.info(f"Attempt: {task.attempt}")
        logger.info("===============================")

        # Get volume path from environment
        volume_path = os.getenv("VOLUME_PATH", "./volume")
        logger.info(f"Using volume path: {volume_path}")

        logger.info(f"Starting task {task_id}")

        try:
            # Execute the actual MapReduce task
            if task.type == worker_pb2.TaskType.MAP:
                output_paths = await self.execute_map_task(task, volume_path)
            elif task.type == worker_pb2.TaskType.REDUCE:
                output_paths = await self.execute_reduce_task(task, volume_path)
            else:
                raise ValueError(f"Unknown task type: {task.type}")

            # Task completed successfully
            await self.complete_task(task, output_paths)

        except Exception as e:
            logger.error(f"Error handling task {task_id}: {e}")
            await self.fail_task(task, str(e))
        finally:
            # Remove from current tasks
            if task_id in self.current_tasks:
                del self.current_tasks[task_id]

    async def send_progress(self, task: worker_pb2.AssignTask, percent: float):
        """
        Send progress update to boss to extend lease.

        Args:
            task: Task being worked on
            percent: Progress percentage (0-100)
        """
        try:
            progress_msg = worker_pb2.WorkerToBoss(
                task_progress=worker_pb2.TaskProgress(
                    task_id=task.task_id,
                    percent=percent,
                    read_bytes=random.randint(0, 10000),
                    write_bytes=random.randint(0, 5000),
                    records_in=random.randint(0, 1000),
                    records_out=random.randint(0, 800)
                )
            )

            await self.stream.write(progress_msg)
            logger.info(f"Sent progress for task {task.task_id.id}: {percent:.1f}%")

        except Exception as e:
            logger.error(f"Failed to send progress: {e}")

    async def complete_task(self, task: worker_pb2.AssignTask, output_paths: list[str]):
        """
        Send task completion result to boss.

        Args:
            task: Completed task
            output_paths: List of output file paths created by the task
        """
        try:
            result = worker_pb2.WorkerToBoss(
                task_result=worker_pb2.TaskResult(
                    task_id=task.task_id,
                    type=task.type,
                    status=worker_pb2.TaskStatus.COMPLETED,
                    output_paths=output_paths
                )
            )

            await self.stream.write(result)
            logger.info(f"Completed task {task.task_id.id} with {len(output_paths)} output files")

        except Exception as e:
            logger.error(f"Failed to send task result: {e}")

    async def fail_task(self, task: worker_pb2.AssignTask, error: str):
        """
        Send task failure result to boss.

        Args:
            task: Failed task
            error: Error message
        """
        try:
            result = worker_pb2.WorkerToBoss(
                task_result=worker_pb2.TaskResult(
                    task_id=task.task_id,
                    type=task.type,
                    status=worker_pb2.TaskStatus.FAILED,
                    output_paths=[]
                )
            )

            await self.stream.write(result)
            logger.error(f"Failed task {task.task_id.id}: {error}")

        except Exception as e:
            logger.error(f"Failed to send task failure: {e}")

    async def execute_map_task(self, task: worker_pb2.AssignTask, volume_path: str) -> list[str]:
        """
        Execute a map task: read input data, apply map function, write partitioned outputs.

        Args:
            task: Map task assignment
            volume_path: Base volume path

        Returns:
            List of output file paths (one per reducer partition)
        """
        logger.info(f"Executing map task {task.task_id.id}")

        # Load user code
        code_path = os.path.join(volume_path, task.code_uri.lstrip('/'))
        map_function = await self.load_user_code(code_path, 'map')

        # Read input data within byte range
        input_path = os.path.join(volume_path, task.input_uri.lstrip('/'))
        input_data = await self.read_input_range(input_path, task.byte_start, task.byte_end)

        # Create output files for each partition
        output_paths = []
        partition_files = {}

        try:
            # Open output files for each partition directly in volume root
            for partition in range(task.reducer_count):
                output_file = os.path.join(volume_path, f"map_{task.task_id.id}_{partition}.txt")
                partition_files[partition] = open(output_file, 'w')
                # Store relative path (remove volume_path prefix for reporting to boss)
                relative_path = output_file.replace(volume_path, '').lstrip('/')
                output_paths.append(relative_path)

            # Process input data line by line
            records_processed = 0
            for line in input_data.strip().split('\n'):
                if not line.strip():
                    continue

                # Parse CSV line (key,value)
                parts = line.strip().split(',', 1)
                if len(parts) != 2:
                    continue

                key, value = parts[0].strip(), parts[1].strip()

                # Apply map function
                for output_key, output_value in map_function(key, value):
                    # Determine partition using hash
                    partition = hash(str(output_key)) % task.reducer_count

                    # Write to appropriate partition file
                    partition_files[partition].write(f"{output_key},{output_value}\n")

                records_processed += 1

                # Send progress updates periodically
                if records_processed % 100 == 0:
                    await self.send_progress(task, 50.0)  # Mid-way progress

            logger.info(f"Map task processed {records_processed} records")

        finally:
            # Close all partition files
            for f in partition_files.values():
                f.close()

        await self.send_progress(task, 100.0)
        return output_paths

    async def execute_reduce_task(self, task: worker_pb2.AssignTask, volume_path: str) -> list[str]:
        """
        Execute a reduce task: read partitioned inputs, apply reduce function, write final output.

        Args:
            task: Reduce task assignment
            volume_path: Base volume path

        Returns:
            List of output file paths
        """
        logger.info(f"Executing reduce task {task.task_id.id}")

        # Load user code
        code_path = os.path.join(volume_path, task.code_uri.lstrip('/'))
        reduce_function = await self.load_user_code(code_path, 'reduce')

        # Create output file directly in volume root
        output_file = os.path.join(volume_path, f"reduce_{task.task_id.id}.txt")

        # Group input data by key
        key_groups = {}
        records_read = 0

        # Read all input files
        for input_path in task.input_paths:
            full_input_path = os.path.join(volume_path, input_path.lstrip('/'))
            logger.info(f"Reading reduce input: {full_input_path}")

            try:
                with open(full_input_path, 'r') as f:
                    for line in f:
                        line = line.strip()
                        if not line:
                            continue

                        # Parse key,value
                        parts = line.split(',', 1)
                        if len(parts) != 2:
                            continue

                        key, value = parts[0].strip(), parts[1].strip()

                        # Try to convert value to int if possible
                        try:
                            value = int(value)
                        except ValueError:
                            pass

                        # Group by key
                        if key not in key_groups:
                            key_groups[key] = []
                        key_groups[key].append(value)

                        records_read += 1

            except FileNotFoundError:
                logger.warning(f"Input file not found: {full_input_path}")

        # Apply reduce function to each key group
        with open(output_file, 'w') as out_f:
            records_written = 0
            for key, values in key_groups.items():
                for output_key, output_value in reduce_function(key, values):
                    out_f.write(f"{output_key},{output_value}\n")
                    records_written += 1

        logger.info(f"Reduce task read {records_read} records, wrote {records_written} results")

        # Return relative path
        relative_path = output_file.replace(volume_path, '').lstrip('/')
        await self.send_progress(task, 100.0)
        return [relative_path]

    async def load_user_code(self, code_path: str, function_name: str):
        """
        Load and return the specified function from user's Python code.

        Args:
            code_path: Path to the Python file
            function_name: Name of function to load ('map' or 'reduce')

        Returns:
            The requested function
        """
        logger.info(f"Loading {function_name} function from {code_path}")

        # Read and execute the user's code
        with open(code_path, 'r') as f:
            code = f.read()

        # Create a namespace to execute the code in
        namespace = {}
        exec(code, namespace)

        # Return the requested function
        if function_name not in namespace:
            raise ValueError(f"Function '{function_name}' not found in {code_path}")

        return namespace[function_name]

    async def read_input_range(self, input_path: str, start: int, end: int) -> str:
        """
        Read a specific byte range from an input file.

        Args:
            input_path: Path to input file
            start: Start byte offset
            end: End byte offset (inclusive)

        Returns:
            The data within the specified range
        """
        logger.info(f"Reading {input_path} bytes {start}-{end}")

        with open(input_path, 'r') as f:
            f.seek(start)
            data = f.read(end - start + 1)

        return data


async def main():
    """
    Main entry point for the worker.
    """
    # Get boss address from environment or use default
    boss_addr = os.getenv("BOSS_ADDR", "localhost:8080")

    # Get worker slots from environment or use default
    slots = int(os.getenv("WORKER_SLOTS", "1"))

    # Create and run worker
    worker = Worker(boss_addr=boss_addr, slots=slots)

    try:
        await worker.run()
    except KeyboardInterrupt:
        logger.info("Worker interrupted by user")
    except Exception as e:
        logger.error(f"Worker error: {e}")


if __name__ == "__main__":
    asyncio.run(main())