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

# Import generated protobuf files
# Note: You'll need to generate these from worker.proto using:
# python -m grpc_tools.protoc -I../boss/proto --python_out=. --grpc_python_out=. ../boss/proto/worker.proto
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

        For now, this simulates task execution. In a real implementation,
        this would:
        - Load user code from code_uri
        - Execute map or reduce function
        - Write output files
        - Send progress updates

        Args:
            task: Task assignment from boss
        """
        task_id = task.task_id.id
        self.current_tasks[task_id] = task

        logger.info(f"Starting task {task_id}")

        # Simulate task execution with progress updates
        task_duration = 10.0  # seconds
        progress_interval = 2.0  # seconds
        start_time = time.time()

        try:
            while True:
                elapsed = time.time() - start_time
                progress = min((elapsed / task_duration) * 100, 100.0)

                # Send progress update
                await self.send_progress(task, progress)

                # Check if task is complete
                if elapsed >= task_duration:
                    await self.complete_task(task)
                    break

                # Wait before next progress update
                await asyncio.sleep(progress_interval)

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

    async def complete_task(self, task: worker_pb2.AssignTask):
        """
        Send task completion result to boss.

        In a real implementation, this would include the actual output paths
        created by the map or reduce function.

        Args:
            task: Completed task
        """
        try:
            result = worker_pb2.WorkerToBoss(
                task_result=worker_pb2.TaskResult(
                    task_id=task.task_id,
                    type=task.type,
                    status=worker_pb2.TaskStatus.COMPLETED,
                    output_paths=[]  # Empty for dummy worker
                )
            )

            await self.stream.write(result)
            logger.info(f"Completed task {task.task_id.id}")

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