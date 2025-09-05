package workflows

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"

	"github.com/Kizunad/modular-workflow-v2/components/agents/tools"
	"github.com/Kizunad/modular-workflow-v2/components/common"
	"github.com/Kizunad/modular-workflow-v2/components/content"
	"github.com/Kizunad/modular-workflow-v2/components/content/token"
	"github.com/Kizunad/modular-workflow-v2/logger"
	"github.com/Kizunad/modular-workflow-v2/providers"
)

// SummarizerWorkflowConfig 摘要工作流配置
type SummarizerWorkflowConfig struct {
	Logger       *logger.ZapLogger
	NovelDir     string // 小说目录路径
	LLMManager   *providers.Manager
	ShowProgress bool
	Model        string // 模型名称
}

// SummarizerWorkflow 摘要工作流
type SummarizerWorkflow struct {
	config *SummarizerWorkflowConfig
	cli    *common.CLIHelper
}

// NewSummarizerWorkflow 创建摘要工作流
func NewSummarizerWorkflow(config *SummarizerWorkflowConfig) *SummarizerWorkflow {
	if config == nil {
		config = &SummarizerWorkflowConfig{}
	}

	if config.Model == "" {
		config.Model = "qwen3:4b"
	}

	return &SummarizerWorkflow{
		config: config,
		cli:    common.NewCLIHelper("章节摘要工作流", "AI驱动的章节摘要生成系统"),
	}
}

// CreateReActAgent 创建摘要 ReAct Agent
func (sw *SummarizerWorkflow) CreateReActAgent() (*react.Agent, error) {
	ctx := context.Background()

	// 获取模型
	model, err := sw.config.LLMManager.GetOllamaModel(ctx, providers.WithModel(sw.config.Model))
	if err != nil {
		if sw.config.Logger != nil {
			sw.config.Logger.Warn(fmt.Sprintf("获取模型 %s 失败，尝试备用模型: %v", sw.config.Model, err))
		}
		// 备用方案使用默认模型
		model, err = sw.config.LLMManager.GetOllamaModel(ctx)
		if err != nil {
			return nil, fmt.Errorf("所有模型都不可用: %w", err)
		}
	}

	// 创建摘要管理工具
	summaryTool := tools.NewSummaryCRUDTool(sw.config.NovelDir, sw.config.LLMManager)
	chapterTool := tools.NewCurrentChapterCRUDTool(sw.config.NovelDir)
	chapterAnalysisTool := tools.NewChapterAnalysisTool(sw.config.NovelDir)

	// 创建工具节点配置
	toolsNodeConfig := &compose.ToolsNodeConfig{
		Tools: []tool.BaseTool{summaryTool, chapterTool, chapterAnalysisTool},
		ExecuteSequentially: false,
	}

	// 创建 ReAct Agent 配置
	agentConfig := &react.AgentConfig{
		ToolCallingModel: model,
		ToolsConfig:      *toolsNodeConfig,
		MaxStep:          10, // 摘要生成通常步骤较少
		MessageModifier:  sw.createMessageModifier(),
	}

	// 创建 ReAct Agent
	return react.NewAgent(ctx, agentConfig)
}

// createMessageModifier 创建消息修饰器
func (sw *SummarizerWorkflow) createMessageModifier() react.MessageModifier {
	// 预先加载上下文数据
	ctxData := sw.getContextData()
	// 使用默认摘要生成提示词
	prompt := `你是一个专业的小说章节摘要分析师。你的任务是：

1. 分析章节内容并提取关键信息
2. 生成结构化的章节摘要
3. 识别重要角色、地点、事件
4. 更新章节索引

工作流程：
1. 获取需要摘要的章节内容
2. 提取章节基本信息（标题、字数等）
3. 使用AI生成结构化摘要
4. 更新索引文件

摘要格式要求：
- 关键事件: 列出2-3个主要事件
- 主要角色: 识别重要角色
- 重要地点: 记录关键场景
- 情节进展: 简述推进的主要情节`

	return func(ctx context.Context, input []*schema.Message) []*schema.Message {
		sysPrompt := fmt.Sprintf(
			`%v

当前上下文信息:
- 章节信息: %v
- 角色信息: %v
- 世界观: %v
- 已有索引: %v

请根据用户要求进行章节摘要分析和生成。`,
			prompt,
			ctxData["chapter"],
			ctxData["characters"],
			ctxData["worldview"],
			ctxData["summary"],
		)
		
		result := make([]*schema.Message, 0, len(input)+1)
		result = append(result, schema.SystemMessage(sysPrompt))
		result = append(result, input...)
		return result
	}
}

