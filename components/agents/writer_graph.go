package agents

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/Kizunad/modular-workflow-v2/components/common"
	"github.com/Kizunad/modular-workflow-v2/components/content"
	"github.com/Kizunad/modular-workflow-v2/logger"
	"github.com/Kizunad/modular-workflow-v2/providers"
)

// WriterGraphConfig Writer Graph配置
type WriterGraphConfig struct {
	LLMManager     *providers.Manager
	RetryConfig    *common.RetryConfig
	Logger         *logger.ZapLogger
	ShowProgress   bool
	MinContentLen  int    // 最小内容长度
	RetryMessage   string // 重试消息
	FailureMessage string // 失败消息
}

// DefaultWriterGraphConfig 默认Writer Graph配置
func DefaultWriterGraphConfig() *WriterGraphConfig {
	return &WriterGraphConfig{
		RetryConfig:    common.HTTPRetryConfig(), // 使用HTTP优化配置
		MinContentLen:  50,
		RetryMessage:   "内容过短，正在重试 (第%d次)",
		FailureMessage: "重试%d次后仍然失败，内容生成不满足要求",
	}
}

// WriterGraphBuilder Writer Graph构建器
type WriterGraphBuilder struct {
	config  *WriterGraphConfig
	builder *common.WorkflowBuilder
	cli     *common.CLIHelper
}

// NewWriterGraphBuilder 创建Writer Graph构建器
func NewWriterGraphBuilder(config *WriterGraphConfig) *WriterGraphBuilder {
	if config == nil {
		config = DefaultWriterGraphConfig()
	}

	if config.RetryConfig == nil {
		config.RetryConfig = common.HTTPRetryConfig()
	}

	if config.MinContentLen <= 0 {
		config.MinContentLen = 50
	}

	builderConfig := &common.WorkflowConfig{
		Logger:          config.Logger,
		CheckPointStore: common.NewInMemoryCheckPointStore(),
		EnableProgress:  config.ShowProgress,
		ProgressPrefix:  "🔧",
	}

	return &WriterGraphBuilder{
		config:  config,
		builder: common.NewWorkflowBuilder(builderConfig),
		cli:     common.NewCLIHelper("AI写作器", "带重试机制的智能写作系统"),
	}
}

// CreateWriterGraphWithRetry 创建带重试机制的Writer Graph
func (wgb *WriterGraphBuilder) CreateWriterGraphWithRetry(ctx context.Context) (*compose.Graph[map[string]any, *schema.Message], error) {
	// 创建带状态的Graph
	graph := wgb.builder.CreateGraph()

	// 创建重试输入转换器
	retryInputConverter := wgb.createRetryInputConverter()

	// 获取ChatModel
	chatModel, err := wgb.getChatModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取聊天模型失败: %w", err)
	}

	// 创建重试输出检查器
	retryOutputChecker := wgb.createRetryOutputChecker()

	// 创建聊天模板
	chatTemplate := wgb.createChatTemplate()

	// 添加节点到Graph
	if err := graph.AddLambdaNode("retry_input_converter", retryInputConverter); err != nil {
		return nil, fmt.Errorf("添加重试输入转换节点失败: %w", err)
	}

	if err := graph.AddChatTemplateNode("template", chatTemplate); err != nil {
		return nil, fmt.Errorf("添加模板节点失败: %w", err)
	}

	if err := graph.AddChatModelNode("chat", chatModel); err != nil {
		return nil, fmt.Errorf("添加聊天模型节点失败: %w", err)
	}

	if err := graph.AddLambdaNode("retry_output_check", retryOutputChecker); err != nil {
		return nil, fmt.Errorf("添加重试输出检查节点失败: %w", err)
	}

	// 连接节点
	if err := wgb.connectNodes(graph); err != nil {
		return nil, fmt.Errorf("连接节点失败: %w", err)
	}

	return graph, nil
}

// createRetryInputConverter 创建重试输入转换器
func (wgb *WriterGraphBuilder) createRetryInputConverter() *compose.Lambda {
	return compose.InvokableLambda(func(ctx context.Context, input map[string]any) (map[string]any, error) {
		if wgb.config.ShowProgress {
			wgb.cli.ShowInfo("🔄", "处理输入数据...")
		}

		var currentRetryCount int
		var finalInput map[string]any

		err := compose.ProcessState[*common.WriterState](ctx, func(ctx context.Context, state *common.WriterState) error {
			state.RetryCount++
			currentRetryCount = state.RetryCount

			if currentRetryCount == 1 && len(input) > 0 {
				// 第一次执行，保存输入数据
				state.SavedInput = make(map[string]any)
				for k, v := range input {
					state.SavedInput[k] = v
				}
				finalInput = input
			} else if currentRetryCount > 1 && len(input) == 0 {
				// 重试时使用保存的输入数据
				finalInput = state.SavedInput
			} else {
				finalInput = input
			}
			return nil
		})

		if err != nil {
			return nil, err
		}

		if wgb.config.ShowProgress && currentRetryCount > 1 {
			wgb.cli.ShowInfo("⚠️", fmt.Sprintf("第%d次重试，使用保存的输入数据", currentRetryCount))
		}

		return finalInput, nil
	})
}

