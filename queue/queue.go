package queue

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Kizunad/modular-workflow-v2/logger"
)

// MessageQueue 消息队列
type MessageQueue struct {
	config     *Config
	logger     *logger.ZapLogger
	processors map[string]TaskProcessor
	tasks      chan Task
	workers    []*Worker
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup

	// 统计信息
	completedTasks int64
	failedTasks    int64
	mu             sync.RWMutex
}

// Worker 工作器
type Worker struct {
	id      int
	status  string
	current string
	queue   *MessageQueue
	mu      sync.RWMutex
}

// New 创建新的消息队列
func New(config *Config, logger *logger.ZapLogger) *MessageQueue {
	ctx, cancel := context.WithCancel(context.Background())

	return &MessageQueue{
		config:     config,
		logger:     logger,
		processors: make(map[string]TaskProcessor),
		tasks:      make(chan Task, config.BufferSize),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Register 注册任务处理器
func (mq *MessageQueue) Register(processor TaskProcessor) {
	mq.mu.Lock()
	defer mq.mu.Unlock()

	taskType := processor.TaskType()
	mq.processors[taskType] = processor
	mq.logger.Info(fmt.Sprintf("注册任务处理器: %s", taskType))
}

// Start 启动消息队列
func (mq *MessageQueue) Start(ctx context.Context) error {
	if !mq.config.Enabled {
		mq.logger.Info("消息队列未启用")
		return nil
	}

	/*
		这里有两个问题：

		你是按值传递 Worker 给 workerLoop，里面的 setStatus 修改的是 拷贝对象，而不是 mq.workers[i]。

		即使 Worker 里有 mu 锁，它锁住的是拷贝，和队列保存的 worker 不是同一个实例。

		所以 GetStatus 时看到的 worker 状态，根本不会变化。
	*/
	// 创建并启动 workers
	mq.workers = make([]*Worker, mq.config.Workers)
	for i := 0; i < mq.config.Workers; i++ {
		worker := Worker{
			id:     i,
			status: "idle",
			queue:  mq,
		}
		mq.workers[i] = &worker

		mq.wg.Add(1)
		go mq.workerLoop(&worker)
	}

	mq.logger.Info(fmt.Sprintf("消息队列启动成功，%d个Worker运行中", mq.config.Workers))
	return nil
}

// Enqueue 入队任务
func (mq *MessageQueue) Enqueue(task Task) error {
	select {
	case <-mq.ctx.Done():
		return fmt.Errorf("队列已关闭")
	case mq.tasks <- task:
		mq.logger.Debug(fmt.Sprintf("任务入队: %s [%s]", task.GetID(), task.GetType()))
		return nil
	default:
		return fmt.Errorf("队列已满")
	}
}

// workerLoop Worker 主循环
func (mq *MessageQueue) workerLoop(worker *Worker) {
	defer mq.wg.Done()

	mq.logger.Debug(fmt.Sprintf("Worker %d 启动", worker.id))

	for {
		select {
		case <-mq.ctx.Done():
			mq.logger.Debug(fmt.Sprintf("Worker %d 停止", worker.id))
			return
		case task, ok := <-mq.tasks:
			if !ok {
				mq.logger.Debug(fmt.Sprintf("Worker %d 队列关闭", worker.id))
				return
			}
			worker.setStatus("processing", task.GetID())
			mq.processTaskWithRetry(task)
			worker.setStatus("idle", "")
		}
	}
}

// processTaskWithRetry 带重试的任务处理
func (mq *MessageQueue) processTaskWithRetry(task Task) {
	var err error

	for attempt := 0; attempt <= mq.config.MaxRetries; attempt++ {
		err = mq.ProcessTask(mq.ctx, task)
		if err == nil {
			atomic.AddInt64(&mq.completedTasks, 1)
			mq.logger.Debug(fmt.Sprintf("任务完成: %s", task.GetID()))
			return
		}

		mq.logger.Warn(fmt.Sprintf(
			"任务 %s 执行失败 (第%d次): %v",
			task.GetID(), attempt+1, err,
		))

		if attempt < mq.config.MaxRetries {
			// 指数退避
			backoff := time.Duration(1<<attempt) * mq.config.RetryInterval
			time.Sleep(backoff)
		}
	}

	atomic.AddInt64(&mq.failedTasks, 1)
	mq.logger.Error(fmt.Sprintf("任务 %s 重试次数耗尽，最终失败: %v", task.GetID(), err))
}

// ProcessTask 处理单个任务
func (mq *MessageQueue) ProcessTask(ctx context.Context, task Task) error {
	mq.mu.RLock()
	processor, exists := mq.processors[task.GetType()]
	mq.mu.RUnlock()

	if !exists {
		return fmt.Errorf("未找到任务类型 %s 的处理器", task.GetType())
	}

	return processor.ProcessTask(ctx, task)
}

// GetStatus 获取队列状态
func (mq *MessageQueue) GetStatus() QueueStatus {
	mq.mu.RLock()
	processorsCount := len(mq.processors)
	mq.mu.RUnlock()

	workerStatuses := make([]WorkerStatus, len(mq.workers))
	for i, worker := range mq.workers {
		workerStatuses[i] = WorkerStatus{
			ID:          worker.id,
			Status:      worker.getStatus(),
			CurrentTask: worker.getCurrent(),
		}
	}

	return QueueStatus{
		PendingTasks:    len(mq.tasks),
		ProcessingTasks: mq.getProcessingCount(),
		CompletedTasks:  atomic.LoadInt64(&mq.completedTasks),
		FailedTasks:     atomic.LoadInt64(&mq.failedTasks),
		Workers:         workerStatuses,
		ProcessorsCount: processorsCount,
	}
}

// getProcessingCount 获取正在处理的任务数
func (mq *MessageQueue) getProcessingCount() int {
	count := 0
	for _, worker := range mq.workers {
		if worker.getStatus() == "processing" {
			count++
		}
	}
	return count
}

// Shutdown 优雅关闭
func (mq *MessageQueue) Shutdown(timeout time.Duration) error {
	mq.logger.Info("开始关闭消息队列...")

	// 停止接收新任务
	mq.cancel()
	close(mq.tasks)

	// 等待现有任务完成
	done := make(chan struct{})
	go func() {
		mq.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		mq.logger.Info("消息队列优雅关闭完成")
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("关闭超时")
	}
}

// Worker 方法
func (w *Worker) setStatus(status, current string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.status = status
	w.current = current
}

func (w *Worker) getStatus() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.status
}

func (w *Worker) getCurrent() string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.current
}

// WaitUntilComplete 等待所有任务处理完成
func (mq *MessageQueue) WaitUntilComplete() {
	if !mq.config.Enabled {
		return
	}

	mq.logger.Info("等待所有任务处理完成...")
	
	for {
		status := mq.GetStatus()
		if status.PendingTasks == 0 && status.ProcessingTasks == 0 {
			mq.logger.Info(fmt.Sprintf("所有任务处理完成！完成: %d, 失败: %d", 
				status.CompletedTasks, status.FailedTasks))
			break
		}
		
		mq.logger.Debug(fmt.Sprintf("队列状态 - 待处理: %d, 处理中: %d, 已完成: %d, 失败: %d",
			status.PendingTasks, status.ProcessingTasks, 
			status.CompletedTasks, status.FailedTasks))
		
		time.Sleep(1 * time.Second)
	}
}
