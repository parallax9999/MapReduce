package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
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
	pb.RegisterClientServiceServer(grpcServer, boss)

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
				// Store CPU and memory metrics from pong
				if m.Pong != nil {
					worker.CpuUsage = float64(m.Pong.CpuUsage)
					worker.MemoryUsage = m.Pong.MemoryUsage
				}
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

/*
ClientControl implements the bidirectional streaming RPC for client communication
*/
func (boss *BossState) ClientControl(stream pb.ClientService_ClientControlServer) error {
	var clientID string

	// Channel to send messages to client (small buffer for real-time dashboard streaming)
	outgoingChan := make(chan *pb.BossToClient, 5)

	// Handle outgoing messages
	boss.wg.Add(1)
	go func() {
		defer boss.wg.Done()
		for {
			select {
			case msg := <-outgoingChan:
				if err := stream.Send(msg); err != nil {
					log.Printf("Error sending to client %s: %v", clientID, err)
					return
				}
			case <-boss.ctx.Done():
				return
			}
		}
	}()

	// Start periodic dashboard state streaming
	boss.wg.Add(1)
	go func() {
		defer boss.wg.Done()
		ticker := time.NewTicker(1 * time.Second)  // Stream every second
		defer ticker.Stop()

		for {
			select {
			case <-boss.ctx.Done():
				return
			case <-ticker.C:
				dashboardState := boss.buildDashboardState()
				dashboardMsg := &pb.BossToClient{
					Msg: &pb.BossToClient_DashboardState{
						DashboardState: dashboardState,
					},
				}
				select {
				case outgoingChan <- dashboardMsg:
					// Dashboard state sent
				default:
					log.Printf("Failed to send dashboard state to client %s - channel full", clientID)
				}
			}
		}
	}()

	// Handle incoming messages
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			log.Printf("Client %s disconnected (EOF)", clientID)
			break
		}
		if err != nil {
			log.Printf("Client %s gRPC error: %v", clientID, err)
			break
		}

		switch m := msg.Msg.(type) {
		case *pb.ClientToBoss_Hello:
			clientID = m.Hello.ClientId
			log.Printf("Client %s connected (user: %s)", clientID, m.Hello.UserName)

		case *pb.ClientToBoss_SubmitJob:
			log.Printf("Client %s submitted job request", clientID)
			boss.handleSubmitJob(m.SubmitJob, clientID)
		}
	}

	log.Printf("gRPC stream ended for client %s", clientID)
	return nil
}

