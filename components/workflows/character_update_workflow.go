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

// CharacterUpdateWorkflowConfig 角色更新工作流配置
type CharacterUpdateWorkflowConfig struct {
	Logger       *logger.ZapLogger
	NovelDir     string // 小说目录路径
	LLMManager   *providers.Manager
	ShowProgress bool
	Model        string // 模型名称
}

// CharacterUpdateWorkflow 角色更新工作流
type CharacterUpdateWorkflow struct {
	config *CharacterUpdateWorkflowConfig
	cli    *common.CLIHelper
}

// NewCharacterUpdateWorkflow 创建角色更新工作流
func NewCharacterUpdateWorkflow(config *CharacterUpdateWorkflowConfig) *CharacterUpdateWorkflow {
	if config == nil {
		config = &CharacterUpdateWorkflowConfig{}
	}

	if config.Model == "" {
		config.Model = "qwen3:4b"
	}

	return &CharacterUpdateWorkflow{
		config: config,
		cli:    common.NewCLIHelper("角色更新工作流", "AI驱动的角色状态更新系统"),
	}
}

// CreateReActAgent 创建角色更新 ReAct Agent
func (cw *CharacterUpdateWorkflow) CreateReActAgent() (*react.Agent, error) {
	ctx := context.Background()

	// 获取模型
	model, err := cw.config.LLMManager.GetOllamaModel(ctx, providers.WithModel(cw.config.Model))
	if err != nil {
		if cw.config.Logger != nil {
			cw.config.Logger.Warn(fmt.Sprintf("获取模型 %s 失败，尝试备用模型: %v", cw.config.Model, err))
		}
		// 备用方案使用默认模型
		model, err = cw.config.LLMManager.GetOllamaModel(ctx)
		if err != nil {
			return nil, fmt.Errorf("所有模型都不可用: %w", err)
		}
	}

	// 创建角色管理工具
	characterTool := tools.NewCharacterCRUDTool(cw.config.NovelDir, cw.config.LLMManager)
	chapterTool := tools.NewCurrentChapterCRUDTool(cw.config.NovelDir)
	chapterAnalysisTool := tools.NewChapterAnalysisTool(cw.config.NovelDir)
	planTool := tools.NewPlanCRUDTool(cw.config.NovelDir)

	// 创建工具节点配置
	toolsNodeConfig := &compose.ToolsNodeConfig{
		Tools: []tool.BaseTool{characterTool, chapterTool, chapterAnalysisTool, planTool},
		ExecuteSequentially: false,
	}

	// 创建 ReAct Agent 配置
	agentConfig := &react.AgentConfig{
		ToolCallingModel: model,
		ToolsConfig:      *toolsNodeConfig,
		MaxStep:          15, // 角色更新可能需要多步分析
		MessageModifier:  cw.createMessageModifier(),
	}

	// 创建 ReAct Agent
	return react.NewAgent(ctx, agentConfig)
}

// createMessageModifier 创建消息修饰器
func (cw *CharacterUpdateWorkflow) createMessageModifier() react.MessageModifier {
	// 预先加载上下文数据
	ctxData := cw.getContextData()
	// 使用默认角色更新提示词
	prompt := `你是一个专业的小说角色状态管理专家。你的任务是：

1. 分析最新章节内容对角色状态的影响
2. 识别角色的重要变化（能力、装备、位置、关系等）
3. 根据分析结果决定是否更新角色信息
4. 如果需要更新，生成准确的更新内容

工作流程：
1. 首先获取最新章节内容
2. 读取当前角色信息
3. 使用AI分析角色变化
4. 如果需要更新，执行更新操作

请严格按照分析结果决定是否更新，避免不必要的修改。`

	return func(ctx context.Context, input []*schema.Message) []*schema.Message {
		sysPrompt := fmt.Sprintf(
			`%v

当前上下文信息:
- 章节信息: %v
- 角色信息: %v
- 世界观: %v
- 总结: %v

请根据用户要求分析角色变化并进行相应操作。`,
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
func (cw *CharacterUpdateWorkflow) getContextData() map[string]any {
	var cfg = content.ContextConfig{
		NovelDir: cw.config.NovelDir,
		Logger:   cw.config.Logger,
	}

	var cb = content.NewContextBuilder(&cfg)
	const maxTokens = 64000 // 适合角色更新任务的token限制
	var percentage = token.TokenPercentages{
		Plan:      0.10, // 减少规划信息
		Character: 0.40, // 重点关注角色信息
		Worldview: 0.10,
		Chapters:  0.30, // 适量章节上下文
		Index:     0.10,
	}

	// 生成上下文数据结构体
	var data, err = cb.BuildTokenAwareContext(&percentage, maxTokens)
	if err != nil {
		if cw.config.Logger != nil {
			cw.config.Logger.Error("构建上下文失败", zap.Error(err))
		}
		// 返回空上下文，但不中断流程
		return map[string]any{
			"chapter":    "",
			"characters": "",
			"worldview":  "",
			"summary":    "",
		}
	}

	if cw.config.ShowProgress {
		cw.cli.ShowInfo("📖", "上下文加载完成")
	}

	// 使用ContextBuilder的GetContextAsMap方法
	return cb.GetContextAsMap(data)
}

// ExecuteWithMonitoring 执行角色更新工作流并提供监控
func (cw *CharacterUpdateWorkflow) ExecuteWithMonitoring(input string) (string, error) {
	agent, err := cw.CreateReActAgent()
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
	if cw.config.ShowProgress {
		cw.showExecutionSteps(future)
	}

	return response.Content, nil
}

// showExecutionSteps 显示执行步骤
func (cw *CharacterUpdateWorkflow) showExecutionSteps(future react.MessageFuture) {
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
		if cw.config.Logger != nil {
			cw.config.Logger.Info(fmt.Sprintf("步骤 %d: Role=%s, Content=%s, ToolCalls=%d",
				stepCount, msg.Role, truncateContent(msg.Content, 100), len(msg.ToolCalls)))
		}

		// 显示工具调用信息
		if len(msg.ToolCalls) > 0 {
			for _, toolCall := range msg.ToolCalls {
				if cw.config.Logger != nil {
					cw.config.Logger.Debug(fmt.Sprintf("工具调用: %s", toolCall.Function.Name))
				}
			}
		}
	}
}

// ProcessCharacterUpdate 处理角色更新任务（兼容原有接口）
func (cw *CharacterUpdateWorkflow) ProcessCharacterUpdate(ctx context.Context, characterName, updateContent string) error {
	var input string
	if updateContent != "" {
		// 直接更新模式
		input = fmt.Sprintf("请直接更新角色 %s 的信息，更新内容：%s", characterName, updateContent)
	} else {
		// AI分析模式
		input = fmt.Sprintf("请分析最新章节内容对角色 %s 的影响，并根据需要更新角色状态", characterName)
	}

	_, err := cw.ExecuteWithMonitoring(input)
	return err
}

