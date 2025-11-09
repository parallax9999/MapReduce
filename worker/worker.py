#!/usr/bin/env python3
"""
MapReduce Worker Implementation

This worker connects to the boss via gRPC and processes map/reduce tasks.
It follows the design laid out in map_reduce_context.claude and mirrors
the structure of the Go dummy-worker implementation.
"""

import asyncio
import aiofiles
import logging
import psutil
import random
import threading
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
        self.send_lock = asyncio.Lock()  # Prevent concurrent stream writes

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

            async with self.send_lock:
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

                msg_type = msg.WhichOneof('msg')
                if msg_type == 'ping':
                    # Handle pings immediately - they should be fast
                    logger.info(f"Received ping {msg.ping}")
                    await self.send_pong()
                else:
                    # Handle other messages (tasks run as background tasks)
                    await self.handle_message(msg)

        except grpc.aio.AioRpcError as e:
            logger.error(f"Stream error: {e}")
        except Exception as e:
            logger.error(f"Unexpected error in run loop: {e}")
        finally:
            logger.info("Worker shutting down")
            self.shutdown = True

    async def handle_message(self, msg: worker_pb2.BossToWorker):
        """
        Handle a message from the boss (excluding pings which are handled separately).

        Args:
            msg: Message from boss
        """
        msg_type = msg.WhichOneof('msg')

        if msg_type == 'assign_task':
            task = msg.assign_task
            logger.info(f"Received task assignment: {task.task_id.id} (type: {worker_pb2.TaskType.Name(task.type)})")

            # Start task in background
            asyncio.create_task(self.handle_task(task))

        elif msg_type == 'kill':
            logger.info("Received kill signal")
            self.shutdown = True

        else:
            logger.warning(f"Unknown message type: {msg_type}")

    async def send_pong(self):
        """
        Send pong response to boss with real system metrics.
        """
        try:
            # Get real system metrics (non-blocking)
            cpu_percent = psutil.cpu_percent(interval=None)  # Non-blocking, returns last reading
            memory = psutil.virtual_memory()
            memory_bytes = memory.used  # Memory usage in bytes

            pong = worker_pb2.WorkerToBoss(
                pong=worker_pb2.Heartbeat(
                    cpu_usage=cpu_percent,
                    memory_usage=memory_bytes
                )
            )

            async with self.send_lock:
                await self.stream.write(pong)
            logger.info(f"SENT PONG RESPONSE (CPU: {cpu_percent:.1f}%, Memory: {memory_bytes/1024/1024:.1f}MB)")

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

    async def send_progress(self, task: worker_pb2.AssignTask, percent: float, 
                           records_in: int = 0, records_out: int = 0,
                           read_bytes: int = 0, write_bytes: int = 0):
        """
        Send progress update to boss to extend lease.

        Args:
            task: Task being worked on
            percent: Progress percentage (0-100)
            records_in: Number of records read
            records_out: Number of records written
            read_bytes: Bytes read from input
            write_bytes: Bytes written to output
        """
        try:
            progress_msg = worker_pb2.WorkerToBoss(
                task_progress=worker_pb2.TaskProgress(
                    task_id=task.task_id,
                    percent=percent,
                    read_bytes=read_bytes,
                    write_bytes=write_bytes,
                    records_in=records_in,
                    records_out=records_out
                )
            )

            async with self.send_lock:
                await self.stream.write(progress_msg)
            logger.info(f"Sent progress for task {task.task_id.id}: {percent:.1f}% "
                       f"(in: {records_in}, out: {records_out})")

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

            async with self.send_lock:
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

            async with self.send_lock:
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

        # Calculate total bytes to process for progress tracking
        total_bytes = task.byte_end - task.byte_start + 1
        bytes_processed = 0
        
        # Read input data within byte range with progress updates
        input_path = os.path.join(volume_path, task.input_uri.lstrip('/'))
        input_data = await self.read_input_range_with_progress(task, input_path, task.byte_start, task.byte_end)

        # Create output files for each partition
        output_paths = []
        partition_files = {}
        
        # If combiner is enabled, collect intermediate results first
        if task.enable_combiner:
            partition_data = {i: {} for i in range(task.reducer_count)}  # partition -> {key: [values]}
        
        try:
            # Process input data line by line
            lines = input_data.strip().split('\n')
            records_processed = 0
            records_written = 0
            bytes_written = 0
            last_progress_time = time.time()
            
            for line in lines:
                if not line.strip():
                    bytes_processed += len(line) + 1  # Include newline
                    continue

                # Track bytes as we process
                bytes_processed += len(line) + 1  # Include newline

                # Parse CSV line (key,value)
                parts = line.strip().split(',', 1)
                if len(parts) != 2:
                    continue

                key, value = parts[0].strip(), parts[1].strip()
                
                # Try to convert value to int if it looks numeric (for wordcount-style tasks)
                try:
                    value = int(value)
                except ValueError:
                    # Keep as string if not numeric
                    pass

                # Apply map function
                for output_key, output_value in map_function(key, value):
                    # Determine partition using hash
                    partition = hash(str(output_key)) % task.reducer_count

                    if task.enable_combiner:
                        # Collect values for combiner
                        if str(output_key) not in partition_data[partition]:
                            partition_data[partition][str(output_key)] = []
                        partition_data[partition][str(output_key)].append(output_value)
                    else:
                        # Write directly to partition files (if they exist)
                        if partition not in partition_files:
                            output_file = os.path.join(volume_path, f"map_{task.task_id.id}_{partition}.txt")
                            partition_files[partition] = open(output_file, 'w')
                        
                        output_line = f"{output_key},{output_value}\n"
                        partition_files[partition].write(output_line)
                        bytes_written += len(output_line.encode('utf-8'))
                        records_written += 1

                records_processed += 1

                # Send progress updates based on time (every 5 seconds) to extend lease
                current_time = time.time()
                if current_time - last_progress_time >= 5.0:
                    # Calculate progress based on bytes processed
                    progress = (bytes_processed / total_bytes * 100) if total_bytes > 0 else 0
                    await self.send_progress(task, progress, 
                                           records_in=records_processed,
                                           records_out=records_written,
                                           read_bytes=bytes_processed,
                                           write_bytes=bytes_written)
                    last_progress_time = current_time

            # If combiner is enabled, apply combiner and write results
            if task.enable_combiner:
                logger.info(f"Applying combiner function to {len([k for p in partition_data.values() for k in p.keys()])} intermediate keys")
                
                # Load combiner function (same as reduce function typically)
                code_path = os.path.join(volume_path, task.code_uri.lstrip('/'))
                combiner_function = await self.load_user_code(code_path, 'reduce')
                
                # Create partition files and apply combiner
                for partition in range(task.reducer_count):
                    if partition_data[partition]:  # Only create files for partitions with data
                        output_file = os.path.join(volume_path, f"map_{task.task_id.id}_{partition}.txt")
                        partition_files[partition] = open(output_file, 'w')
                        
                        # Apply combiner to each key's values
                        for key, values in partition_data[partition].items():
                            for combined_key, combined_value in combiner_function(key, values):
                                output_line = f"{combined_key},{combined_value}\n"
                                partition_files[partition].write(output_line)
                                bytes_written += len(output_line.encode('utf-8'))
                                records_written += 1
                
                # Create output paths list for all partitions
                for partition in range(task.reducer_count):
                    output_file = os.path.join(volume_path, f"map_{task.task_id.id}_{partition}.txt")
                    relative_path = output_file.replace(volume_path, '').lstrip('/')
                    output_paths.append(relative_path)
                    
            else:
                # Create output paths list for non-combiner case
                for partition in range(task.reducer_count):
                    output_file = os.path.join(volume_path, f"map_{task.task_id.id}_{partition}.txt")
                    relative_path = output_file.replace(volume_path, '').lstrip('/')
                    output_paths.append(relative_path)

            logger.info(f"Map task processed {records_processed} records, wrote {records_written} outputs")

        finally:
            # Close all partition files
            for f in partition_files.values():
                f.close()

        # Send final progress with complete metrics
        await self.send_progress(task, 100.0, 
                                records_in=records_processed,
                                records_out=records_written,
                                read_bytes=bytes_processed,
                                write_bytes=bytes_written)
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
        bytes_read = 0
        bytes_written = 0

        # Read all input files
        for file_index, input_path in enumerate(task.input_paths):
            full_input_path = os.path.join(volume_path, input_path.lstrip('/'))
            logger.info(f"Reading reduce input: {full_input_path}")

            try:
                async with aiofiles.open(full_input_path, 'r') as f:
                    async for line in f:
                        bytes_read += len(line.encode('utf-8'))
                        line = line.strip()
                        if not line:
                            continue

                        # Send progress update every 50KB during file reading (stay at 0%)
                        if bytes_read % 50000 == 0:
                            await self.send_progress(task, 0.0, records_read, 0, bytes_read, 0)
                            await asyncio.sleep(0)  # Yield control

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
        async with aiofiles.open(output_file, 'w') as out_f:
            records_written = 0
            total_keys = len(key_groups)
            keys_processed = 0
            
            for key, values in key_groups.items():
                for output_key, output_value in reduce_function(key, values):
                    output_line = f"{output_key},{output_value}\n"
                    await out_f.write(output_line)
                    bytes_written += len(output_line.encode('utf-8'))
                    records_written += 1
                    
                    # Send progress update during processing (0-100% based on keys processed)
                    if records_written % 100 == 0:
                        progress_percent = (keys_processed / total_keys) * 100.0 if total_keys > 0 else 0.0
                        await self.send_progress(task, progress_percent, records_read, records_written, bytes_read, bytes_written)
                        await asyncio.sleep(0)
                
                keys_processed += 1

        logger.info(f"Reduce task read {records_read} records, wrote {records_written} results")

        # Return relative path
        relative_path = output_file.replace(volume_path, '').lstrip('/')
        
        # Send final progress with complete metrics
        await self.send_progress(task, 100.0,
                                records_in=records_read,
                                records_out=records_written,
                                read_bytes=bytes_read,
                                write_bytes=bytes_written)
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
        async with aiofiles.open(code_path, 'r') as f:
            code = await f.read()

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

        async with aiofiles.open(input_path, 'r') as f:
            await f.seek(start)
            data = await f.read(end - start + 1)

        return data

    async def read_input_range_with_progress(self, task, input_path: str, start: int, end: int) -> str:
        """
        Read a specific byte range from an input file with progress updates.
        
        Args:
            task: Task object for progress reporting
            input_path: Path to input file
            start: Start byte offset
            end: End byte offset (inclusive)
            
        Returns:
            The data within the specified range
        """
        total_bytes = end - start + 1
        bytes_read = 0
        data_chunks = []
        
        logger.info(f"Reading {input_path} bytes {start}-{end} ({total_bytes:,} bytes)")
        
        async with aiofiles.open(input_path, 'r') as f:
            await f.seek(start)
            
            # Read in 1MB chunks to allow progress updates
            chunk_size = 1024 * 1024  # 1MB chunks
            
            while bytes_read < total_bytes:
                remaining = total_bytes - bytes_read
                read_size = min(chunk_size, remaining)
                
                chunk = await f.read(read_size)
                if not chunk:  # EOF
                    break
                    
                data_chunks.append(chunk)
                bytes_read += len(chunk)
                
                # Send progress every chunk (every 1MB)
                await self.send_progress(task, 0.0, 0, 0, bytes_read, 0)
                await asyncio.sleep(0)  # Yield control
        
        return ''.join(data_chunks)


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