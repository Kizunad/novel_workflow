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
	"github.com/Kizunad/modular-workflow-v2/components/content/managers"
	"github.com/Kizunad/modular-workflow-v2/components/content/token"
	"github.com/Kizunad/modular-workflow-v2/logger"
	"github.com/Kizunad/modular-workflow-v2/providers"
)

// WriteWorkflowConfig 写作工作流配置
type WriteWorkflowConfig struct {
	Logger       *logger.ZapLogger
	NovelDir     string // 小说目录路径
	LLMManager   *providers.Manager
	ShowProgress bool
	WriterModel  string // 写作模型名称
}

// WriteWorkflow 写作工作流
type WriteWorkflow struct {
	config *WriteWorkflowConfig
	cli    *common.CLIHelper
}

// NewWriteWorkflow 创建写作工作流
func NewWriteWorkflow(config *WriteWorkflowConfig) *WriteWorkflow {
	if config == nil {
		config = &WriteWorkflowConfig{}
	}

	if config.WriterModel == "" {
		config.WriterModel = "deepseek-chat"
	}

	return &WriteWorkflow{
		config: config,
		cli:    common.NewCLIHelper("写作工作流", "AI驱动的章节创作系统"),
	}
}

// CreateReActAgent 创建写作 ReAct Agent
func (ww *WriteWorkflow) CreateReActAgent() (*react.Agent, error) {
	ctx := context.Background()

	writerModel, err := ww.config.LLMManager.GetOpenAIModel(ctx, providers.WithModel(ww.config.WriterModel))
	if err != nil {
		if ww.config.Logger != nil {
			ww.config.Logger.Warn(fmt.Sprintf("获取规划模型 %s 失败，尝试备用模型: %v", ww.config.WriterModel, err))
		}
		// 备用方案使用默认OpenAI模型
		writerModel, err = ww.config.LLMManager.GetOpenAIModel(ctx)
		if err != nil {
			return nil, fmt.Errorf("所有规划模型都不可用: %w", err)
		}
	}

	// 创建章节管理工具
	currentChapterTool := tools.NewCurrentChapterCRUDTool(ww.config.NovelDir)
	planCRUDTool := tools.NewPlanCRUDTool(ww.config.NovelDir)

	// 创建工具节点配置
	toolsNodeConfig := &compose.ToolsNodeConfig{
		Tools:               []tool.BaseTool{planCRUDTool, currentChapterTool},
		ExecuteSequentially: false,
	}

	// 创建 ReAct Agent 配置
	agentConfig := &react.AgentConfig{
		ToolCallingModel: writerModel,
		ToolsConfig:      *toolsNodeConfig,
		MaxStep:          10,
		MessageModifier:  ww.createMessageModifier(),
	}

	// 创建 ReAct Agent
	return react.NewAgent(ctx, agentConfig)
}

func (ww *WriteWorkflow) createMessageModifier() react.MessageModifier {
	// 预先加载上下文数据，避免重复调用
	ctxData := ww.getContextData()
	prompt := content.Novel_writer_prompt
	planManager := managers.NewPlannerContentManager(ww.config.NovelDir)

	firstPlan, found := planManager.GetFirstUnfinishedPlan()
	if !found {
		ww.config.Logger.Warn("没有找到未完成的计划")
	} else {
		ww.config.Logger.Info("当前plan:", zap.String("plan", firstPlan.Plan))
	}
	return func(ctx context.Context, input []*schema.Message) []*schema.Message {
		var planInfo string
		if found {
			planInfo = fmt.Sprintf("章节:%s 规划:%s", firstPlan.Chapter, firstPlan.Plan)
		} else {
			planInfo = "无未完成计划"
		}

		sysPrompt := fmt.Sprintf(
			`%v,当前上下文:-章节:%v-世界观:%v-角色:%v-现有规划:%v-总结: %v`,
			prompt,
			ctxData["chapter"],
			ctxData["worldview"],
			ctxData["characters"],
			planInfo,
			ctxData["summary"],
		)
		result := make([]*schema.Message, 0, len(input)+1)
		result = append(result, schema.SystemMessage(sysPrompt))
		result = append(result, input...)
		return result
	}
}

// getContextData 获取上下文数据
func (ww *WriteWorkflow) getContextData() map[string]any {
	var cfg = content.ContextConfig{
		NovelDir: ww.config.NovelDir,
		Logger:   ww.config.Logger,
	}

	var cb = content.NewContextBuilder(&cfg)
	const maxTokens = 128000 // 适合写作任务的token限制
	var percentage = token.TokenPercentages{
		Plan:      0.15, // 适合写作的规划信息
		Character: 0.05,
		Worldview: 0.05,
		Chapters:  0.60, // 更多章节上下文
		Index:     0.15,
	}

	// 生成上下文数据结构体
	var data, err = cb.BuildTokenAwareContext(&percentage, maxTokens)
	if err != nil {
		panic(fmt.Errorf("%w", err))
	}

	if ww.config.ShowProgress {
		ww.cli.ShowInfo("📖", "上下文加载完成")
	}

	// 使用ContextBuilder的GetContextAsMap方法确保键名正确
	return cb.GetContextAsMap(data)
}

func (ww *WriteWorkflow) ExecuteWithMonitoring(input string) (string, error) {
	agent, err := ww.CreateReActAgent()
	if err != nil {
		return "", fmt.Errorf("创建 ReAct Agent 失败: %w", err)
	}

	ctx := context.Background()

	// 创建 MessageFuture 选项
	option, future := react.WithMessageFuture()

	// 执行 Agent
	response, err := agent.Generate(ctx, []*schema.Message{
		schema.UserMessage(input),
	}, option)

	if err != nil {
		return "", err
	}

	// 获取每次循环的消息
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
		ww.config.Logger.Info(fmt.Sprintf("步骤 %d: Role=%s, Content=%s, ToolCalls=%d",
			stepCount, msg.Role, msg.Content, len(msg.ToolCalls)))
	}

	return response.Content, nil
}
