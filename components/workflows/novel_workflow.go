package workflows

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/Kizunad/modular-workflow-v2/components/agents"
	"github.com/Kizunad/modular-workflow-v2/components/common"
	"github.com/Kizunad/modular-workflow-v2/components/content"
	"github.com/Kizunad/modular-workflow-v2/components/content/managers"
	"github.com/Kizunad/modular-workflow-v2/components/content/token"
	"github.com/Kizunad/modular-workflow-v2/logger"
	"github.com/Kizunad/modular-workflow-v2/providers"
)

const (
	KeyContext = "key_context"
	KeyPlanner = "key_planner"
	KeyWriter  = "key_writer"
)

// NovelWorkflowConfig 小说工作流配置
type NovelWorkflowConfig struct {
	Logger           *logger.ZapLogger
	Generator        *content.Generator        // 完整内容生成器（供planner使用）
	LimitedGenerator *content.LimitedGenerator // 限制内容生成器（供writer使用）
	Planner          *agents.Planner           // 规划器
	WorldviewManager *managers.WorldviewManager
	CharacterManager *managers.CharacterManager
	LLMManager       *providers.Manager
	CompiledWriter   compose.Runnable[map[string]any, *schema.Message]
	CheckPointStore  compose.CheckPointStore
	ShowProgress     bool
}

// NovelWorkflow 小说续写工作流
type NovelWorkflow struct {
	config  *NovelWorkflowConfig
	builder *common.WorkflowBuilder
	cli     *common.CLIHelper
}

// NewNovelWorkflow 创建小说续写工作流
func NewNovelWorkflow(config *NovelWorkflowConfig) *NovelWorkflow {
	if config == nil {
		config = &NovelWorkflowConfig{}
	}

	if config.CheckPointStore == nil {
		config.CheckPointStore = common.NewInMemoryCheckPointStore()
	}

	builderConfig := &common.WorkflowConfig{
		Logger:          config.Logger,
		CheckPointStore: config.CheckPointStore,
		EnableProgress:  config.ShowProgress,
		ProgressPrefix:  "📋",
	}

	return &NovelWorkflow{
		config:  config,
		builder: common.NewWorkflowBuilder(builderConfig),
		cli:     common.NewCLIHelper("小说续写工作流", "AI驱动的小说续写系统"),
	}
}

// CreateWorkflow 创建完整的小说续写工作流
func (nw *NovelWorkflow) CreateWorkflow() (compose.Runnable[string, string], error) {
	f := nw.builder.CreateGenericWorkflow()

	// 添加上下文生成节点
	f.AddLambdaNode(
		KeyContext,
		nw.builder.CreateStepLambda(
			KeyContext,
			"生成小说上下文",
			func(ctx context.Context, input interface{}) (interface{}, error) {
				userPrompt := input.(string)

				// 获取动态上下文
				contextData := nw.dynamicContext()

				if nw.config.ShowProgress {
					nw.cli.ShowSuccess("动态上下文生成完成")
				}

				// 构建最终输入数据
				finalInput := map[string]any{
					"context":    contextData["context"],
					"worldview":  contextData["worldview"],
					"characters": contextData["characters"],
					"chapter":    contextData["chapter"],
					"input":      userPrompt,
				}

				return finalInput, nil
			},
		),
	).AddInput(compose.START)

	// 添加规划节点 - 分析所有章节制定策略
	f.AddLambdaNode(
		KeyPlanner,
		nw.builder.CreateStepLambda(
			KeyPlanner,
			"规划器分析并制定写作策略",
			func(ctx context.Context, input interface{}) (interface{}, error) {
				inputMap := input.(map[string]any)

				// 创建规划请求
				planRequest := &agents.PlanRequest{
					UserPrompt:  inputMap["input"].(string),
					Worldview:   fmt.Sprintf("%v", inputMap["worldview"]),
					Characters:  inputMap["characters"],
					AllChapters: inputMap["context"].(string), // 使用完整章节内容
					Context:     inputMap["context"].(string),
				}

				// 执行规划
				strategy, err := nw.config.Planner.Plan(ctx, planRequest)
				if err != nil {
					return nil, fmt.Errorf("规划失败: %w", err)
				}
				// 将规划结果保存到 planner.json，并更新最新聚合内容
				if nw.config.Generator != nil {
					pcm := managers.NewPlannerContentManager(nw.config.Generator.GetNovelDir())
					// 将本次规划写入为下一章的计划（现有章节数 + 1）
					if count, err := pcm.CountChapters(); err == nil {
						title := fmt.Sprintf("%03d", count+1)
						_ = pcm.UpsertPlan(title, strategy)
					} else if nw.config.Logger != nil {
						nw.config.Logger.Warn("统计章节数失败: " + err.Error())
					}
				}
				// 将规划结果添加到输入中
				inputMap["writing_strategy"] = strategy
				return inputMap, nil
			},
		),
	).AddInput(KeyContext)

	// 添加写作节点 - 基于规划策略创作内容
	f.AddLambdaNode(
		KeyWriter,
		nw.builder.CreateStepLambda(
			KeyWriter,
			"AI作者基于策略创作小说内容",
			func(ctx context.Context, input interface{}) (interface{}, error) {
				inputMap := input.(map[string]any)

				// 更新上下文为限制版本（只包含前两章）
				limitedContent, err := nw.getLimitedContext()
				if err != nil {
					if nw.config.Logger != nil {
						nw.config.Logger.Warn("获取限制上下文失败: " + err.Error())
					}
				} else {
					inputMap["context"] = limitedContent
				}

				result, err := nw.config.CompiledWriter.Invoke(ctx, inputMap)
				if err != nil {
					return nil, fmt.Errorf("写作失败: %w", err)
				}

				return result.Content, nil
			},
		),
	).AddInput(KeyPlanner)

	// 设置结束节点
	f.End().AddInput(KeyWriter)

	return nw.builder.CompileWorkflow(f)
}

