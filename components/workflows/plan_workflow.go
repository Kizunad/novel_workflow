package workflows

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/flow/agent/react"
	"github.com/cloudwego/eino/schema"

	"github.com/Kizunad/modular-workflow-v2/components/agents/tools"
	"github.com/Kizunad/modular-workflow-v2/components/common"
	"github.com/Kizunad/modular-workflow-v2/components/content"
	"github.com/Kizunad/modular-workflow-v2/components/content/token"
	"github.com/Kizunad/modular-workflow-v2/logger"
	"github.com/Kizunad/modular-workflow-v2/providers"
)

// PlanWorkflowConfig 规划工作流配置
type PlanWorkflowConfig struct {
	Logger       *logger.ZapLogger
	NovelDir     string // 小说目录路径
	LLMManager   *providers.Manager
	ShowProgress bool
	PlannerModel string // 规划模型名称
}

// PlanWorkflow 规划工作流
type PlanWorkflow struct {
	config *PlanWorkflowConfig
	cli    *common.CLIHelper
}

// NewPlanWorkflow 创建规划工作流
func NewPlanWorkflow(config *PlanWorkflowConfig) *PlanWorkflow {
	if config == nil {
		config = &PlanWorkflowConfig{}
	}

	if config.PlannerModel == "" {
		config.PlannerModel = "deepseek-chat"
	}

	return &PlanWorkflow{
		config: config,
		cli:    common.NewCLIHelper("规划工作流", "AI驱动的章节规划系统"),
	}
}

// getContextData 获取上下文数据
func (pw *PlanWorkflow) getContextData() map[string]any {
	var cfg = content.ContextConfig{
		NovelDir: pw.config.NovelDir,
		Logger:   pw.config.Logger,
	}

	var cb = content.NewContextBuilder(&cfg)
	const maxTokens = 128000 //128k tokens
	var percentage = token.TokenPercentages{
		Plan:      0.6,  // 1000000 * 0.6 / 1.5 = 400000 中文
		Character: 0.03, // 1000000 * 0.03 / 1.5 = 20000
		Worldview: 0.04, // 1000000 * 0.04 / 1.5 = 26666
		Index:     0.03, // 1000000 * 0.03 / 1.5 = 20000
		Chapters:  0.3,  // 1000000 * 0.3 / 1.5 = 200000
	}

	// 生成 文章Data 数据结构体
	var data, err = cb.BuildTokenAwareContext(&percentage, maxTokens)
	if err != nil {
		panic(fmt.Errorf("%w", err))
	}

	if pw.config.ShowProgress {
		pw.cli.ShowInfo("📖", "上下文加载完成")
	}

	// 使用ContextBuilder的GetContextAsMap方法确保键名正确
	return cb.GetContextAsMap(data)
}

func (pw *PlanWorkflow) CreateReActAgent() (*react.Agent, error) {
	ctx := context.Background()

	plannerModel, err := pw.config.LLMManager.GetOpenAIModel(ctx, providers.WithModel(pw.config.PlannerModel))
	if err != nil {
		if pw.config.Logger != nil {
			pw.config.Logger.Warn(fmt.Sprintf("获取规划模型 %s 失败，尝试备用模型: %v", pw.config.PlannerModel, err))
		}
		// 备用方案使用默认OpenAI模型
		plannerModel, err = pw.config.LLMManager.GetOpenAIModel(ctx)
		if err != nil {
			return nil, fmt.Errorf("所有规划模型都不可用: %w", err)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("获取 plannerModel 失败: %w", err)
	}

	// 创建工具配置
	planCrudTool := tools.NewPlanCRUDTool(pw.config.NovelDir)
	currentChapterTool := tools.NewCurrentChapterCRUDTool(pw.config.NovelDir)

	agentConfig := &react.AgentConfig{
		ToolCallingModel: plannerModel,
		ToolsConfig: compose.ToolsNodeConfig{
			Tools:               []tool.BaseTool{planCrudTool, currentChapterTool},
			ExecuteSequentially: false,
		},
		MaxStep:         10,
		MessageModifier: pw.createMessageModifier(),
	}

	return react.NewAgent(ctx, agentConfig)

}

func (pw *PlanWorkflow) createMessageModifier() react.MessageModifier {
	// 预先加载上下文数据，避免重复调用
	ctxData := pw.getContextData()
	prompt := content.Novel_planner_prompt

	return func(ctx context.Context, input []*schema.Message) []*schema.Message {
		sysPrompt := fmt.Sprintf(
			`%v,当前上下文:-章节:%v-世界观:%v-角色:%v-现有规划:%v-总结: %v`,
			prompt,
			ctxData["chapter"],
			ctxData["worldview"],
			ctxData["characters"],
			ctxData["plan"],
			ctxData["summary"],
		)
		result := make([]*schema.Message, 0, len(input)+1)
		result = append(result, schema.SystemMessage(sysPrompt))
		result = append(result, input...)
		return result
	}
}

func (pw *PlanWorkflow) ExecuteWithMonitoring(input string) (string, error) {
	agent, err := pw.CreateReActAgent()
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
		pw.config.Logger.Info(fmt.Sprintf("步骤 %d: Role=%s, Content=%s, ToolCalls=%d",
			stepCount, msg.Role, msg.Content, len(msg.ToolCalls)))
	}

	return response.Content, nil
}
