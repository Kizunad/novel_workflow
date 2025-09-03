package agents

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/Kizunad/modular-workflow-v2/components/agents/tools"
	"github.com/Kizunad/modular-workflow-v2/components/content"
	"github.com/Kizunad/modular-workflow-v2/components/content/managers"
	"github.com/Kizunad/modular-workflow-v2/logger"
	"github.com/Kizunad/modular-workflow-v2/providers"
)

// PlannerConfig Planner配置
type PlannerConfig struct {
	LLMManager       *providers.Manager
	ContentGenerator *content.Generator
	Logger           *logger.ZapLogger
	ShowProgress     bool
	PlannerModel     string // 指定planner使用的模型
}

// Planner 规划器，负责读取所有章节并制定写作策略
type Planner struct {
	config *PlannerConfig
}

// NewPlanner 创建规划器
func NewPlanner(config *PlannerConfig) *Planner {
	if config.PlannerModel == "" {
		config.PlannerModel = "gemini-2.5-flash"
	}
	return &Planner{config: config}
}

// PlanRequest 规划请求
type PlanRequest struct {
	UserPrompt  string      `json:"user_prompt"`
	Worldview   string      `json:"worldview"`
	Characters  interface{} `json:"characters"`
	AllChapters string      `json:"all_chapters"` // 所有章节内容
	Context     string      `json:"context"`      // 完整上下文
}

// PlanResult 规划结果
type PlanResult struct {
	WritingStrategy   string `json:"writing_strategy"`   // 写作策略
	PlotDirection     string `json:"plot_direction"`     // 情节方向
	CharacterGuidance string `json:"character_guidance"` // 角色指导
	StyleGuidance     string `json:"style_guidance"`     // 文风指导
}

// CreatePlanningGraph 创建规划Graph
func (p *Planner) CreatePlanningGraph(ctx context.Context) (*compose.Graph[*PlanRequest, *schema.Message], error) {
	// 创建Graph
	graph := compose.NewGraph[*PlanRequest, *schema.Message]()

	// 获取规划模型 (使用gemini-2.5-flash)
	plannerModel, err := p.getPlannerModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取规划模型失败: %w", err)
	}

	// 创建规划模板
	planTemplate := p.createPlanTemplate()

	// 创建输入转换器
	inputConverter := p.createInputConverter()

	// 创建工具节点 - 只包含NovelReadChapterTool
	toolsNode, err := p.createToolsNode(ctx)
	if err != nil {
		return nil, fmt.Errorf("创建工具节点失败: %w", err)
	}

	// 创建消息合并器
	messageMerger := p.createMessageMerger()

	// 添加节点
	if err := graph.AddLambdaNode("input_converter", inputConverter); err != nil {
		return nil, fmt.Errorf("添加输入转换节点失败: %w", err)
	}

	if err := graph.AddChatTemplateNode("plan_template", planTemplate); err != nil {
		return nil, fmt.Errorf("添加规划模板节点失败: %w", err)
	}

	if err := graph.AddChatModelNode("planner_model", plannerModel); err != nil {
		return nil, fmt.Errorf("添加规划模型节点失败: %w", err)
	}

	if err := graph.AddToolsNode("tools", toolsNode); err != nil {
		return nil, fmt.Errorf("添加工具节点失败: %w", err)
	}

	if err := graph.AddLambdaNode("message_merger", messageMerger); err != nil {
		return nil, fmt.Errorf("添加消息合并节点失败: %w", err)
	}

	// 连接节点
	if err := graph.AddEdge(compose.START, "input_converter"); err != nil {
		return nil, fmt.Errorf("连接 START -> input_converter 失败: %w", err)
	}
	if err := graph.AddEdge("input_converter", "plan_template"); err != nil {
		return nil, fmt.Errorf("连接 input_converter -> plan_template 失败: %w", err)
	}
	if err := graph.AddEdge("plan_template", "planner_model"); err != nil {
		return nil, fmt.Errorf("连接 plan_template -> planner_model 失败: %w", err)
	}

	// 添加分支：根据planner_model的输出决定是否调用工具
	branchCondition := func(ctx context.Context, message *schema.Message) (string, error) {
		// 检查消息是否包含工具调用
		if len(message.ToolCalls) > 0 {
			return "tools", nil
		}
		// 没有工具调用，直接结束
		return compose.END, nil
	}

	// 使用NewGraphBranch正确创建分支
	branch := compose.NewGraphBranch(branchCondition, map[string]bool{
		"tools":     true,
		compose.END: true,
	})
	if err := graph.AddBranch("planner_model", branch); err != nil {
		return nil, fmt.Errorf("添加分支条件失败: %w", err)
	}

	// 工具调用路径：tools -> message_merger -> planner_model (形成循环)
	if err := graph.AddEdge("tools", "message_merger"); err != nil {
		return nil, fmt.Errorf("连接 tools -> message_merger 失败: %w", err)
	}
	if err := graph.AddEdge("message_merger", "planner_model"); err != nil {
		return nil, fmt.Errorf("连接 message_merger -> planner_model 失败: %w", err)
	}

	return graph, nil
}