// contextData 上下文数据结构
type contextData struct {
	title      string
	summary    string
	worldview  string
	characters string
	chapter    string
}

// getBaseContext 获取基础上下文数据（完整版本）
func (nw *NovelWorkflow) getBaseContext() *contextData {
	// 完整上下文配置：更多Token给章节内容
	return nw.getTokenAwareContext(&token.TokenPercentages{
		Index:     0.05, // 5%  - 标题摘要
		Worldview: 0.10, // 10% - 世界观
		Character: 0.10, // 10% - 角色
		Chapters:  0.70, // 70% - 章节内容（主要部分）
		Plan:      0.05, // 5%  - 规划
	}, 500000)
}

// getBaseContextLimited 获取限制版本的基础上下文数据
func (nw *NovelWorkflow) getBaseContextLimited() *contextData {
	// 限制版本配置：平衡分配，减少章节内容
	return nw.getTokenAwareContext(&token.TokenPercentages{
		Index:     0.15, // 15% - 标题摘要（增加）
		Worldview: 0.20, // 20% - 世界观（增加）
		Character: 0.20, // 20% - 角色（增加）
		Chapters:  0.35, // 35% - 章节内容（减少）
		Plan:      0.10, // 10% - 规划（增加）
	}, 128000) // 更小的Token总数
}

