package content

import (
	"fmt"
	"path/filepath"

	"github.com/Kizunad/modular-workflow-v2/components/content/managers"
	"github.com/Kizunad/modular-workflow-v2/components/content/token"
	"github.com/Kizunad/modular-workflow-v2/logger"
)

// ContextData 上下文数据结构
type ContextData struct {
	Title      string `json:"title"`
	Summary    string `json:"summary"`
	Worldview  string `json:"worldview"`
	Characters string `json:"characters"`
	Chapter    string `json:"chapter"`
	Plan       string `json:"plan"` // 规划信息
}

// ContextConfig 上下文构建配置
type ContextConfig struct {
	NovelDir string            // 小说目录（所有内容的根目录）
	Logger   *logger.ZapLogger // 日志记录器
}

// ContextBuilder Token感知的上下文构建器
type ContextBuilder struct {
	config *ContextConfig
}

// NewContextBuilder 创建上下文构建器
func NewContextBuilder(config *ContextConfig) *ContextBuilder {
	return &ContextBuilder{
		config: config,
	}
}

// BuildTokenAwareContext 构建Token感知的上下文数据
// tokenPercentages: Token分配百分比配置
// maxTokens: 最大Token数量
func (cb *ContextBuilder) BuildTokenAwareContext(tokenPercentages *token.TokenPercentages, maxTokens int) (*ContextData, error) {
	ctx := &ContextData{}

	// 创建Token预算管理器
	tokenBudget, err := token.NewTokenBudgetManager(maxTokens, tokenPercentages)
	if err != nil {
		if cb.config.Logger != nil {
			cb.config.Logger.Error(fmt.Sprintf("创建Token预算管理器失败: %v", err))
		}
		return nil, fmt.Errorf("创建Token预算管理器失败: %w", err)
	}

	allocation := tokenBudget.GetAllocatedTokens()

	// 获取标题和摘要（使用index token分配）
	if cb.config.NovelDir != "" {
		indexReader := managers.NewIndexReaderWithTokenBudget(cb.config.NovelDir, tokenBudget)

		if indexTokens, exists := allocation["index"]; exists {
			ctx.Title = indexReader.GetTitle()
			ctx.Summary, _ = indexReader.GetSummaryWithTokenLimit(indexTokens)
		} else {
			ctx.Title = indexReader.GetTitle()
			ctx.Summary = indexReader.GetSummary()
		}
	}

	// 设置默认值
	if ctx.Title == "" {
		ctx.Title = "无章节标题"
	}
	if ctx.Summary == "" {
		ctx.Summary = "暂无章节摘要"
	}

	// 获取Token感知的世界观（使用标准路径）
	worldviewManager := managers.NewWorldviewManagerWithTokenBudget(cb.config.NovelDir, tokenBudget)

	if worldviewTokens, exists := allocation["worldview"]; exists {
		if cb.config.Logger != nil {
			cb.config.Logger.Info(fmt.Sprintf("[DEBUG] 使用Token限制读取世界观: 最大Token=%d", worldviewTokens))
		}
		ctx.Worldview, _ = worldviewManager.GetCurrentWithTokenLimit(worldviewTokens)
	} else {
		if cb.config.Logger != nil {
			cb.config.Logger.Info("[DEBUG] 使用完整读取世界观")
		}
		ctx.Worldview = worldviewManager.GetCurrent()
	}

	if cb.config.Logger != nil {
		cb.config.Logger.Info(fmt.Sprintf("[DEBUG] 世界观读取结果: 长度=%d, 内容预览='%s'", len(ctx.Worldview),
			func() string {
				if len(ctx.Worldview) > 50 {
					return ctx.Worldview[:50] + "..."
				}
				return ctx.Worldview
			}()))
	}

	if ctx.Worldview == "" {
		if cb.config.Logger != nil {
			cb.config.Logger.Warn("世界观文件不存在或内容为空")
		}
		ctx.Worldview = "暂无世界观设定"
	}

	// 获取Token感知的角色信息（使用标准路径）
	characterPath := filepath.Join(cb.config.NovelDir, "character.md")
	characterManager := managers.NewCharacterManagerWithTokenBudget(characterPath, tokenBudget)

	if characterTokens, exists := allocation["character"]; exists {
		ctx.Characters, _ = characterManager.GetCurrentWithTokenLimit(characterTokens)
	} else {
		ctx.Characters = characterManager.GetCurrent()
	}

	// 获取Token感知的章节内容（使用标准的章节管理器）
	chapterManager := managers.NewChapterManager(cb.config.NovelDir)

	if _, exists := allocation["chapters"]; exists {
		// 获取最新章节内容并限制Token数量
		if content, err := chapterManager.GetLatestChapterContent(); err == nil {
			ctx.Chapter, _ = tokenBudget.TruncateToTokenLimit(content, "chapters")
		} else {
			ctx.Chapter = ""
		}
	} else {
		// 不限制Token数量时，获取最新章节
		if content, err := chapterManager.GetLatestChapterContent(); err == nil {
			ctx.Chapter = content
		} else {
			ctx.Chapter = ""
		}
	}

	// 获取Token感知的规划信息（使用标准的规划管理器）
	plannerManager := managers.NewPlannerContentManagerWithTokenBudget(cb.config.NovelDir, tokenBudget)

	if planTokens, exists := allocation["plan"]; exists {
		// 获取规划信息并限制Token数量
		ctx.Plan, _ = plannerManager.GetPlansWithTokenLimit(planTokens)
	} else {
		// 不限制Token数量时，获取所有规划
		ctx.Plan = plannerManager.FormatPlansForContext()
	}

	// 记录Token使用情况
	if cb.config.Logger != nil {
		cb.config.Logger.Info(fmt.Sprintf("Token分配: index=%d, worldview=%d, character=%d, chapters=%d, plan=%d",
			allocation["index"], allocation["worldview"], allocation["character"], allocation["chapters"], allocation["plan"]))
	}

	return ctx, nil
}

