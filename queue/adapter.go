package queue

import (
	"context"
	"fmt"
	
	"github.com/Kizunad/modular-workflow-v2/components/workflows"
)

// SummarizerAdapter 摘要处理器适配器
type SummarizerAdapter struct {
	workflow *workflows.SummarizerWorkflow
}

// NewSummarizerAdapter 创建摘要处理器适配器
func NewSummarizerAdapter(workflow *workflows.SummarizerWorkflow) *SummarizerAdapter {
	return &SummarizerAdapter{
		workflow: workflow,
	}
}

// TaskType 实现 TaskProcessor 接口
func (a *SummarizerAdapter) TaskType() string {
	return "summarize"
}

// ProcessTask 实现 TaskProcessor 接口 
func (a *SummarizerAdapter) ProcessTask(ctx context.Context, task Task) error {
	// 从任务载荷获取参数
	payload := task.GetPayload()
	
	// 根据载荷类型处理不同的摘要任务
	switch p := payload.(type) {
	case map[string]interface{}:
		if chapterContent, ok := p["chapter_content"].(string); ok {
			// 处理章节内容摘要
			return a.workflow.ProcessSummarize(ctx, chapterContent)
		} else if chapterID, ok := p["chapter_id"].(string); ok {
			// 通过章节ID处理摘要
			return a.workflow.ProcessSummarizeByID(ctx, chapterID)
		}
		// 如果没有指定参数，处理最新章节摘要
		return a.workflow.ProcessLatestChapterSummary(ctx)
	case string:
		// 如果载荷是字符串，作为章节内容处理
		return a.workflow.ProcessSummarize(ctx, p)
	default:
		return fmt.Errorf("不支持的载荷类型: %T", payload)
	}
}

// CharacterUpdateAdapter 角色更新处理器适配器
type CharacterUpdateAdapter struct {
	workflow *workflows.CharacterUpdateWorkflow
}

// NewCharacterUpdateAdapter 创建角色更新处理器适配器
func NewCharacterUpdateAdapter(workflow *workflows.CharacterUpdateWorkflow) *CharacterUpdateAdapter {
	return &CharacterUpdateAdapter{
		workflow: workflow,
	}
}

// TaskType 实现 TaskProcessor 接口
func (a *CharacterUpdateAdapter) TaskType() string {
	return "character_update"
}

// ProcessTask 实现 TaskProcessor 接口
func (a *CharacterUpdateAdapter) ProcessTask(ctx context.Context, task Task) error {
	// 从任务载荷获取参数
	payload := task.GetPayload()
	
	switch p := payload.(type) {
	case map[string]interface{}:
		characterName, nameOk := p["character_name"].(string)
		updateContent, _ := p["update_content"].(string) // 可选参数
		
		if !nameOk {
			return fmt.Errorf("缺少必要参数: character_name")
		}
		
		return a.workflow.ProcessCharacterUpdate(ctx, characterName, updateContent)
	default:
		return fmt.Errorf("不支持的载荷类型，需要map[string]interface{}，得到: %T", payload)
	}
}

// WorldviewSummarizerAdapter 世界观总结处理器适配器
type WorldviewSummarizerAdapter struct {
	workflow *workflows.WorldviewSummarizerWorkflow
}

// NewWorldviewSummarizerAdapter 创建世界观总结处理器适配器
func NewWorldviewSummarizerAdapter(workflow *workflows.WorldviewSummarizerWorkflow) *WorldviewSummarizerAdapter {
	return &WorldviewSummarizerAdapter{
		workflow: workflow,
	}
}

// TaskType 实现 TaskProcessor 接口
func (a *WorldviewSummarizerAdapter) TaskType() string {
	return "worldview_summarizer"
}

// ProcessTask 实现 TaskProcessor 接口
func (a *WorldviewSummarizerAdapter) ProcessTask(ctx context.Context, task Task) error {
	// 从任务载荷获取参数
	payload := task.GetPayload()
	
	switch p := payload.(type) {
	case map[string]interface{}:
		updateContent, _ := p["update_content"].(string) // 可选参数
		return a.workflow.ProcessWorldviewSummarizer(ctx, updateContent)
	case string:
		// 如果载荷是字符串，作为更新内容处理
		return a.workflow.ProcessWorldviewSummarizer(ctx, p)
	default:
		// 如果没有载荷或载荷为nil，执行AI分析模式
		return a.workflow.ProcessWorldviewSummarizer(ctx, "")
	}
}