// getPlannerModel 获取规划模型 (固定使用gemini-2.5-flash)
func (p *Planner) getPlannerModel(ctx context.Context) (model.ToolCallingChatModel, error) {
	plannerModel, err := p.config.LLMManager.GetOpenAIModel(ctx, providers.WithModel(p.config.PlannerModel))
	if err != nil {
		if p.config.Logger != nil {
			p.config.Logger.Warn(fmt.Sprintf("获取规划模型 %s 失败，尝试备用模型: %v", p.config.PlannerModel, err))
		}
		// 备用方案使用默认OpenAI模型
		plannerModel, err = p.config.LLMManager.GetOpenAIModel(ctx)
		if err != nil {
			return nil, fmt.Errorf("所有规划模型都不可用: %w", err)
		}
	}
	return plannerModel, nil
}

// createInputConverter 创建输入转换器
func (p *Planner) createInputConverter() *compose.Lambda {
	return compose.InvokableLambda(func(ctx context.Context, input *PlanRequest) (map[string]any, error) {
		if p.config.ShowProgress {
			p.config.Logger.Info("🧠 规划器开始分析所有章节...")
		}

		// 获取所有章节内容（如果没有提供的话）
		allChapters := input.AllChapters
		if allChapters == "" && p.config.ContentGenerator != nil {
			generatedContent, err := p.config.ContentGenerator.Generate()
			if err == nil {
				allChapters = generatedContent
			}
		}

		// 分析内容长度
		contentLength := len(allChapters)

		// 使用 PlannerContentManager 记录最新内容并读取最近规划
		var recentPlansFormatted string
		var chaptersCount int
		if p.config.ContentGenerator != nil {
			pcm := managers.NewPlannerContentManager(p.config.ContentGenerator.GetNovelDir())
			// 保存最新聚合内容（即 content 生成的字符串）
			_ = pcm.UpdateLatestContent(allChapters)

			// 获取章节数量
			chaptersCount, _ = pcm.CountChapters()

			// 获取所有计划并取最近的5条
			allPlans := pcm.GetAllPlans()
			var recents []managers.PlanEntry
			if len(allPlans) > 5 {
				recents = allPlans[len(allPlans)-5:]
			} else {
				recents = allPlans
			}
			var b strings.Builder
			for _, r := range recents {
				plan := strings.TrimSpace(r.Plan)
				if plan == "" {
					plan = "（暂无规划）"
				}
				b.WriteString("- ")
				b.WriteString(r.Title)
				b.WriteString(": ")
				b.WriteString(plan)
				b.WriteString("\n")
			}
			recentPlansFormatted = strings.TrimSpace(b.String())
		}

		if p.config.ShowProgress {
			p.config.Logger.Info(fmt.Sprintf("📊 已分析内容: 总长度%d字符", contentLength))
		}

		return map[string]any{
			"user_prompt":    input.UserPrompt,
			"worldview":      input.Worldview,
			"characters":     input.Characters,
			"all_chapters":   allChapters,
			"context":        input.Context,
			"content_length": contentLength,
			"recent_plans":   recentPlansFormatted,
			"chapters_count": chaptersCount,
		}, nil
	})
}

