package providers

import (
	"context"

	"github.com/cloudwego/eino/components/model"
	"go.uber.org/zap"

	"github.com/Kizunad/modular-workflow-v2/config"
	"github.com/Kizunad/modular-workflow-v2/logger"
)

// Manager LLM管理器实现 - 简单的fallback逻辑
type Manager struct {
	ollama *OllamaProvider
	openai *OpenAIProvider
	logger logger.ZapLogger
}

// NewManager 创建LLM管理器
func NewManager(config *config.Config, logger logger.ZapLogger) *Manager {
	return &Manager{
		ollama: NewOllamaProvider(&config.LLM.Ollama, logger),
		openai: NewOpenAIProvider(&config.LLM.OpenAI, logger),
		logger: logger,
	}
}

// GetOllamaModel 获取Ollama模型（带fallback逻辑）
// 先尝试Ollama，不可用时自动切换到OpenAI
func (m *Manager) GetOllamaModel(ctx context.Context, options ...ProviderOption) (model.ToolCallingChatModel, error) {
	/*不使用HealthCheck*/
	model, err := m.ollama.GetModel(ctx, options...)
	if err != nil {
		m.logger.Warn("🔄 Ollama模型获取失败，切换到OpenAI", zap.Error(err))
		return m.GetOpenAIModel(ctx, options...)
	}
	return model, nil
}

// GetOpenAIModel 直接获取OpenAI模型（不做fallback）
func (m *Manager) GetOpenAIModel(ctx context.Context, options ...ProviderOption) (model.ToolCallingChatModel, error) {
	/*不使用HealthCheck*/
	m.logger.Debug("使用 OpenAI 模型")
	return m.openai.GetModel(ctx, options...)
}

// GetOllamaProvider 获取Ollama提供商（用于测试等场景）
func (m *Manager) GetOllamaProvider() *OllamaProvider {
	return m.ollama
}

// GetOpenAIProvider 获取OpenAI提供商（用于测试等场景）
func (m *Manager) GetOpenAIProvider() *OpenAIProvider {
	return m.openai
}
