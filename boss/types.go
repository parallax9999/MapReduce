package main

import (
	"context"
	"sync"
	"time"
	
	pb "mapreduce/pb"
)

// TaskType represents the type of task (Map or Reduce)
type TaskType int

const (
	MapTask TaskType = iota
	ReduceTask
)

// TaskStatus represents the current status of a task throughout its lifecycle
type TaskStatus int

const (
	Queued     TaskStatus = iota // Task is waiting to be assigned
	Assigned                     // Task has been assigned to a worker
	InProgress                   // Worker is actively working on the task
	Completed                    // Task finished successfully
	Failed                       // Task failed and needs to be retried
	Canceled                     // Task was explicitly canceled
)

// DataFormat represents the format of input/output data
type DataFormat int

const (
	TEXT DataFormat = iota
	PARQUET
	JSON
)

// JobPhase represents the current phase of a MapReduce job
type JobPhase int

const (
	Pending JobPhase = iota
	Mapping
	Reducing
	Done
	JobFailed
	JobCanceled
)

// TaskState represents a single map or reduce task
type TaskState struct {
	ID     string
	Type   TaskType
	Status TaskStatus

	// Pointer back to Job
	JobID string

	// Pointer back to Worker
	WorkerID string

	// Lease management
	LeaseID     string
	LeaseExpiry time.Time
	Attempt     int

	// Progress tracking
	Progress   float64
	RecordsIn  int64
	RecordsOut int64

	// Task configuration
	CodeURI     string
	OutputDir   string
	InputType   DataFormat
	OutputType  DataFormat
	OutputPaths []string

	// Map-specific fields
	InputURI       string
	ReducerCount   int32
	ByteStart      int64
	ByteEnd        int64
	EnableCombiner bool

	// Reduce-specific fields
	InputPaths []string
}

// WorkerState tracks the state of a single worker
type WorkerState struct {
	ID          string
	OutChan     chan *pb.BossToWorker // Channel to send messages to worker
	Current     int
	Capacity    int
	Healthy     bool
	LastPing    time.Time
	ActiveTasks map[string]*TaskState
	mutex       sync.RWMutex
}

// JobState represents a complete MapReduce job
type JobState struct {
	ID    string
	Phase JobPhase

	CodeURI        string
	MapperCount    int
	ReducerCount   int
	EnableCombiner bool
	OriginalFiles  []string
	InputType      DataFormat
	OutputType     DataFormat

	// 2D array: [mapper_index][partition_index] = file_path
	MapOutputs [][]string

	// Set-based completion tracking
	CompletedTasks map[string]bool // Set of completed task IDs

	// Counters
	MapTasksTotal    int
	MapTasksDone     int
	ReduceTasksTotal int
	ReduceTasksDone  int
	TasksDone        int
	TasksTotal       int

	CreatedAt time.Time
	DoneAt    *time.Time
	mutex     sync.RWMutex
}

// BossState is the central state of the MapReduce master
type BossState struct {
	pb.UnimplementedWorkerServiceServer
	
	// Cluster state
	Workers      map[string]*WorkerState
	workersMutex sync.RWMutex

	// Job management
	Jobs      map[string]*JobState
	jobsMutex sync.RWMutex

	// Task queue
	PendingTasks chan *TaskState

	// Shutdown coordination
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}
