package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
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
	time.Sleep(2 * time.Second)

	// Create a test task
	testTask := &TaskState{
		ID:             "test-map-task-1",
		JobID:          "test-job-1",
		Type:           MapTask,
		Status:         Queued,
		Progress:       0.0,
		CodeURI:        "/test/wordcount.py",
		OutputDir:      "/jobs/test-job-1",
		InputType:      TEXT,
		OutputType:     TEXT,
		InputURI:       "/test/input.txt",
		ReducerCount:   2,
		ByteStart:      0,
		ByteEnd:        1000,
		EnableCombiner: false,
		Attempt:        0,
	}

	log.Printf("Adding test task %s to pending queue", testTask.ID)
	boss.PendingTasks <- testTask
}