// createToolsNode 创建工具节点 - 只包含NovelReadChapterTool
func (p *Planner) createToolsNode(ctx context.Context) (*compose.ToolsNode, error) {
	// 只包含章节读取工具
	readChapterTool := tools.NewNovelReadChapterTool()
	planningTools := []tool.BaseTool{readChapterTool}

	// 创建工具节点配置
	toolsNodeConfig := &compose.ToolsNodeConfig{
		Tools:               planningTools,
		ExecuteSequentially: false, // 并行执行工具调用
	}

	return compose.NewToolNode(ctx, toolsNodeConfig)
}

// createMessageMerger 创建消息合并器
func (p *Planner) createMessageMerger() *compose.Lambda {
	return compose.InvokableLambda(func(ctx context.Context, toolResults []*schema.Message) ([]*schema.Message, error) {
		// 获取原始对话历史（从第一次planner_model调用的输入）
		if len(toolResults) == 0 {
			return []*schema.Message{}, nil
		}

		// 简单返回工具结果，让planner_model基于工具结果继续规划
		return toolResults, nil
	})
}

// createPlanTemplate 创建规划模板
func (p *Planner) createPlanTemplate() prompt.ChatTemplate {
	return prompt.FromMessages(
		schema.FString,
		schema.SystemMessage(string(content.Novel_planner_prompt)+`

你可以使用以下工具来获取更多信息：

**novel_read_chapter**: 根据章节ID读取指定章节的完整内容
- 参数: chapter_id (章节ID，例如：001、002或example_chapter_1.json)
- 用法: 当需要查看特定章节内容进行分析时使用

如需查看特定章节内容来制定更精准的规划，请调用相应工具。`),
		schema.UserMessage(`## 分析材料

**世界观设定：**
{worldview}

**角色信息：**
{characters}

**已有章节内容 (共{content_length}字符)：**
{all_chapters}

**用户写作需求：**
{user_prompt}

**历史规划（最近5条，章节总数：{chapters_count}）：**
{recent_plans}

## 请提供写作规划

请分析以上材料，制定详细的写作策略，包括：

1. **情节发展策略** - 下一章节应该如何推进剧情
2. **角色塑造方向** - 主要角色的发展方向和互动重点
3. **冲突设置建议** - 应该引入什么样的冲突或转折
4. **文风和节奏控制** - 建议的写作风格和叙事节奏
5. **世界观运用** - 如何更好地运用已设定的世界观元素
6. **具体写作指导** - 给writer的详细指导意见

 请基于历史规划保持前后一致与远瞻性，提供专业、详细、可操作的规划建议：`),
	)
}

// Plan 执行规划
func (p *Planner) Plan(ctx context.Context, request *PlanRequest) (string, error) {
	// 创建规划Graph
	graph, err := p.CreatePlanningGraph(ctx)
	if err != nil {
		return "", fmt.Errorf("创建规划图失败: %w", err)
	}

	// 编译Graph
	runnable, err := graph.Compile(ctx)
	if err != nil {
		return "", fmt.Errorf("编译规划图失败: %w", err)
	}

	// 执行规划
	result, err := runnable.Invoke(ctx, request)
	if err != nil {
		return "", fmt.Errorf("执行规划失败: %w", err)
	}

	if p.config.ShowProgress {
		p.config.Logger.Info("✅ 规划完成，策略制定成功")
	}

	// 将规划结果写入 planner.json，标题为下一个章节编号（现有章节数+1）
	if p.config.ContentGenerator != nil {
		pcm := managers.NewPlannerContentManager(p.config.ContentGenerator.GetNovelDir())
		if count, err := pcm.CountChapters(); err == nil {
			title := fmt.Sprintf("%03d", count+1)
			_ = pcm.UpsertPlan(title, result.Content)
		}
	}

	return result.Content, nil
}