package main

import (
	"log"
	"time"
)

/*
Monitoring worker nodes in a few goroutines.
Pings worker nodes periodically.
Checks for failed workers.
Checks for expired leases.
*/
func (boss *BossState) startHealthMonitor() {
	// Ping workers periodically
	// We don't need to do anything since the pong is handled by handleWorkerConnection()
	boss.wg.Add(1)
	go func() {
		defer boss.wg.Done()
		log.Println("Starting worker ping loop...")

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-boss.ctx.Done():
				return
			case <-ticker.C:
				boss.pingAllWorkers()
			}
		}
	}()

	// Check for failed workers every 5 secs
	// If a worker is dead, requeue their ActiveTasks
	boss.wg.Add(1)
	go func() {
		defer boss.wg.Done()
		log.Println("Starting worker failure detection loop...")

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-boss.ctx.Done():
				return
			case <-ticker.C:
				boss.detectFailedWorkers()
			}
		}
	}()

	// Check for expired leases every 5 secs
	// If a lease is expired, requeue the task
	boss.wg.Add(1)
	go func() {
		defer boss.wg.Done()
		log.Println("Starting lease expiry detection loop...")

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-boss.ctx.Done():
				return
			case <-ticker.C:
				boss.detectExpiredLeases()
			}
		}
	}()
}

func (boss *BossState) pingAllWorkers() {
	boss.workersMutex.RLock()
	defer boss.workersMutex.RUnlock()

	for workerID := range boss.Workers {
		// TODO: Send ping over gRPC stream
		log.Printf("Pinging worker %s", workerID)
	}
}

func (boss *BossState) detectFailedWorkers() {
	threshold := 30 * time.Second
	now := time.Now()

	boss.workersMutex.Lock()
	defer boss.workersMutex.Unlock()

	for workerID, worker := range boss.Workers {
		worker.mutex.Lock()
		if worker.Healthy && now.Sub(worker.LastPing) > threshold {
			log.Printf("Marking worker %s as failed (last ping: %v ago)",
				workerID, now.Sub(worker.LastPing))

			worker.Healthy = false

			// Requeue all active tasks
			for taskID, task := range worker.ActiveTasks {
				task.Attempt++
				task.WorkerID = ""
				task.LeaseID = ""
				task.Status = Queued

				select {
				case boss.PendingTasks <- task:
					log.Printf("Requeued task %s from failed worker", taskID)
				default:
					log.Printf("Failed to requeue task %s - queue full", taskID)
				}
			}

			// Clear active tasks
			worker.ActiveTasks = make(map[string]*TaskState)
			worker.Current = 0
		}
		worker.mutex.Unlock()
	}
}

func (boss *BossState) detectExpiredLeases() {
	now := time.Now()

	boss.workersMutex.RLock()
	defer boss.workersMutex.RUnlock()

	for workerID, worker := range boss.Workers {
		worker.mutex.Lock()
		for taskID, task := range worker.ActiveTasks {
			if now.After(task.LeaseExpiry) {
				log.Printf("Task %s on worker %s has expired lease, requeuing", taskID, workerID)

				// Requeue the task
				task.Attempt++
				task.WorkerID = ""
				task.LeaseID = ""
				task.Status = Queued

				select {
				case boss.PendingTasks <- task:
					// Remove from worker's active tasks
					delete(worker.ActiveTasks, taskID)
					worker.Current--
				default:
					log.Printf("Failed to requeue expired task %s - queue full", taskID)
				}
			}
		}
		worker.mutex.Unlock()
	}
}
