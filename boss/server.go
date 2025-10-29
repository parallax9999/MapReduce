package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"

	pb "mapreduce/pb"
)

// startGRPCServer starts the gRPC server for worker and client connections
func (boss *BossState) startGRPCServer(port int) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %v", port, err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterWorkerServiceServer(grpcServer, boss)

	log.Printf("gRPC server listening on port %d", port)

	boss.wg.Add(1)
	go func() {
		defer boss.wg.Done()
		defer lis.Close()
		defer grpcServer.Stop()

		go func() {
			if err := grpcServer.Serve(lis); err != nil {
				log.Printf("gRPC server error: %v", err)
			}
		}()

		<-boss.ctx.Done()
		log.Println("gRPC server shutting down...")
		grpcServer.GracefulStop()
	}()

	return nil
}

/*
bidirectional streaming RPC for worker communication
*/
func (boss *BossState) Control(stream pb.WorkerService_ControlServer) error {
	var workerID string
	var worker *WorkerState

	// Channel to send messages to worker
	outgoingChan := make(chan *pb.BossToWorker, 100)

	// Handle outgoing messages
	boss.wg.Add(1)
	go func() {
		defer boss.wg.Done()
		for {
			select {
			case msg := <-outgoingChan:
				if err := stream.Send(msg); err != nil {
					log.Printf("Error sending to worker %s: %v", workerID, err)
					return
				}
			case <-boss.ctx.Done():
				return
			}
		}
	}()

	// Handle incoming messages
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			log.Printf("Worker %s disconnected (EOF)", workerID)
			break
		}
		if err != nil {
			log.Printf("Worker %s gRPC error: %v", workerID, err)
			break
		}

		switch m := msg.Msg.(type) {
		case *pb.WorkerToBoss_Hello:
			workerID = m.Hello.WorkerId.Id
			log.Printf("Worker %s connected with capacity %d", workerID, m.Hello.Slots)

			// Register worker
			boss.workersMutex.Lock()
			worker = &WorkerState{
				ID:          workerID,
				OutChan:     outgoingChan,
				Current:     0,
				Capacity:    int(m.Hello.Slots),
				Healthy:     true,
				LastPing:    time.Now(),
				ActiveTasks: make(map[string]*TaskState),
			}
			boss.Workers[workerID] = worker
			boss.workersMutex.Unlock()

		case *pb.WorkerToBoss_Pong:
			if worker != nil {
				worker.mutex.Lock()
				worker.LastPing = time.Now()
				worker.mutex.Unlock()
			}

		case *pb.WorkerToBoss_TaskResult:
			boss.handleTaskResult(m.TaskResult)

		case *pb.WorkerToBoss_TaskProgress:
			boss.handleTaskProgress(m.TaskProgress)

		case *pb.WorkerToBoss_Log:
			log.Printf("Worker %s: %s", workerID, m.Log)
		}
	}

	// gRPC stream ended - rely on ping-pong health check for cleanup
	log.Printf("gRPC stream ended for worker %s", workerID)
	return nil
}

// handleTaskResult processes task completion messages from workers
func (boss *BossState) handleTaskResult(result *pb.TaskResult) {
	taskID := result.TaskId.Id
	success := result.Status == pb.TaskStatus_COMPLETED
	outputPaths := result.OutputPaths

	log.Printf("Received task result for %s: %v", taskID, result.Status)

	// Find the task
	boss.workersMutex.RLock()
	var task *TaskState
	for _, worker := range boss.Workers {
		worker.mutex.RLock()
		if t, exists := worker.ActiveTasks[taskID]; exists {
			task = t
			// Remove from worker's active tasks
			worker.mutex.RUnlock()
			worker.mutex.Lock()
			delete(worker.ActiveTasks, taskID)
			worker.Current--
			worker.mutex.Unlock()
			break
		}
		worker.mutex.RUnlock()
	}
	boss.workersMutex.RUnlock()

	if task != nil {
		boss.handleTaskCompletion(task, success, outputPaths)
	} else {
		log.Printf("Warning: Received result for unknown task %s", taskID)
	}
}

// handleTaskProgress processes progress updates from workers
func (boss *BossState) handleTaskProgress(progress *pb.TaskProgress) {
	taskID := progress.TaskId.Id

	// Find and update the task
	boss.workersMutex.RLock()
	for _, worker := range boss.Workers {
		worker.mutex.RLock()
		if task, exists := worker.ActiveTasks[taskID]; exists {
			task.Progress = progress.Percent
			task.RecordsIn = progress.RecordsIn
			task.RecordsOut = progress.RecordsOut
			// Extend lease
			task.LeaseExpiry = time.Now().Add(30 * time.Second)
			worker.mutex.RUnlock()
			break
		}
		worker.mutex.RUnlock()
	}
	boss.workersMutex.RUnlock()
}

/*
Creates and initializes a new BossState.
Map of wortkers, Map of Jobs, and channel for PendingTasks
*/
func InitializeBossState() *BossState {
	ctx, cancel := context.WithCancel(context.Background())

	return &BossState{
		Workers:      make(map[string]*WorkerState),
		Jobs:         make(map[string]*JobState),
		PendingTasks: make(chan *TaskState, 1000), // Buffered channel
		ctx:          ctx,
		cancel:       cancel,
	}
}
