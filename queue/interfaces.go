package queue

import "context"

// TaskProcessor 任务处理器接口
type TaskProcessor interface {
	ProcessTask(ctx context.Context, task Task) error
	TaskType() string
}

// Task 任务接口
type Task interface {
	GetID() string
	GetType() string
	GetPriority() int
	GetPayload() interface{}
}

// TaskStatus 任务状态
type TaskStatus int

const (
	TaskStatusPending TaskStatus = iota
	TaskStatusProcessing
	TaskStatusCompleted
	TaskStatusFailed
)

// WorkerStatus Worker状态
type WorkerStatus struct {
	ID          int    `json:"id"`
	Status      string `json:"status"`
	CurrentTask string `json:"current_task,omitempty"`
}

// QueueStatus 队列状态
type QueueStatus struct {
	PendingTasks    int            `json:"pending_tasks"`
	ProcessingTasks int            `json:"processing_tasks"`
	CompletedTasks  int64          `json:"completed_tasks"`
	FailedTasks     int64          `json:"failed_tasks"`
	Workers         []WorkerStatus `json:"workers"`
	ProcessorsCount int            `json:"processors_count"`
}