// getContextData 获取上下文数据
func (sw *SummarizerWorkflow) getContextData() map[string]any {
	var cfg = content.ContextConfig{
		NovelDir: sw.config.NovelDir,
		Logger:   sw.config.Logger,
	}

	var cb = content.NewContextBuilder(&cfg)
	const maxTokens = 32000 // 摘要任务相对简单，token需求较少
	var percentage = token.TokenPercentages{
		Plan:      0.05, // 很少规划信息
		Character: 0.15, // 适量角色信息
		Worldview: 0.10, // 少量世界观
		Chapters:  0.60, // 重点关注章节内容
		Index:     0.10, // 已有摘要索引
	}

	// 生成上下文数据结构体
	var data, err = cb.BuildTokenAwareContext(&percentage, maxTokens)
	if err != nil {
		if sw.config.Logger != nil {
			sw.config.Logger.Error("构建上下文失败", zap.Error(err))
		}
		// 返回空上下文，但不中断流程
		return map[string]any{
			"chapter":    "",
			"characters": "",
			"worldview":  "",
			"summary":    "",
		}
	}

	if sw.config.ShowProgress {
		sw.cli.ShowInfo("📖", "上下文加载完成")
	}

	// 使用ContextBuilder的GetContextAsMap方法
	return cb.GetContextAsMap(data)
}

// ExecuteWithMonitoring 执行摘要工作流并提供监控
func (sw *SummarizerWorkflow) ExecuteWithMonitoring(input string) (string, error) {
	agent, err := sw.CreateReActAgent()
	if err != nil {
		return "", fmt.Errorf("创建 ReAct Agent 失败: %w", err)
	}

	ctx := context.Background()

	// 创建 MessageFuture 选项进行监控
	option, future := react.WithMessageFuture()

	// 执行 Agent
	response, err := agent.Generate(ctx, []*schema.Message{
		schema.UserMessage(input),
	}, option)

	if err != nil {
		return "", err
	}

	// 如果开启进度显示，展示执行步骤
	if sw.config.ShowProgress {
		sw.showExecutionSteps(future)
	}

	return response.Content, nil
}

// showExecutionSteps 显示执行步骤
func (sw *SummarizerWorkflow) showExecutionSteps(future react.MessageFuture) {
	iter := future.GetMessages()
	stepCount := 0
	
	for {
		msg, hasNext, err := iter.Next()
		if err != nil {
			break
		}
		if !hasNext {
			break
		}

		stepCount++
		if sw.config.Logger != nil {
			sw.config.Logger.Info(fmt.Sprintf("步骤 %d: Role=%s, Content=%s, ToolCalls=%d",
				stepCount, msg.Role, truncateContent(msg.Content, 100), len(msg.ToolCalls)))
		}

		// 显示工具调用信息
		if len(msg.ToolCalls) > 0 {
			for _, toolCall := range msg.ToolCalls {
				if sw.config.Logger != nil {
					sw.config.Logger.Debug(fmt.Sprintf("工具调用: %s", toolCall.Function.Name))
				}
			}
		}
	}
}

// ProcessSummarize 处理摘要任务（兼容原有接口）
func (sw *SummarizerWorkflow) ProcessSummarize(ctx context.Context, chapterContent string) error {
	input := fmt.Sprintf("请为以下章节内容生成摘要：\n\n%s", chapterContent)
	_, err := sw.ExecuteWithMonitoring(input)
	return err
}

// ProcessSummarizeByID 通过章节ID处理摘要任务
func (sw *SummarizerWorkflow) ProcessSummarizeByID(ctx context.Context, chapterID string) error {
	input := fmt.Sprintf("请为章节 %s 生成摘要", chapterID)
	_, err := sw.ExecuteWithMonitoring(input)
	return err
}

// ProcessLatestChapterSummary 处理最新章节摘要任务
func (sw *SummarizerWorkflow) ProcessLatestChapterSummary(ctx context.Context) error {
	input := "请为最新章节生成摘要"
	_, err := sw.ExecuteWithMonitoring(input)
	return err
}