// handleTaskResult processes task completion messages from workers
func (boss *BossState) handleTaskResult(result *pb.TaskResult) {
	taskID := result.TaskId.Id
	success := result.Status == pb.TaskStatus_COMPLETED
	outputPaths := result.OutputPaths
	workerReportedType := result.Type

	log.Printf("Received task result for %s: %v (type: %v)", taskID, result.Status, workerReportedType)

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
		// Validate that worker reported the correct task type
		expectedType := task.Type
		var expectedPbType pb.TaskType
		if expectedType == MapTask {
			expectedPbType = pb.TaskType_MAP
		} else {
			expectedPbType = pb.TaskType_REDUCE
		}

		if workerReportedType != expectedPbType {
			log.Printf("Warning: Task %s type mismatch - expected %v, worker reported %v",
				taskID, expectedPbType, workerReportedType)
			// crazy edge case idk
		}

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


// handleSubmitJob processes job submission requests and creates real jobs
func (boss *BossState) handleSubmitJob(req *pb.SubmitJobRequest, clientID string) {
	log.Printf("Job submission from client %s: %d input files, %d reducers",
		clientID, len(req.InputFiles), req.ReducerCount)

	// For now, handle single file only
	if len(req.InputFiles) != 1 {
		log.Printf("Error: Expected 1 input file, got %d", len(req.InputFiles))
		return
	}

	inputFile := req.InputFiles[0]

	// Get file size for byte range calculation
	fileSize, err := boss.getFileSize(inputFile)
	if err != nil {
		log.Printf("Error getting file size for %s: %v", inputFile, err)
		return
	}

	// Create job
	jobID := fmt.Sprintf("job-%s-%d", clientID, time.Now().UnixNano())

	job := &JobState{
		ID:               jobID,
		Phase:            Mapping,
		CodeURI:          req.CodeUri,
		MapperCount:      int(req.MapperCount),
		ReducerCount:     int(req.ReducerCount),
		EnableCombiner:   req.EnableCombiner,
		OriginalFiles:    req.InputFiles,
		InputType:        boss.convertDataFormat(req.InputType),
		OutputType:       boss.convertDataFormat(req.OutputType),
		OutputDir:        req.OutputDir,
		MapOutputs:       make([][]string, int(req.MapperCount)),
		CompletedTasks:   make(map[string]bool),
		MapTasksTotal:    int(req.MapperCount),
		MapTasksDone:     0,
		ReduceTasksTotal: int(req.ReducerCount),
		ReduceTasksDone:  0,
		TasksDone:        0,
		TasksTotal:       int(req.MapperCount) + int(req.ReducerCount),
		CreatedAt:        time.Now(),
	}

	// Register job
	boss.jobsMutex.Lock()
	boss.Jobs[jobID] = job
	boss.jobsMutex.Unlock()

	log.Printf("Created job %s: %d mappers, %d reducers, file size: %d bytes",
		jobID, req.MapperCount, req.ReducerCount, fileSize)

	// Create map tasks with byte ranges
	boss.createMapTasks(job, inputFile, fileSize)
}

// getFileSize returns the size of a file in bytes
func (boss *BossState) getFileSize(filePath string) (int64, error) {
	// Use VOLUME_PATH environment variable
	volumePath := os.Getenv("VOLUME_PATH")
	if volumePath == "" {
		volumePath = "./volume" // fallback to relative path
	}
	fullPath := volumePath + filePath

	file, err := os.Open(fullPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open file %s: %v", fullPath, err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return 0, fmt.Errorf("failed to stat file %s: %v", fullPath, err)
	}

	return stat.Size(), nil
}

// convertDataFormat converts protobuf DataFormat to internal DataFormat
func (boss *BossState) convertDataFormat(pbFormat pb.DataFormat) DataFormat {
	switch pbFormat {
	case pb.DataFormat_TEXT:
		return TEXT
	case pb.DataFormat_PARQUET:
		return PARQUET
	case pb.DataFormat_JSON:
		return JSON
	default:
		return TEXT
	}
}

// createMapTasks creates map tasks with line-aligned byte ranges for a job
func (boss *BossState) createMapTasks(job *JobState, inputFile string, fileSize int64) {
	mapperCount := job.MapperCount

	// Get line-aligned byte ranges for CSV files
	ranges, err := boss.getLineAlignedByteRanges(inputFile, fileSize, mapperCount)
	if err != nil {
		log.Printf("Error creating line-aligned ranges for %s: %v", inputFile, err)
		return
	}

	for i := 0; i < mapperCount; i++ {
		taskID := fmt.Sprintf("map-%s-%d", job.ID, i)

		byteStart := ranges[i].start
		byteEnd := ranges[i].end - 1 // Convert from half-open to inclusive range

		mapTask := &TaskState{
			ID:             taskID,
			JobID:          job.ID,
			Type:           MapTask,
			Status:         Queued,
			Progress:       0.0,
			CodeURI:        job.CodeURI,
			InputType:      job.InputType,
			OutputType:     job.OutputType,
			InputURI:       inputFile,
			ReducerCount:   int32(job.ReducerCount),
			ByteStart:      byteStart,
			ByteEnd:        byteEnd,
			EnableCombiner: job.EnableCombiner,
			Attempt:        0,
		}

		// Queue the task
		select {
		case boss.PendingTasks <- mapTask:
			log.Printf("Queued map task %s: bytes %d-%d (%d bytes)",
				taskID, byteStart, byteEnd, byteEnd-byteStart+1)
		default:
			log.Printf("Failed to queue map task %s - queue full", taskID)
		}
	}

	log.Printf("Created %d map tasks for job %s with line-aligned ranges", mapperCount, job.ID)
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
