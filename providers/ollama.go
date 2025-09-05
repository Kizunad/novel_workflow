package providers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Kizunad/modular-workflow-v2/config"
	"github.com/Kizunad/modular-workflow-v2/logger"
	"github.com/cloudwego/eino-ext/components/model/ollama"
	"github.com/cloudwego/eino/components/model"

	"go.uber.org/zap"
)

// ProviderOption 提供商选项函数
type ProviderOption func(*providerOptions)

// providerOptions 提供商选项结构
type providerOptions struct {
	modelName string
}

// WithModel 指定模型名称
func WithModel(modelName string) ProviderOption {
	return func(opts *providerOptions) {
		opts.modelName = modelName
	}
}

type OllamaProvider struct {
	config *config.OllamaConfig
	logger logger.ZapLogger
}

func NewOllamaProvider(config *config.OllamaConfig, logger logger.ZapLogger) *OllamaProvider {
	return &OllamaProvider{
		config: config,
		logger: logger,
	}
}

// GetModel 实现LLMProvider接口，创建并返回Ollama模型
func (o *OllamaProvider) GetModel(ctx context.Context, options ...ProviderOption) (model.ToolCallingChatModel, error) {
	models := o.config.Models
	if len(models) == 0 {
		return nil, fmt.Errorf("Ollama模型配置为空")
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
		o.logger.Debug(fmt.Sprintf("未指定模型，自动选择第一个配置模型: %s", modelName))
	}

	o.logger.Info("初始化 OllamaModel",
		zap.String("base_url", o.config.BaseURL),
		zap.String("model", modelName))

	chatModelConfig := &ollama.ChatModelConfig{
		BaseURL: o.config.BaseURL,
		Model:   modelName,
	}

	chatModel, err := ollama.NewChatModel(ctx, chatModelConfig)
	if err != nil {
		o.logger.Error("初始化 OllamaModel 失败", zap.Error(err))
		return nil, fmt.Errorf("初始化 OllamaModel 失败: %w", err)
	}

	o.logger.Info("初始化 OllamaModel 成功")
	return chatModel, nil
}

// HealthCheck 实现LLMProvider接口，检查Ollama服务可用性
func (o *OllamaProvider) HealthCheck(ctx context.Context) error {
	o.logger.Debug("执行HealthCheck")

	// 创建带超时的context
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// 构建请求
	req, err := http.NewRequestWithContext(ctx, "GET", o.config.BaseURL+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("创建健康检查请求失败: %w", err)
	}

	// 发送请求
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		o.logger.Warn("Ollama健康检查失败", zap.Error(err))
		return fmt.Errorf("ollama不可用: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode == 503 {
		return fmt.Errorf("ollama服务忙碌 (503)")
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("ollama返回异常状态: %d", resp.StatusCode)
	}

	o.logger.Debug("Ollama健康检查通过")
	return nil
}

// GetConfig 获取配置（用于测试等场景）
func (o *OllamaProvider) GetConfig() *config.OllamaConfig {
	return o.config
}
