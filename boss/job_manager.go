package main

import (
	"fmt"
	"log"
	"time"
)

/*
processes a completed task and manages job state transitions.
*/
func (boss *BossState) handleTaskCompletion(task *TaskState, success bool, outputPaths []string) {
	if !success {
		// Task failed, requeue it
		boss.requeueFailedTask(task)
		return
	}

	// Get the parent job
	boss.jobsMutex.RLock()
	job, exists := boss.Jobs[task.JobID]
	boss.jobsMutex.RUnlock()

	// crazy edge case? idk
	if !exists {
		log.Printf("Warning: Task %s completed for unknown job %s", task.ID, task.JobID)
		return
	}

	// Task succeeded, lock job state so we can update it.
	job.mutex.Lock()
	defer job.mutex.Unlock()

	// A task may be done multiple times (false retry, etc.)
	// So we maintain a completed set to make sure we aren't processing duplicates
	if !job.CompletedTasks[task.ID] {

		// Mark task as completed
		job.CompletedTasks[task.ID] = true

		// Update task state
		task.OutputPaths = outputPaths
		task.Status = Completed
		task.Progress = 100.0

		switch task.Type {
		case MapTask:
			job.MapTasksDone++
			log.Printf("Job %s: Map tasks completed: %d/%d", job.ID, job.MapTasksDone, job.MapTasksTotal)

			// Store map outputs for reduce phase (TODO: organize by partition)
			// For now, just store the output paths

			// Check if all map tasks are done, and we haven't already transitioned
			// transition into reducing phase
			if job.MapTasksDone == job.MapTasksTotal && job.Phase == Mapping {
				log.Printf("Job %s: All map tasks completed, starting reduce phase", job.ID)
				job.Phase = Reducing

				// Queue reduce tasks (outside of lock)
				go boss.queueReduceTasks(job)
			}
		case ReduceTask:
			job.ReduceTasksDone++
			log.Printf("Job %s: Reduce tasks completed: %d/%d", job.ID, job.ReduceTasksDone, job.ReduceTasksTotal)

			// Check if all reduce tasks are done
			if job.ReduceTasksDone == job.ReduceTasksTotal && job.Phase == Reducing {
				log.Printf("Job %s: All reduce tasks completed, job finished!", job.ID)
				job.Phase = Done
				now := time.Now()
				job.DoneAt = &now
			}
		}
	} else {
		// Task already in completed set - ignore duplicate
		log.Printf("Ignoring duplicate completion for task %s (already in completed set)", task.ID)
	}
}

// requeueFailedTask handles task failures by requeueing with incremented attempt counter
func (boss *BossState) requeueFailedTask(task *TaskState) {
	task.Attempt++
	task.WorkerID = ""
	task.LeaseID = ""
	task.Progress = 0.0
	task.Status = Failed

	log.Printf("Task %s failed (attempt %d), requeuing", task.ID, task.Attempt)

	// Check if we should give up on this task after too many attempts
	if task.Attempt > 3 {
		log.Printf("Task %s failed %d times, marking job as failed", task.ID, task.Attempt)
		boss.markJobAsFailed(task.JobID, fmt.Sprintf("Task %s failed after %d attempts", task.ID, task.Attempt))
		return
	}

	// Requeue the task
	select {
	case boss.PendingTasks <- task:
		log.Printf("Requeued failed task %s", task.ID)
	default:
		log.Printf("Failed to requeue task %s - queue full", task.ID)
	}
}

// queueReduceTasks creates and queues reduce tasks for a job
func (boss *BossState) queueReduceTasks(job *JobState) {
	// Read job info safely
	job.mutex.RLock()
	jobID := job.ID
	reducerCount := job.ReducerCount
	codeURI := job.CodeURI
	inputType := job.InputType
	outputType := job.OutputType
	job.mutex.RUnlock()

	for i := 0; i < reducerCount; i++ {
		reduceTask := &TaskState{
			ID:         fmt.Sprintf("reduce-%s-%d", jobID, i),
			JobID:      jobID,
			Type:       ReduceTask,
			Status:     Queued,
			Progress:   0.0,
			CodeURI:    codeURI,
			OutputDir:  fmt.Sprintf("/jobs/%s", jobID),
			InputType:  inputType,
			OutputType: outputType,
			InputPaths: []string{}, // TODO: Collect map outputs for this partition
			Attempt:    0,
		}

		select {
		case boss.PendingTasks <- reduceTask:
			log.Printf("Queued reduce task %s", reduceTask.ID)
		default:
			log.Printf("Failed to queue reduce task %s - queue full", reduceTask.ID)
		}
	}
}

// markJobAsFailed marks a job as failed with an error message
func (boss *BossState) markJobAsFailed(jobID string, errorMsg string) {
	boss.jobsMutex.RLock()
	job, exists := boss.Jobs[jobID]
	boss.jobsMutex.RUnlock()

	if !exists {
		return
	}

	job.mutex.Lock()
	defer job.mutex.Unlock()

	if job.Phase != Done && job.Phase != JobFailed {
		log.Printf("Marking job %s as failed: %s", jobID, errorMsg)
		job.Phase = JobFailed
		now := time.Now()
		job.DoneAt = &now
	}
}

// createMapTasksForJob creates and queues map tasks for a job
func (boss *BossState) createMapTasksForJob(job *JobState, inputFiles []string) {
	// This would be called when a client submits a job
	// For now, it's a placeholder for the actual implementation

	job.mutex.Lock()
	defer job.mutex.Unlock()

	// Initialize completed tasks set
	job.CompletedTasks = make(map[string]bool)

	// Calculate map tasks based on input files
	// For simplicity, assume one map task per input file
	mapTaskCount := len(inputFiles)
	job.MapTasksTotal = mapTaskCount
	job.TasksTotal = mapTaskCount + job.ReducerCount

	// Queue map tasks
	for i, inputFile := range inputFiles {
		mapTask := &TaskState{
			ID:             fmt.Sprintf("map-%s-%d", job.ID, i),
			JobID:          job.ID,
			Type:           MapTask,
			Status:         Queued,
			Progress:       0.0,
			CodeURI:        job.CodeURI,
			OutputDir:      fmt.Sprintf("/jobs/%s", job.ID),
			InputType:      job.InputType,
			OutputType:     job.OutputType,
			InputURI:       inputFile,
			ReducerCount:   int32(job.ReducerCount),
			ByteStart:      0,  // TODO: Calculate actual byte ranges
			ByteEnd:        -1, // TODO: Calculate actual byte ranges
			EnableCombiner: job.EnableCombiner,
			Attempt:        0,
		}

		select {
		case boss.PendingTasks <- mapTask:
			log.Printf("Queued map task %s", mapTask.ID)
		default:
			log.Printf("Failed to queue map task %s - queue full", mapTask.ID)
		}
	}

	log.Printf("Created job %s with %d map tasks and %d reduce tasks",
		job.ID, mapTaskCount, job.ReducerCount)
}
