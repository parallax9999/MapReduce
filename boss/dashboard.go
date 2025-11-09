package main

import (
	"time"

	pb "mapreduce/pb"
)

// buildDashboardState creates a complete dashboard state snapshot
func (boss *BossState) buildDashboardState() *pb.DashboardState {
	boss.workersMutex.RLock()
	boss.jobsMutex.RLock()
	defer boss.workersMutex.RUnlock()
	defer boss.jobsMutex.RUnlock()

	now := time.Now()

	// Build system overview
	systemOverview := boss.buildSystemOverview()

	// Build worker info
	workers := boss.buildWorkerInfos(now)

	// Build job info
	jobs := boss.buildJobInfos(now)

	// Build task info
	activeTasks, pendingTasks := boss.buildTaskInfos(now)

	return &pb.DashboardState{
		System:       systemOverview,
		Workers:      workers,
		Jobs:         jobs,
		ActiveTasks:  activeTasks,
		PendingTasks: pendingTasks,
		Timestamp:    now.Unix(),
	}
}

// buildSystemOverview creates system-wide statistics
func (boss *BossState) buildSystemOverview() *pb.SystemOverview {
	totalWorkers := len(boss.Workers)
	healthyWorkers := 0
	totalCapacity := 0
	usedCapacity := 0

	for _, worker := range boss.Workers {
		if worker.Healthy {
			healthyWorkers++
		}
		totalCapacity += worker.Capacity
		usedCapacity += worker.Current
	}

	totalJobs := len(boss.Jobs)
	activeJobs := 0
	for _, job := range boss.Jobs {
		if job.Phase == Mapping || job.Phase == Reducing {
			activeJobs++
		}
	}

	pendingTaskCount := len(boss.PendingTasks)

	return &pb.SystemOverview{
		TotalWorkers:     int32(totalWorkers),
		HealthyWorkers:   int32(healthyWorkers),
		TotalCapacity:    int32(totalCapacity),
		UsedCapacity:     int32(usedCapacity),
		PendingTaskCount: int32(pendingTaskCount),
		TotalJobs:        int32(totalJobs),
		ActiveJobs:       int32(activeJobs),
	}
}

// buildWorkerInfos creates worker status information
func (boss *BossState) buildWorkerInfos(now time.Time) []*pb.WorkerInfo {
	var workers []*pb.WorkerInfo

	for _, worker := range boss.Workers {
		worker.mutex.RLock()

		// Get active task IDs
		var activeTaskIds []string
		for taskID := range worker.ActiveTasks {
			activeTaskIds = append(activeTaskIds, taskID)
		}

		lastPingSecondsAgo := int64(now.Sub(worker.LastPing).Seconds())

		workers = append(workers, &pb.WorkerInfo{
			Id:                 worker.ID,
			Healthy:            worker.Healthy,
			Capacity:           int32(worker.Capacity),
			CurrentTasks:       int32(worker.Current),
			ActiveTaskIds:      activeTaskIds,
			LastPingSecondsAgo: lastPingSecondsAgo,
			CpuUsagePercent:    float32(worker.CpuUsage),
			MemoryUsageBytes:   worker.MemoryUsage,
		})

		worker.mutex.RUnlock()
	}

	return workers
}

