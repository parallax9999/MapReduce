package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "mapreduce/pb"
)

type Worker struct {
	id          string
	bossAddr    string
	client      pb.WorkerServiceClient
	stream      pb.WorkerService_ControlClient
	currentTask *pb.AssignTask
	ctx         context.Context
	cancel      context.CancelFunc
}

func main() {
	workerID := fmt.Sprintf("worker-%d-%d", rand.Intn(10000), time.Now().UnixNano())
	bossAddr := "localhost:8080"

	log.Printf("Starting dummy worker %s", workerID)

	worker := &Worker{
		id:       workerID,
		bossAddr: bossAddr,
	}

	if err := worker.connect(); err != nil {
		log.Fatalf("Failed to connect to boss: %v", err)
	}

	worker.run()
}

func (w *Worker) connect() error {
	conn, err := grpc.Dial(w.bossAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to dial boss: %v", err)
	}

	w.client = pb.NewWorkerServiceClient(conn)
	w.ctx, w.cancel = context.WithCancel(context.Background())

	stream, err := w.client.Control(w.ctx)
	if err != nil {
		return fmt.Errorf("failed to create control stream: %v", err)
	}
	w.stream = stream

	// Send hello message
	hello := &pb.WorkerToBoss{
		Msg: &pb.WorkerToBoss_Hello{
			Hello: &pb.WorkerHello{
				WorkerId: &pb.WorkerId{Id: w.id},
				Slots:    1, // Single task capacity
			},
		},
	}

	if err := w.stream.Send(hello); err != nil {
		return fmt.Errorf("failed to send hello: %v", err)
	}

	log.Printf("Connected to boss and sent hello")
	return nil
}

func (w *Worker) run() {

	// Start message handling
	go w.handleMessages()

	// Keep running
	select {
	case <-w.ctx.Done():
		log.Println("Worker shutting down")
	}
}

func (w *Worker) handleMessages() {
	for {
		msg, err := w.stream.Recv()
		if err != nil {
			log.Printf("Error receiving message: %v", err)
			w.cancel()
			return
		}

		switch m := msg.Msg.(type) {
		case *pb.BossToWorker_AssignTask:
			log.Printf("Received task assignment: %s", m.AssignTask.TaskId.Id)
			go w.handleTask(m.AssignTask)

		case *pb.BossToWorker_Ping:
			log.Printf("Received ping: %d", m.Ping)
			// Send pong response
			pong := &pb.WorkerToBoss{
				Msg: &pb.WorkerToBoss_Pong{
					Pong: &pb.Heartbeat{
						CpuUsage:    rand.Float64() * 100,
						MemoryUsage: rand.Int63n(1000000),
					},
				},
			}

			if err := w.stream.Send(pong); err != nil {
				log.Printf("Failed to send pong: %v", err)
			} else {
				log.Println("Sent pong response")
			}

		case *pb.BossToWorker_Kill:
			log.Println("Received kill signal")
			w.cancel()
			return
		}
	}
}

func (w *Worker) handleTask(task *pb.AssignTask) {
	w.currentTask = task
	taskID := task.TaskId.Id

	log.Printf("Starting task %s", taskID)

	// Send progress updates every 2 seconds for 10 seconds
	progressTicker := time.NewTicker(2 * time.Second)
	defer progressTicker.Stop()

	taskStart := time.Now()
	taskDuration := 10 * time.Second

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-progressTicker.C:
			elapsed := time.Since(taskStart)
			progress := (elapsed.Seconds() / taskDuration.Seconds()) * 100

			if progress >= 100 {
				progress = 100
			}

			progressMsg := &pb.WorkerToBoss{
				Msg: &pb.WorkerToBoss_TaskProgress{
					TaskProgress: &pb.TaskProgress{
						TaskId:     task.TaskId,
						Percent:    progress,
						ReadBytes:  rand.Int63n(10000),
						WriteBytes: rand.Int63n(5000),
						RecordsIn:  rand.Int63n(1000),
						RecordsOut: rand.Int63n(800),
					},
				},
			}

			if err := w.stream.Send(progressMsg); err != nil {
				log.Printf("Failed to send progress: %v", err)
				return
			}
			log.Printf("Sent progress for task %s: %.1f%%", taskID, progress)

			// If we're done, send completion
			if elapsed >= taskDuration {
				w.completeTask(task)
				return
			}
		}
	}
}

func (w *Worker) completeTask(task *pb.AssignTask) {
	result := &pb.WorkerToBoss{
		Msg: &pb.WorkerToBoss_TaskResult{
			TaskResult: &pb.TaskResult{
				TaskId:      task.TaskId,
				Type:        task.Type,
				Status:      pb.TaskStatus_COMPLETED,
				OutputPaths: []string{}, // Empty for dummy worker
			},
		},
	}

	if err := w.stream.Send(result); err != nil {
		log.Printf("Failed to send task result: %v", err)
		return
	}

	log.Printf("Completed task %s", task.TaskId.Id)
	w.currentTask = nil
}
