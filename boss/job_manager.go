package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

			// Store map outputs organized by partition
			mapperIndex, err := boss.extractMapperIndex(task.ID)
			if err != nil {
				log.Printf("Error extracting mapper index from task %s: %v", task.ID, err)
			} else {
				// Store the output paths for this mapper
				// outputPaths should contain r files, one per partition
				if len(outputPaths) == job.ReducerCount {
					job.MapOutputs[mapperIndex] = outputPaths
					log.Printf("Stored %d partition files for mapper %d", len(outputPaths), mapperIndex)
				} else {
					log.Printf("Warning: Expected %d partitions from mapper %d, got %d",
						job.ReducerCount, mapperIndex, len(outputPaths))
					// Store what we got anyway
					if mapperIndex < len(job.MapOutputs) {
						job.MapOutputs[mapperIndex] = outputPaths
					}
				}
			}

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

			// Move reduce output files to client's specified output directory (async)
			go boss.moveReduceOutputs(job, outputPaths)

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
	mapOutputs := job.MapOutputs
	job.mutex.RUnlock()

	// For each reducer, collect inputs from all mappers for that partition
	for partitionIndex := 0; partitionIndex < reducerCount; partitionIndex++ {
		var inputPaths []string

		// Collect files for this partition from all mappers
		for mapperIndex := 0; mapperIndex < len(mapOutputs); mapperIndex++ {
			mapperOutputs := mapOutputs[mapperIndex]
			if partitionIndex < len(mapperOutputs) {
				partitionFile := mapperOutputs[partitionIndex]
				if partitionFile != "" {
					inputPaths = append(inputPaths, partitionFile)
				}
			}
		}

		log.Printf("Reduce task %d: collected %d input files from mappers", partitionIndex, len(inputPaths))

		reduceTask := &TaskState{
			ID:         fmt.Sprintf("reduce-%s-%d", jobID, partitionIndex),
			JobID:      jobID,
			Type:       ReduceTask,
			Status:     Queued,
			Progress:   0.0,
			CodeURI:    codeURI,
			InputType:  inputType,
			OutputType: outputType,
			InputPaths: inputPaths, // All map outputs for this partition
			Attempt:    0,
		}

		select {
		case boss.PendingTasks <- reduceTask:
			log.Printf("Queued reduce task %s with %d input files", reduceTask.ID, len(inputPaths))
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

// moveReduceOutputs moves completed reduce task outputs to the client's specified output directory
func (boss *BossState) moveReduceOutputs(job *JobState, outputPaths []string) {
	if len(outputPaths) == 0 {
		return
	}

	// Get volume path
	volumePath := os.Getenv("VOLUME_PATH")
	if volumePath == "" {
		volumePath = "./volume"
	}

	// Create output directory if it doesn't exist
	outputDir := filepath.Join(volumePath, job.OutputDir)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Printf("Failed to create output directory %s: %v", outputDir, err)
		return
	}

	// Move each output file to the final output directory
	for _, outputPath := range outputPaths {
		// outputPath is what the worker reported (e.g., "/tmp/reduce_output_0.txt")
		srcPath := filepath.Join(volumePath, outputPath)

		// Extract just the filename for the destination
		filename := filepath.Base(outputPath)
		destPath := filepath.Join(outputDir, filename)

		// Move/copy the file
		if err := boss.moveFile(srcPath, destPath); err != nil {
			log.Printf("Failed to move output file %s to %s: %v", srcPath, destPath, err)
		} else {
			log.Printf("Moved reduce output: %s -> %s", srcPath, destPath)
		}
	}
}

// moveFile moves a file from src to dest (copy + delete since we might be across filesystems)
func (boss *BossState) moveFile(src, dest string) error {
	// Read source file
	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read source file: %v", err)
	}

	// Write to destination
	if err := os.WriteFile(dest, data, 0644); err != nil {
		return fmt.Errorf("failed to write destination file: %v", err)
	}

	// Remove source file
	if err := os.Remove(src); err != nil {
		log.Printf("Warning: failed to remove source file %s: %v", src, err)
		// Don't return error since the important part (copying) succeeded
	}

	return nil
}

// extractMapperIndex extracts the mapper index from a task ID like "map-job-123-2" -> 2
func (boss *BossState) extractMapperIndex(taskID string) (int, error) {
	// Task IDs have format: "map-{jobID}-{mapperIndex}"
	parts := strings.Split(taskID, "-")
	if len(parts) < 2 {
		return 0, fmt.Errorf("invalid task ID format: %s", taskID)
	}

	// The last part should be the mapper index
	indexStr := parts[len(parts)-1]
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse mapper index from %s: %v", indexStr, err)
	}

	return index, nil
}
