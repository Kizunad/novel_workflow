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

// WorldviewSummarizerWorkflowConfig 世界观总结工作流配置
type WorldviewSummarizerWorkflowConfig struct {
	Logger       *logger.ZapLogger
	NovelDir     string // 小说目录路径
	LLMManager   *providers.Manager
	ShowProgress bool
	Model        string // 模型名称
}

// WorldviewSummarizerWorkflow 世界观总结工作流
type WorldviewSummarizerWorkflow struct {
	config *WorldviewSummarizerWorkflowConfig
	cli    *common.CLIHelper
}

// NewWorldviewSummarizerWorkflow 创建世界观总结工作流
func NewWorldviewSummarizerWorkflow(config *WorldviewSummarizerWorkflowConfig) *WorldviewSummarizerWorkflow {
	if config == nil {
		config = &WorldviewSummarizerWorkflowConfig{}
	}

	if config.Model == "" {
		config.Model = "qwen3:4b"
	}

	return &WorldviewSummarizerWorkflow{
		config: config,
		cli:    common.NewCLIHelper("世界观总结工作流", "AI驱动的世界观设定更新系统"),
	}
}

// CreateReActAgent 创建世界观总结 ReAct Agent
func (ww *WorldviewSummarizerWorkflow) CreateReActAgent() (*react.Agent, error) {
	ctx := context.Background()

	// 获取模型
	model, err := ww.config.LLMManager.GetOllamaModel(ctx, providers.WithModel(ww.config.Model))
	if err != nil {
		if ww.config.Logger != nil {
			ww.config.Logger.Warn(fmt.Sprintf("获取模型 %s 失败，尝试备用模型: %v", ww.config.Model, err))
		}
		// 备用方案使用默认模型
		model, err = ww.config.LLMManager.GetOllamaModel(ctx)
		if err != nil {
			return nil, fmt.Errorf("所有模型都不可用: %w", err)
		}
	}

	// 创建世界观管理工具
	worldviewTool := tools.NewWorldviewCRUDTool(ww.config.NovelDir, ww.config.LLMManager)
	chapterTool := tools.NewCurrentChapterCRUDTool(ww.config.NovelDir)
	chapterAnalysisTool := tools.NewChapterAnalysisTool(ww.config.NovelDir)
	planTool := tools.NewPlanCRUDTool(ww.config.NovelDir)

	// 创建工具节点配置
	toolsNodeConfig := &compose.ToolsNodeConfig{
		Tools: []tool.BaseTool{worldviewTool, chapterTool, chapterAnalysisTool, planTool},
		ExecuteSequentially: false,
	}

	// 创建 ReAct Agent 配置
	agentConfig := &react.AgentConfig{
		ToolCallingModel: model,
		ToolsConfig:      *toolsNodeConfig,
		MaxStep:          15, // 世界观分析可能需要多步推理
		MessageModifier:  ww.createMessageModifier(),
	}

	// 创建 ReAct Agent
	return react.NewAgent(ctx, agentConfig)
}

// createMessageModifier 创建消息修饰器
func (ww *WorldviewSummarizerWorkflow) createMessageModifier() react.MessageModifier {
	// 预先加载上下文数据
	ctxData := ww.getContextData()
	// 使用默认世界观总结提示词
	prompt := `你是一个专业的小说世界观分析专家。你的任务是：

1. 分析最新章节内容中的世界设定信息
2. 识别新的魔法系统、地理信息、种族设定等
3. 判断是否需要更新世界观文档
4. 如果需要更新，生成结构化的世界观内容

工作流程：
1. 首先获取最新章节内容
2. 读取当前世界观设定
3. 使用AI分析世界观变化
4. 如果发现新设定，执行合并更新操作

更新原则：
- 保持原有格式结构
- 对现有世界补充使用[UPDATE]标记
- 全新世界创建新的编号条目
- 只有在确实发现新设定时才更新`

	return func(ctx context.Context, input []*schema.Message) []*schema.Message {
		sysPrompt := fmt.Sprintf(
			`%v

当前上下文信息:
- 最新世界观设定: %v
- 章节信息: %v
- 规划信息: %v
- 总结: %v

请根据用户要求分析世界观变化并进行相应操作。`,
			prompt,
			ctxData["worldview"],
			ctxData["chapter"],
			ctxData["plan"],
			ctxData["summary"],
		)
		
		result := make([]*schema.Message, 0, len(input)+1)
		result = append(result, schema.SystemMessage(sysPrompt))
		result = append(result, input...)
		return result
	}
}

// getContextData 获取上下文数据
func (ww *WorldviewSummarizerWorkflow) getContextData() map[string]any {
	var cfg = content.ContextConfig{
		NovelDir: ww.config.NovelDir,
		Logger:   ww.config.Logger,
	}

	var cb = content.NewContextBuilder(&cfg)
	const maxTokens = 64000 // 适合世界观分析的token限制
	var percentage = token.TokenPercentages{
		Plan:      0.15, // 适量规划信息
		Character: 0.10, // 减少角色信息
		Worldview: 0.40, // 重点关注世界观
		Chapters:  0.25, // 章节上下文
		Index:     0.10,
	}

	// 生成上下文数据结构体
	var data, err = cb.BuildTokenAwareContext(&percentage, maxTokens)
	if err != nil {
		if ww.config.Logger != nil {
			ww.config.Logger.Error("构建上下文失败", zap.Error(err))
		}
		// 返回空上下文，但不中断流程
		return map[string]any{
			"chapter":   "",
			"worldview": "",
			"plan":      "",
			"summary":   "",
		}
	}

	if ww.config.ShowProgress {
		ww.cli.ShowInfo("📖", "上下文加载完成")
	}

	// 使用ContextBuilder的GetContextAsMap方法
	return cb.GetContextAsMap(data)
}

// ExecuteWithMonitoring 执行世界观总结工作流并提供监控
func (ww *WorldviewSummarizerWorkflow) ExecuteWithMonitoring(input string) (string, error) {
	agent, err := ww.CreateReActAgent()
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
	if ww.config.ShowProgress {
		ww.showExecutionSteps(future)
	}

	return response.Content, nil
}

// showExecutionSteps 显示执行步骤
func (ww *WorldviewSummarizerWorkflow) showExecutionSteps(future react.MessageFuture) {
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
		if ww.config.Logger != nil {
			ww.config.Logger.Info(fmt.Sprintf("步骤 %d: Role=%s, Content=%s, ToolCalls=%d",
				stepCount, msg.Role, truncateContent(msg.Content, 100), len(msg.ToolCalls)))
		}

		// 显示工具调用信息
		if len(msg.ToolCalls) > 0 {
			for _, toolCall := range msg.ToolCalls {
				if ww.config.Logger != nil {
					ww.config.Logger.Debug(fmt.Sprintf("工具调用: %s", toolCall.Function.Name))
				}
			}
		}
	}
}

// ProcessWorldviewSummarizer 处理世界观总结任务（兼容原有接口）
func (ww *WorldviewSummarizerWorkflow) ProcessWorldviewSummarizer(ctx context.Context, updateContent string) error {
	var input string
	if updateContent != "" {
		// 直接更新模式
		input = fmt.Sprintf("请直接更新世界观设定，更新内容：%s", updateContent)
	} else {
		// AI分析模式
		input = "请分析最新章节内容中的世界设定信息，并根据需要更新世界观文档"
	}

	_, err := ww.ExecuteWithMonitoring(input)
	return err
}