// getTokenAwareContext 获取Token感知的基础上下文数据（核心方法）
func (nw *NovelWorkflow) getTokenAwareContext(tokenPercentages *token.TokenPercentages, maxTokens int) *contextData {
	ctx := &contextData{}

	// 使用传入的Token百分比配置创建Token预算管理器

	tokenBudget, err := token.NewTokenBudgetManager(maxTokens, tokenPercentages)
	if err != nil {
		if nw.config.Logger != nil {
			nw.config.Logger.Error(fmt.Sprintf("创建Token预算管理器失败: %v", err))
		}
		// 回退到非Token感知模式
		panic(fmt.Errorf("%w", err)) //直接报错
	}

	allocation := tokenBudget.GetAllocatedTokens()

	// 获取标题和摘要（使用index token分配）
	if nw.config.Generator != nil {
		indexReader := managers.NewIndexReaderWithTokenBudget(nw.config.Generator.GetNovelDir(), tokenBudget)

		if indexTokens, exists := allocation["index"]; exists {
			ctx.title = indexReader.GetTitle()
			ctx.summary, _ = indexReader.GetSummaryWithTokenLimit(indexTokens)
		} else {
			ctx.title = indexReader.GetTitle()
			ctx.summary = indexReader.GetSummary()
		}
	}

	// 设置默认值
	if ctx.title == "" {
		ctx.title = "无章节标题"
	}
	if ctx.summary == "" {
		ctx.summary = "暂无章节摘要"
	}

	// 获取Token感知的世界观
	if nw.config.WorldviewManager != nil {
		worldviewPath := nw.config.WorldviewManager.GetWorldviewPath()
		if nw.config.Logger != nil {
			nw.config.Logger.Info(fmt.Sprintf("[DEBUG] WorldviewManager配置信息: 路径=%s", worldviewPath))
		}

		// 从完整路径提取目录路径
		novelDir := filepath.Dir(worldviewPath)
		if nw.config.Logger != nil {
			nw.config.Logger.Info(fmt.Sprintf("[DEBUG] 提取的小说目录: %s", novelDir))
		}

		worldviewManager := managers.NewWorldviewManagerWithTokenBudget(novelDir, tokenBudget)

		if worldviewTokens, exists := allocation["worldview"]; exists {
			if nw.config.Logger != nil {
				nw.config.Logger.Info(fmt.Sprintf("[DEBUG] 使用Token限制读取世界观: 最大Token=%d", worldviewTokens))
			}
			ctx.worldview, _ = worldviewManager.GetCurrentWithTokenLimit(worldviewTokens)
		} else {
			if nw.config.Logger != nil {
				nw.config.Logger.Info("[DEBUG] 使用完整读取世界观")
			}
			ctx.worldview = worldviewManager.GetCurrent()
		}

		if nw.config.Logger != nil {
			nw.config.Logger.Info(fmt.Sprintf("[DEBUG] 世界观读取结果: 长度=%d, 内容预览='%s'", len(ctx.worldview),
				func() string {
					if len(ctx.worldview) > 50 {
						return ctx.worldview[:50] + "..."
					}
					return ctx.worldview
				}()))
		}
	} else {
		if nw.config.Logger != nil {
			nw.config.Logger.Info("[DEBUG] WorldviewManager为空，跳过世界观加载")
		}
	}

	if ctx.worldview == "" {
		if nw.config.Logger != nil {
			nw.config.Logger.Warn("世界观文件不存在或内容为空")
		}
		ctx.worldview = "暂无世界观设定"
	}

	// 获取Token感知的角色信息
	if nw.config.CharacterManager != nil {
		characterManager := managers.NewCharacterManagerWithTokenBudget(nw.config.CharacterManager.GetCharacterPath(), tokenBudget)

		if characterTokens, exists := allocation["character"]; exists {
			ctx.characters, _ = characterManager.GetCurrentWithTokenLimit(characterTokens)
		} else {
			ctx.characters = characterManager.GetCurrent()
		}
	}

	// 获取Token感知的章节内容
	if nw.config.Generator != nil {
		if _, exists := allocation["chapters"]; exists {
			// 使用Token限制生成章节内容
			content, err := nw.config.Generator.Generate()
			if err != nil {
				if nw.config.Logger != nil {
					nw.config.Logger.Error(fmt.Sprintf("获取章节内容失败: %v", err))
				}
				ctx.chapter = ""
			} else {
				// 截断章节内容到指定Token限制
				ctx.chapter, _ = tokenBudget.TruncateToTokenLimit(content, "chapters")
			}
		}
	}

	// 记录Token使用情况
	if nw.config.Logger != nil {
		nw.config.Logger.Info(fmt.Sprintf("Token分配: index=%d, worldview=%d, character=%d, chapters=%d",
			allocation["index"], allocation["worldview"], allocation["character"], allocation["chapters"]))
	}

	return ctx
}

// formatContext 格式化上下文为文本
func (nw *NovelWorkflow) formatContext(ctx *contextData) string {
	return fmt.Sprintf("章节标题: %s\n\n章节摘要:\n%s\n\n世界观:\n%s\n\n角色信息:\n%s\n\n当前章节:\n%s",
		ctx.title, ctx.summary, ctx.worldview, ctx.characters, ctx.chapter)
}

// dynamicContext 获取动态上下文信息，包括世界观、角色和章节内容（完整版本）
func (nw *NovelWorkflow) dynamicContext() map[string]any {
	ctx := nw.getBaseContext() // 使用完整版本配置

	return map[string]any{
		"title":      ctx.title,
		"summary":    ctx.summary,
		"worldview":  ctx.worldview,
		"characters": ctx.characters,
		"chapter":    ctx.chapter,
		"context":    nw.formatContext(ctx),
	}
}

// getLimitedContext 获取限制上下文（限制版本：更平衡的Token分配）
func (nw *NovelWorkflow) getLimitedContext() (string, error) {
	// 使用限制版本的Token配置
	ctx := nw.getBaseContextLimited()

	return nw.formatContext(ctx), nil
}
