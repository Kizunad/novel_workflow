package queue

import (
	"context"
	"fmt"
	
	"github.com/Kizunad/modular-workflow-v2/character_update"
	"github.com/Kizunad/modular-workflow-v2/summarizer"
	"github.com/Kizunad/modular-workflow-v2/worldview_summarizer"
)

// SummarizerAdapter 摘要处理器适配器
type SummarizerAdapter struct {
	processor *summarizer.Processor
}

// NewSummarizerAdapter 创建摘要处理器适配器
func NewSummarizerAdapter(processor *summarizer.Processor) *SummarizerAdapter {
	return &SummarizerAdapter{
		processor: processor,
	}
}

// TaskType 实现 TaskProcessor 接口
func (a *SummarizerAdapter) TaskType() string {
	return a.processor.TaskType()
}

// ProcessTask 实现 TaskProcessor 接口 
func (a *SummarizerAdapter) ProcessTask(ctx context.Context, task Task) error {
	// 将 queue.Task 转换为 summarizer.Task
	summarizerTask, ok := task.(*summarizer.SummarizeTask)
	if !ok {
		return fmt.Errorf("任务类型不匹配，期望 SummarizeTask，得到 %T", task)
	}
	
	return a.processor.ProcessTask(ctx, summarizerTask)
}

// CharacterUpdateAdapter 角色更新处理器适配器
type CharacterUpdateAdapter struct {
	processor *character_update.Processor
}

// NewCharacterUpdateAdapter 创建角色更新处理器适配器
func NewCharacterUpdateAdapter(processor *character_update.Processor) *CharacterUpdateAdapter {
	return &CharacterUpdateAdapter{
		processor: processor,
	}
}

// TaskType 实现 TaskProcessor 接口
func (a *CharacterUpdateAdapter) TaskType() string {
	return a.processor.TaskType()
}

// ProcessTask 实现 TaskProcessor 接口
func (a *CharacterUpdateAdapter) ProcessTask(ctx context.Context, task Task) error {
	// 将 queue.Task 转换为 character_update.Task
	updateTask, ok := task.(*character_update.CharacterUpdateTask)
	if !ok {
		return fmt.Errorf("任务类型不匹配，期望 CharacterUpdateTask，得到 %T", task)
	}
	
	return a.processor.ProcessTask(ctx, updateTask)
}

// WorldviewSummarizerAdapter 世界观总结处理器适配器
type WorldviewSummarizerAdapter struct {
	processor *worldview_summarizer.Processor
}

// NewWorldviewSummarizerAdapter 创建世界观总结处理器适配器
func NewWorldviewSummarizerAdapter(processor *worldview_summarizer.Processor) *WorldviewSummarizerAdapter {
	return &WorldviewSummarizerAdapter{
		processor: processor,
	}
}

// TaskType 实现 TaskProcessor 接口
func (a *WorldviewSummarizerAdapter) TaskType() string {
	return a.processor.TaskType()
}

// ProcessTask 实现 TaskProcessor 接口
func (a *WorldviewSummarizerAdapter) ProcessTask(ctx context.Context, task Task) error {
	// 将 queue.Task 转换为 worldview_summarizer.Task
	worldviewTask, ok := task.(*worldview_summarizer.WorldviewSummarizerTask)
	if !ok {
		return fmt.Errorf("任务类型不匹配，期望 WorldviewSummarizerTask，得到 %T", task)
	}
	
	return a.processor.ProcessTask(ctx, worldviewTask)
}