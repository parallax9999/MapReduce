package main

import (
	"fmt"
	"log"
	pb "mapreduce/pb"
	"time"
)

/*
Task scheduling logic.
If there is a task, initialize the lease and try to assign it to an available worker.
If all workers are unavailable, put the task back into the queue.
*/
func (boss *BossState) startSchedulerLoop() {
	boss.wg.Add(1)

	go func() {
		defer boss.wg.Done()

		log.Println("Starting scheduler loop...")

		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-boss.ctx.Done():
				log.Println("Scheduler loop shutting down...")
				return

			case task := <-boss.PendingTasks:
				// Initialize task lease
				task.LeaseID = fmt.Sprintf("lease-%d", time.Now().UnixNano())
				task.LeaseExpiry = time.Now().Add(30 * time.Second)

				// Try to assign to available worker
				assigned := boss.assignTaskToWorker(task)

				// No available workers, requeue the task
				if !assigned {
					select {
					// sneaky atomic check + send in the pocket
					case boss.PendingTasks <- task:
						// Successfully requeued
					default:
						// Queue is full, drop the task for now
						log.Printf("Warning: Dropping task %s due to full queue", task.ID)
					}
				}

			case <-ticker.C:
				// Periodic status logging?
				boss.logSchedulerStatus()
				continue
			}
		}
	}()
}

/*
Finds an available worker and assigns the task via gRPC stream.
Greedy solution. Will always try to queue the first worker
*/
func (boss *BossState) assignTaskToWorker(task *TaskState) bool {
	boss.workersMutex.RLock()
	defer boss.workersMutex.RUnlock()

	for workerID, worker := range boss.Workers {
		worker.mutex.Lock()
		if worker.Healthy && worker.Current < worker.Capacity {
			// Assign task to this worker
			worker.Current++
			worker.ActiveTasks[task.ID] = task
			task.WorkerID = workerID
			task.Status = Assigned

			// Send AssignTask over gRPC stream
			assignMsg := boss.createAssignTaskMessage(task)

			select {
			case worker.OutChan <- assignMsg:
				log.Printf("Assigned task %s to worker %s", task.ID, workerID)
			default:
				log.Printf("Failed to send task %s to worker %s - channel full", task.ID, workerID)
				// Revert assignment
				worker.Current--
				delete(worker.ActiveTasks, task.ID)
				task.WorkerID = ""
				task.Status = Queued
				worker.mutex.Unlock()
				return false
			}

			worker.mutex.Unlock()
			return true
		}
		worker.mutex.Unlock()
	}

	return false
}

// temp for logging stuff
// logSchedulerStatus logs current system status every tick
func (boss *BossState) logSchedulerStatus() {
	boss.workersMutex.RLock()
	boss.jobsMutex.RLock()

	totalWorkers := len(boss.Workers)
	healthyWorkers := 0
	activeTasks := 0

	for _, worker := range boss.Workers {
		if worker.Healthy {
			healthyWorkers++
			activeTasks += worker.Current
		}
	}

	totalJobs := len(boss.Jobs)
	pendingTasks := len(boss.PendingTasks)

	boss.workersMutex.RUnlock()
	boss.jobsMutex.RUnlock()

	log.Printf("Status: Workers=%d/%d healthy, Jobs=%d, Tasks=%d active, %d pending",
		healthyWorkers, totalWorkers, totalJobs, activeTasks, pendingTasks)
}

/*
converts a TaskState to protobuf AssignTask message
*/
func (boss *BossState) createAssignTaskMessage(task *TaskState) *pb.BossToWorker {
	var taskType pb.TaskType
	if task.Type == MapTask {
		taskType = pb.TaskType_MAP
	} else {
		taskType = pb.TaskType_REDUCE
	}

	var inputType, outputType pb.DataFormat
	switch task.InputType {
	case TEXT:
		inputType = pb.DataFormat_TEXT
	case PARQUET:
		inputType = pb.DataFormat_PARQUET
	case JSON:
		inputType = pb.DataFormat_JSON
	}

	switch task.OutputType {
	case TEXT:
		outputType = pb.DataFormat_TEXT
	case PARQUET:
		outputType = pb.DataFormat_PARQUET
	case JSON:
		outputType = pb.DataFormat_JSON
	}

	assignTask := &pb.AssignTask{
		TaskId:     &pb.TaskId{Id: task.ID},
		Type:       taskType,
		JobId:      task.JobID,
		CodeUri:    task.CodeURI,
		LeaseId:    task.LeaseID,
		LeaseMs:    int64(task.LeaseExpiry.Sub(time.Now()).Milliseconds()),
		Attempt:    int32(task.Attempt),
		OutputDir:  task.OutputDir,
		InputType:  inputType,
		OutputType: outputType,
	}

	// Add task-specific fields
	if task.Type == MapTask {
		assignTask.InputUri = task.InputURI
		assignTask.ByteStart = task.ByteStart
		assignTask.ByteEnd = task.ByteEnd
		assignTask.ReducerCount = task.ReducerCount
		assignTask.EnableCombiner = task.EnableCombiner
	} else {
		assignTask.InputPaths = task.InputPaths
	}

	return &pb.BossToWorker{
		Msg: &pb.BossToWorker_AssignTask{
			AssignTask: assignTask,
		},
	}
}
