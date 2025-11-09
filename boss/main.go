package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.Println("Starting MapReduce Boss...")

	// Initialize boss state
	boss := InitializeBossState()

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Scheduler loop pops from PendingTasks channel and distributes them to workers
	boss.startSchedulerLoop()

	boss.startHealthMonitor()

	// Start gRPC server
	if err := boss.startGRPCServer(8080); err != nil {
		log.Fatalf("Failed to start gRPC server: %v", err)
	}

	go boss.test()

	log.Println("MapReduce Boss started successfully")

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutdown signal received...")

	// Cancel all goroutines
	boss.cancel()

	// Wait for all goroutines to finish
	boss.wg.Wait()

	log.Println("MapReduce Boss shutdown complete")
}

func (boss *BossState) test() {
	// time.Sleep(2 * time.Second)

	// // Create test job first
	// testJob := &JobState{
	// 	ID:               "test-job-1",
	// 	Phase:            Mapping,
	// 	CodeURI:          "/test/wordcount.py",
	// 	MapperCount:      2,
	// 	ReducerCount:     2,
	// 	EnableCombiner:   false,
	// 	OriginalFiles:    []string{"/test/input.txt", "/test/input2.txt"},
	// 	InputType:        TEXT,
	// 	OutputType:       TEXT,
	// 	MapOutputs:       make([][]string, 2),
	// 	CompletedTasks:   make(map[string]bool),
	// 	MapTasksTotal:    2,
	// 	MapTasksDone:     0,
	// 	ReduceTasksTotal: 2,
	// 	ReduceTasksDone:  0,
	// 	TasksDone:        0,
	// 	TasksTotal:       4, // 2 map + 2 reduce
	// 	CreatedAt:        time.Now(),
	// }

	// // Register the job
	// boss.jobsMutex.Lock()
	// boss.Jobs[testJob.ID] = testJob
	// boss.jobsMutex.Unlock()

	// log.Printf("Created test job %s with %d map tasks and %d reduce tasks",
	// 	testJob.ID, testJob.MapTasksTotal, testJob.ReduceTasksTotal)

	// // Create 2 map tasks
	// testTask1 := &TaskState{
	// 	ID:             "test-map-task-1",
	// 	JobID:          "test-job-1",
	// 	Type:           MapTask,
	// 	Status:         Queued,
	// 	Progress:       0.0,
	// 	CodeURI:        "/test/wordcount.py",
	// 	OutputDir:      "/jobs/test-job-1",
	// 	InputType:      TEXT,
	// 	OutputType:     TEXT,
	// 	InputURI:       "/test/input.txt",
	// 	ReducerCount:   2,
	// 	ByteStart:      0,
	// 	ByteEnd:        1000,
	// 	EnableCombiner: false,
	// 	Attempt:        0,
	// }

	// testTask2 := &TaskState{
	// 	ID:             "test-map-task-2",
	// 	JobID:          "test-job-1",
	// 	Type:           MapTask,
	// 	Status:         Queued,
	// 	Progress:       0.0,
	// 	CodeURI:        "/test/wordcount.py",
	// 	OutputDir:      "/jobs/test-job-1",
	// 	InputType:      TEXT,
	// 	OutputType:     TEXT,
	// 	InputURI:       "/test/input2.txt",
	// 	ReducerCount:   2,
	// 	ByteStart:      0,
	// 	ByteEnd:        1000,
	// 	EnableCombiner: false,
	// 	Attempt:        0,
	// }

	// log.Printf("Adding test tasks to pending queue")
	// boss.PendingTasks <- testTask1
	// boss.PendingTasks <- testTask2

	// time.Sleep(2 * time.Second)

	// // Create a second test job
	// testJob2 := &JobState{
	// 	ID:               "test-job-2",
	// 	Phase:            Mapping,
	// 	CodeURI:          "/test/wordcount.py",
	// 	MapperCount:      1,
	// 	ReducerCount:     1,
	// 	EnableCombiner:   false,
	// 	OriginalFiles:    []string{"/test/input.txt"},
	// 	InputType:        TEXT,
	// 	OutputType:       TEXT,
	// 	MapOutputs:       make([][]string, 1),
	// 	CompletedTasks:   make(map[string]bool),
	// 	MapTasksTotal:    1,
	// 	MapTasksDone:     0,
	// 	ReduceTasksTotal: 1,
	// 	ReduceTasksDone:  0,
	// 	TasksDone:        0,
	// 	TasksTotal:       2, // 1 map + 1 reduce
	// 	CreatedAt:        time.Now(),
	// }

	// // Register the job
	// boss.jobsMutex.Lock()
	// boss.Jobs[testJob2.ID] = testJob2
	// boss.jobsMutex.Unlock()

	// log.Printf("Created test job %s with %d map tasks and %d reduce tasks",
	// 	testJob2.ID, testJob2.MapTasksTotal, testJob2.ReduceTasksTotal)

	// // Create a test task
	// testTask2 := &TaskState{
	// 	ID:             "test-map-task-2",
	// 	JobID:          "test-job-2",
	// 	Type:           MapTask,
	// 	Status:         Queued,
	// 	Progress:       0.0,
	// 	CodeURI:        "/test/wordcount.py",
	// 	OutputDir:      "/jobs/test-job-1",
	// 	InputType:      TEXT,
	// 	OutputType:     TEXT,
	// 	InputURI:       "/test/input.txt",
	// 	ReducerCount:   1,
	// 	ByteStart:      0,
	// 	ByteEnd:        1000,
	// 	EnableCombiner: false,
	// 	Attempt:        0,
	// }

	// log.Printf("Adding test task %s to pending queue", testTask.ID)
	// boss.PendingTasks <- testTask2
}
