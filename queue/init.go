package queue

import (
	"fmt"

	"github.com/Kizunad/modular-workflow-v2/components/workflows"
	"github.com/Kizunad/modular-workflow-v2/config"
	"github.com/Kizunad/modular-workflow-v2/logger"
	"github.com/Kizunad/modular-workflow-v2/providers"
)

// InitQueue 初始化队列并注册所有 Worker
func InitQueue(
	cfg *config.MessageQueueConfig,
	novelDir string,
	llmManager *providers.Manager,
	logger *logger.ZapLogger,
) (*MessageQueue, error) {
	
	if !cfg.Enabled {
		logger.Info("消息队列被禁用，跳过初始化")
		return nil, nil
	}
	
	// 创建消息队列
	queueConfig := NewConfig(cfg)
	mq := New(queueConfig, logger)
	
	// 注册 Summarizer 工作流
	summarizerWorkflow := workflows.NewSummarizerWorkflow(&workflows.SummarizerWorkflowConfig{
		Logger:       logger,
		NovelDir:     novelDir,
		LLMManager:   llmManager,
		ShowProgress: true,
		Model:        "qwen3:4b",
	})
	summarizerAdapter := NewSummarizerAdapter(summarizerWorkflow)
	mq.Register(summarizerAdapter)
	
	// 注册 CharacterUpdate 工作流
	characterUpdateWorkflow := workflows.NewCharacterUpdateWorkflow(&workflows.CharacterUpdateWorkflowConfig{
		Logger:       logger,
		NovelDir:     novelDir,
		LLMManager:   llmManager,
		ShowProgress: true,
		Model:        "qwen3:4b",
	})
	characterUpdateAdapter := NewCharacterUpdateAdapter(characterUpdateWorkflow)
	mq.Register(characterUpdateAdapter)
	
	// 注册 WorldviewSummarizer 工作流
	worldviewSummarizerWorkflow := workflows.NewWorldviewSummarizerWorkflow(&workflows.WorldviewSummarizerWorkflowConfig{
		Logger:       logger,
		NovelDir:     novelDir,
		LLMManager:   llmManager,
		ShowProgress: true,
		Model:        "qwen3:4b",
	})
	worldviewSummarizerAdapter := NewWorldviewSummarizerAdapter(worldviewSummarizerWorkflow)
	mq.Register(worldviewSummarizerAdapter)
	
	// 未来可以注册更多处理器
	// indexProcessor := indexer.NewProcessor(...)
	// mq.Register(indexProcessor)
	
	// backupProcessor := backup.NewProcessor(...)
	// mq.Register(backupProcessor)
	
	logger.Info(fmt.Sprintf("队列初始化完成，注册了 %d 个处理器", len(mq.processors)))
	
	return mq, nil
}

// GenericTask 通用任务实现
type GenericTask struct {
	ID       string
	Type     string
	Priority int
	Payload  interface{}
}

// GetID 实现 Task 接口
func (t *GenericTask) GetID() string {
	return t.ID
}

// GetType 实现 Task 接口
func (t *GenericTask) GetType() string {
	return t.Type
}

// GetPriority 实现 Task 接口
func (t *GenericTask) GetPriority() int {
	return t.Priority
}

// GetPayload 实现 Task 接口
func (t *GenericTask) GetPayload() interface{} {
	return t.Payload
}

// Helper 创建摘要任务的辅助函数
func CreateSummarizeTask(taskID, chapterContent string) Task {
	return &GenericTask{
		ID:       taskID,
		Type:     "summarize",
		Priority: 5,
		Payload:  chapterContent,
	}
}

// Helper 创建通过章节ID摘要任务的辅助函数
func CreateSummarizeByIDTask(taskID, chapterID string) Task {
	return &GenericTask{
		ID:       taskID,
		Type:     "summarize",
		Priority: 5,
		Payload: map[string]interface{}{
			"chapter_id": chapterID,
		},
	}
}

// Helper 创建最新章节摘要任务的辅助函数
func CreateLatestChapterSummarizeTask(taskID string) Task {
	return &GenericTask{
		ID:       taskID,
		Type:     "summarize",
		Priority: 5,
		Payload:  map[string]interface{}{},
	}
}

// Helper 创建角色更新任务的辅助函数
func CreateCharacterUpdateTask(taskID, characterName, updateContent string) Task {
	return &GenericTask{
		ID:       taskID,
		Type:     "character_update",
		Priority: 5,
		Payload: map[string]interface{}{
			"character_name": characterName,
			"update_content": updateContent,
		},
	}
}

// Helper 创建世界观总结任务的辅助函数
func CreateWorldviewSummarizerTask(taskID, updateContent string) Task {
	return &GenericTask{
		ID:       taskID,
		Type:     "worldview_summarizer",
		Priority: 5,
		Payload: map[string]interface{}{
			"update_content": updateContent,
		},
	}
}

// Helper 创建AI分析世界观任务的辅助函数
func CreateWorldviewAnalysisTask(taskID string) Task {
	return &GenericTask{
		ID:       taskID,
		Type:     "worldview_summarizer",
		Priority: 5,
		Payload:  map[string]interface{}{},
	}
}