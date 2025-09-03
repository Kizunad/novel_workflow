package queue

import (
	"fmt"

	"github.com/Kizunad/modular-workflow-v2/character_update"
	"github.com/Kizunad/modular-workflow-v2/config"
	"github.com/Kizunad/modular-workflow-v2/logger"
	"github.com/Kizunad/modular-workflow-v2/providers"
	"github.com/Kizunad/modular-workflow-v2/summarizer"
	"github.com/Kizunad/modular-workflow-v2/worldview_summarizer"
)

// InitQueue 初始化队列并注册所有 Worker
func InitQueue(
	cfg *config.MessageQueueConfig,
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
	
	// 注册 Summarizer 处理器
	summarizerProcessor := summarizer.NewProcessor(llmManager, logger)
	summarizerAdapter := NewSummarizerAdapter(summarizerProcessor)
	mq.Register(summarizerAdapter)
	
	// 注册 CharacterUpdate 处理器
	characterUpdateProcessor := character_update.NewProcessor(llmManager, logger)
	characterUpdateAdapter := NewCharacterUpdateAdapter(characterUpdateProcessor)
	mq.Register(characterUpdateAdapter)
	
	// 注册 WorldviewSummarizer 处理器
	worldviewSummarizerProcessor := worldview_summarizer.NewProcessor(llmManager, logger)
	worldviewSummarizerAdapter := NewWorldviewSummarizerAdapter(worldviewSummarizerProcessor)
	mq.Register(worldviewSummarizerAdapter)
	
	// 未来可以注册更多处理器
	// indexProcessor := indexer.NewProcessor(...)
	// mq.Register(indexProcessor)
	
	// backupProcessor := backup.NewProcessor(...)
	// mq.Register(backupProcessor)
	
	logger.Info(fmt.Sprintf("队列初始化完成，注册了 %d 个处理器", len(mq.processors)))
	
	return mq, nil
}

// Helper 创建摘要任务的辅助函数
func CreateSummarizeTask(novelPath, content string) *summarizer.SummarizeTask {
	return summarizer.NewSummarizeTask(novelPath, content)
}

// Helper 创建角色更新任务的辅助函数
func CreateCharacterUpdateTask(novelPath, characterName, updateContent string) *character_update.CharacterUpdateTask {
	return character_update.NewCharacterUpdateTask(novelPath, characterName, updateContent)
}

// Helper 创建世界观总结任务的辅助函数
func CreateWorldviewSummarizerTask(novelPath, updateContent string) *worldview_summarizer.WorldviewSummarizerTask {
	return worldview_summarizer.NewWorldviewSummarizerTask(novelPath, updateContent)
}