// buildJobInfos creates job status information
func (boss *BossState) buildJobInfos(now time.Time) []*pb.JobInfo {
	var jobs []*pb.JobInfo

	for _, job := range boss.Jobs {
		job.mutex.RLock()

		// Convert job phase to string
		var phaseStr string
		switch job.Phase {
		case Pending:
			phaseStr = "Pending"
		case Mapping:
			phaseStr = "Mapping"
		case Reducing:
			phaseStr = "Reducing"
		case Done:
			phaseStr = "Done"
		case JobFailed:
			phaseStr = "Failed"
		case JobCanceled:
			phaseStr = "Canceled"
		default:
			phaseStr = "Unknown"
		}

		// Calculate overall progress
		totalTasks := job.MapTasksTotal + job.ReduceTasksTotal
		doneTasks := job.MapTasksDone + job.ReduceTasksDone
		var overallProgress float32
		if totalTasks > 0 {
			overallProgress = float32(doneTasks) / float32(totalTasks) * 100.0
		}

		createdSecondsAgo := int64(now.Sub(job.CreatedAt).Seconds())

		jobs = append(jobs, &pb.JobInfo{
			Id:                job.ID,
			Phase:             phaseStr,
			MapTasksDone:      int32(job.MapTasksDone),
			MapTasksTotal:     int32(job.MapTasksTotal),
			ReduceTasksDone:   int32(job.ReduceTasksDone),
			ReduceTasksTotal:  int32(job.ReduceTasksTotal),
			OverallProgress:   overallProgress,
			MapperCount:       int32(job.MapperCount),
			ReducerCount:      int32(job.ReducerCount),
			CreatedSecondsAgo: createdSecondsAgo,
			EnableCombiner:    job.EnableCombiner,
			InputFiles:        job.OriginalFiles,
			OutputPath:        job.OutputDir,
			CodeUri:           job.CodeURI,
		})

		job.mutex.RUnlock()
	}

	return jobs
}

// buildTaskInfos creates task status information
func (boss *BossState) buildTaskInfos(now time.Time) ([]*pb.TaskInfo, []*pb.TaskInfo) {
	var activeTasks []*pb.TaskInfo
	var pendingTasks []*pb.TaskInfo

	// Collect active tasks from workers
	for _, worker := range boss.Workers {
		worker.mutex.RLock()
		for _, task := range worker.ActiveTasks {
			activeTasks = append(activeTasks, boss.buildTaskInfo(task, now))
		}
		worker.mutex.RUnlock()
	}

	// Collect pending tasks from queue (non-blocking peek)
	queueLength := len(boss.PendingTasks)
	for i := 0; i < queueLength; i++ {
		select {
		case task := <-boss.PendingTasks:
			pendingTasks = append(pendingTasks, boss.buildTaskInfo(task, now))
			// Put the task back in the queue
			boss.PendingTasks <- task
		default:
			break
		}
	}

	return activeTasks, pendingTasks
}

// buildTaskInfo creates a single task's information
func (boss *BossState) buildTaskInfo(task *TaskState, now time.Time) *pb.TaskInfo {
	// Convert task type to string
	var typeStr string
	if task.Type == MapTask {
		typeStr = "MAP"
	} else {
		typeStr = "REDUCE"
	}

	// Convert task status to string
	var statusStr string
	switch task.Status {
	case Queued:
		statusStr = "Queued"
	case Assigned:
		statusStr = "Assigned"
	case InProgress:
		statusStr = "InProgress"
	case Completed:
		statusStr = "Completed"
	case Failed:
		statusStr = "Failed"
	case Canceled:
		statusStr = "Canceled"
	default:
		statusStr = "Unknown"
	}

	// Calculate lease expiry
	var leaseExpiresInSeconds int64
	if !task.LeaseExpiry.IsZero() {
		remaining := task.LeaseExpiry.Sub(now)
		if remaining > 0 {
			leaseExpiresInSeconds = int64(remaining.Seconds())
		} else {
			leaseExpiresInSeconds = 0 // Expired
		}
	}

	// Count input files for reduce tasks
	var inputFileCount int32
	if task.Type == ReduceTask {
		inputFileCount = int32(len(task.InputPaths))
	}

	return &pb.TaskInfo{
		Id:                     task.ID,
		Type:                   typeStr,
		Status:                 statusStr,
		JobId:                  task.JobID,
		WorkerId:               task.WorkerID,
		ProgressPercent:        float32(task.Progress),
		RecordsIn:              task.RecordsIn,
		RecordsOut:             task.RecordsOut,
		Attempt:                int32(task.Attempt),
		LeaseExpiresInSeconds:  leaseExpiresInSeconds,
		ByteStart:              task.ByteStart,
		ByteEnd:                task.ByteEnd,
		InputFileCount:         inputFileCount,
	}
}