// FormatContext 将上下文数据格式化为文本
func (cb *ContextBuilder) FormatContext(ctx *ContextData) string {
	return fmt.Sprintf("章节标题: %s\n\n章节摘要:\n%s\n\n世界观:\n%s\n\n角色信息:\n%s\n\n当前章节:\n%s\n\n规划信息:\n%s",
		ctx.Title, ctx.Summary, ctx.Worldview, ctx.Characters, ctx.Chapter, ctx.Plan)
}

// GetContextAsMap 将上下文数据转换为map格式
func (cb *ContextBuilder) GetContextAsMap(ctx *ContextData) map[string]any {
	return map[string]any{
		"title":      ctx.Title,
		"summary":    ctx.Summary,
		"worldview":  ctx.Worldview,
		"characters": ctx.Characters,
		"chapter":    ctx.Chapter,
		"plan":       ctx.Plan,
		"context":    cb.FormatContext(ctx),
	}
}

// BuildFullContext 构建完整上下文（不使用Token限制）
func (cb *ContextBuilder) BuildFullContext() (*ContextData, error) {
	ctx := &ContextData{}

	// 获取标题和摘要
	if cb.config.NovelDir != "" {
		indexReader := managers.NewIndexReader(cb.config.NovelDir)
		ctx.Title = indexReader.GetTitle()
		ctx.Summary = indexReader.GetSummary()
	}

	// 设置默认值
	if ctx.Title == "" {
		ctx.Title = "无章节标题"
	}
	if ctx.Summary == "" {
		ctx.Summary = "暂无章节摘要"
	}

	// 获取世界观（使用标准路径）
	worldviewManager := managers.NewWorldviewManager(cb.config.NovelDir)
	ctx.Worldview = worldviewManager.GetCurrent()

	if ctx.Worldview == "" {
		if cb.config.Logger != nil {
			cb.config.Logger.Warn("世界观文件不存在或内容为空")
		}
		ctx.Worldview = "暂无世界观设定"
	}

	// 获取角色信息（使用标准路径）
	characterPath := filepath.Join(cb.config.NovelDir, "character.md")
	characterManager := managers.NewCharacterManager(characterPath)
	ctx.Characters = characterManager.GetCurrent()

	// 获取章节内容（使用标准的章节管理器）
	chapterManager := managers.NewChapterManager(cb.config.NovelDir)
	if content, err := chapterManager.GetLatestChapterContent(); err == nil {
		ctx.Chapter = content
	} else {
		ctx.Chapter = ""
	}

	// 获取规划信息（使用标准的规划管理器）
	plannerManager := managers.NewPlannerContentManager(cb.config.NovelDir)
	ctx.Plan = plannerManager.FormatPlansForContext()

	return ctx, nil
}
