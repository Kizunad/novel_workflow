package providers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"go.uber.org/zap"

	"github.com/Kizunad/modular-workflow-v2/config"
	"github.com/Kizunad/modular-workflow-v2/logger"
)

// OpenAIProvider OpenAI模型提供商实现
type OpenAIProvider struct {
	config *config.OpenAIConfig
	logger logger.ZapLogger
}

// NewOpenAIProvider 创建OpenAI提供商实例
func NewOpenAIProvider(config *config.OpenAIConfig, logger logger.ZapLogger) *OpenAIProvider {
	return &OpenAIProvider{
		config: config,
		logger: logger,
	}
}

// GetModel 实现LLMProvider接口，创建并返回OpenAI模型
func (p *OpenAIProvider) GetModel(ctx context.Context, options ...ProviderOption) (model.ToolCallingChatModel, error) {
	models := p.config.Models
	if len(models) == 0 {
		return nil, fmt.Errorf("OpenAI模型配置为空")
	}

	// 应用选项
	opts := &providerOptions{}
	for _, option := range options {
		option(opts)
	}

	// 选择模型
	modelName := opts.modelName
	if modelName == "" {
		modelName = models[0] // 使用第一个配置的模型
		p.logger.Debug(fmt.Sprintf("未指定模型，自动选择第一个配置模型: %s", modelName))
	}

	p.logger.Info("初始化 OpenAI Model",
		zap.String("base_url", p.config.BaseURL),
		zap.String("model", modelName),
		zap.String("api_key", "***hidden***"))

	chatModelConfig := &openai.ChatModelConfig{
		APIKey:  p.config.APIKey,
		Model:   modelName,
		BaseURL: p.config.BaseURL,
	}

	chatModel, err := openai.NewChatModel(ctx, chatModelConfig)
	if err != nil {
		p.logger.Error("初始化 OpenAI Model 失败", zap.Error(err))
		return nil, fmt.Errorf("初始化 OpenAI Model 失败: %w", err)
	}

	return chatModel, nil
}

// HealthCheck 实现LLMProvider接口，检查OpenAI服务可用性
// 只有两种状态：可用(nil) 或 不可用(error)
func (p *OpenAIProvider) HealthCheck(ctx context.Context) error {
	p.logger.Debug("执行 OpenAI HealthCheck")

	// 创建带超时的context
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// 构建健康检查请求 - new-api使用/api/status端点
	// 需要从BaseURL中移除/v1路径，因为健康检查端点在根路径下
	healthURL := strings.TrimSuffix(strings.TrimSuffix(p.config.BaseURL, "/"), "/v1")
	fullHealthURL := healthURL + "/api/status"
	req, err := http.NewRequestWithContext(ctx, "GET", fullHealthURL, nil)
	if err != nil {
		return fmt.Errorf("openai不可用")
	}

	// 添加认证头
	if p.config.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	}

	// 发送请求
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		p.logger.Error("健康检查请求失败", 
			zap.Error(err), 
			zap.String("url", fullHealthURL))
		return fmt.Errorf("openai不可用: %v", err)
	}
	defer resp.Body.Close()

	// 记录响应详情
	p.logger.Debug("健康检查响应", 
		zap.Int("status_code", resp.StatusCode), 
		zap.String("url", fullHealthURL))

	// 简单判断：200 = 可用，其他 = 不可用
	if resp.StatusCode != 200 {
		p.logger.Error("健康检查状态码错误", 
			zap.Int("status_code", resp.StatusCode), 
			zap.String("url", p.config.BaseURL+"/api/status"))
		return fmt.Errorf("openai不可用: 状态码 %d", resp.StatusCode)
	}

	p.logger.Debug("OpenAI 健康检查通过")
	return nil
}

// GetConfig 获取配置（用于测试等场景）
func (p *OpenAIProvider) GetConfig() *config.OpenAIConfig {
	return p.config
}