// createRetryOutputChecker 创建重试输出检查器
func (wgb *WriterGraphBuilder) createRetryOutputChecker() *compose.Lambda {
	return compose.InvokableLambda(func(ctx context.Context, result *schema.Message) (*schema.Message, error) {
		if wgb.config.ShowProgress {
			wgb.cli.ShowInfo("🔍", "检查输出质量...")
		}

		var currentRetryCount int

		err := compose.ProcessState[*common.WriterState](ctx, func(ctx context.Context, state *common.WriterState) error {
			currentRetryCount = state.RetryCount
			return nil
		})
		if err != nil {
			return nil, err
		}

		// 检查输出是否为空或过短
		if result.Content == "" || len(strings.TrimSpace(result.Content)) < wgb.config.MinContentLen {
			var shouldRetry bool
			err = compose.ProcessState[*common.WriterState](ctx, func(ctx context.Context, state *common.WriterState) error {
				state.LastResult = result.Content
				shouldRetry = state.RetryCount < wgb.config.RetryConfig.MaxRetries
				return nil
			})
			if err != nil {
				return nil, err
			}

			if !shouldRetry {
				return nil, fmt.Errorf(wgb.config.FailureMessage, currentRetryCount)
			}

			// 显示重试信息
			if wgb.config.ShowProgress {
				wgb.cli.ShowInfo("⚠️", fmt.Sprintf(wgb.config.RetryMessage, currentRetryCount))
			}

			// 抛出中断并重试错误
			return nil, compose.NewInterruptAndRerunErr(fmt.Sprintf("内容长度不足 (%d字符，需要至少%d字符)",
				len(strings.TrimSpace(result.Content)), wgb.config.MinContentLen))
		}

		// 成功时更新状态并显示结果
		err = compose.ProcessState[*common.WriterState](ctx, func(ctx context.Context, state *common.WriterState) error {
			state.LastResult = result.Content
			return nil
		})

		if wgb.config.ShowProgress {
			contentLen := len(strings.TrimSpace(result.Content))
			if currentRetryCount > 1 {
				wgb.cli.ShowSuccess(fmt.Sprintf("重试成功！生成内容长度: %d字符", contentLen))
			} else {
				wgb.cli.ShowSuccess(fmt.Sprintf("内容生成完成，长度: %d字符", contentLen))
			}
		}

		return result, err
	})
}

// getChatModel 获取聊天模型
func (wgb *WriterGraphBuilder) getChatModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	if wgb.config.LLMManager == nil {
		return nil, fmt.Errorf("LLMManager未配置")
	}

	// 优先使用 OpenAI，备用 Ollama
	chatModel, err := common.WithHTTPRetry(ctx,
		func(ctx context.Context) (model.ToolCallingChatModel, error) {
			return wgb.config.LLMManager.GetOpenAIModel(ctx)
		},
		func(attempt int, err error, delay time.Duration) {
			if wgb.config.Logger != nil {
				wgb.config.Logger.Warn(fmt.Sprintf("获取OpenAI模型重试 (第%d次): %v，延迟%v", attempt, err, delay))
			}
		},
	)

	if err != nil {
		if wgb.config.Logger != nil {
			wgb.config.Logger.Warn("OpenAI模型不可用，尝试Ollama: " + err.Error())
		}

		chatModel, err = wgb.config.LLMManager.GetOllamaModel(ctx)
		if err != nil {
			return nil, fmt.Errorf("所有LLM模型都不可用: %w", err)
		}
	}

	return chatModel, err
}

// createChatTemplate 创建聊天模板
func (wgb *WriterGraphBuilder) createChatTemplate() prompt.ChatTemplate {
	return prompt.FromMessages(
		schema.FString,
		schema.SystemMessage(string(content.Novel_writer_prompt)),
		schema.UserMessage("上下文：{context}\n\n用户要求：{input}\n\n写作策略：{writing_strategy}\n\n请根据策略开始创作："),
	)
}

// connectNodes 连接所有节点
func (wgb *WriterGraphBuilder) connectNodes(graph *compose.Graph[map[string]any, *schema.Message]) error {
	connections := [][]string{
		{compose.START, "retry_input_converter"},
		{"retry_input_converter", "template"},
		{"template", "chat"},
		{"chat", "retry_output_check"},
		{"retry_output_check", compose.END},
	}

	for _, conn := range connections {
		from, to := conn[0], conn[1]
		if err := graph.AddEdge(from, to); err != nil {
			return fmt.Errorf("连接 %s -> %s 失败: %w", from, to, err)
		}
	}

	return nil
}