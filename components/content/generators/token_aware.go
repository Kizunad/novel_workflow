package generators

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Kizunad/modular-workflow-v2/components/content/context"
	"github.com/Kizunad/modular-workflow-v2/components/content/managers"
	"github.com/Kizunad/modular-workflow-v2/components/content/token"
	content "github.com/Kizunad/modular-workflow-v2/components/content/utils"
)

// TokenAwareGenerator Token感知的内容生成器
type TokenAwareGenerator struct {
	// 配置
	config *GeneratorConfig

	// Token管理
	tokenBudget *token.TokenBudgetManager

	// 文件管理器
	worldviewManager *managers.WorldviewManager
	characterManager *managers.CharacterManager
	indexReader      *managers.IndexReader
	plannerManager   *managers.PlannerContentManager
	chapterManager   *managers.ChapterManager


	// 指标
	metrics       *GeneratorMetrics
	lastGenerated time.Time
	
	// 缓存最后生成的内容
	lastContent   string
	lastTokenCount int

	// 同步
	mutex sync.RWMutex
}


// NewTokenAwareGenerator 创建Token感知生成器
func NewTokenAwareGenerator(config *GeneratorConfig) (*TokenAwareGenerator, error) {
	if config == nil {
		config = DefaultGeneratorConfig("")
	}

	if config.NovelDir == "" {
		return nil, content.NewInvalidConfigError("novel directory cannot be empty", nil)
	}

	// 创建Token预算管理器
	tokenBudget, err := token.NewTokenBudgetManager(config.MaxTokens, config.TokenPercentages)
	if err != nil {
		return nil, fmt.Errorf("failed to create token budget manager: %w", err)
	}

	generator := &TokenAwareGenerator{
		config:      config,
		tokenBudget: tokenBudget,
		metrics:     NewGeneratorMetrics(),
	}

	// 初始化文件管理器
	generator.initializeManagers()

	return generator, nil
}

// initializeManagers 初始化文件管理器
func (tag *TokenAwareGenerator) initializeManagers() {
	tag.worldviewManager = managers.NewWorldviewManagerWithTokenBudget(
		tag.config.NovelDir, tag.tokenBudget)
	tag.characterManager = managers.NewCharacterManagerWithTokenBudget(
		tag.config.NovelDir, tag.tokenBudget)
	tag.indexReader = managers.NewIndexReaderWithTokenBudget(
		tag.config.NovelDir, tag.tokenBudget)
	tag.plannerManager = managers.NewPlannerContentManagerWithTokenBudget(
		tag.config.NovelDir, tag.tokenBudget)
	tag.chapterManager = managers.NewChapterManager(tag.config.NovelDir)
}

// Generate 生成完整内容
func (tag *TokenAwareGenerator) Generate() (string, error) {
	content, _, err := tag.GenerateWithTokenLimit(tag.config.MaxTokens)
	return content, err
}

// GenerateWithTokenLimit 生成限制Token的内容
func (tag *TokenAwareGenerator) GenerateWithTokenLimit(maxTokens int) (string, int, error) {
	startTime := time.Now()

	// 生成内容
	content, tokenCount, err := tag.generateContent(maxTokens)
	if err != nil {
		tag.metrics.RecordError("generation", err.Error())
		return "", 0, err
	}

	// 更新指标和缓存
	duration := time.Since(startTime)
	tag.metrics.RecordGeneration(duration, tokenCount)
	tag.lastGenerated = time.Now()
	
	// 缓存生成的内容
	tag.lastContent = content
	tag.lastTokenCount = tokenCount

	return content, tokenCount, nil
}

// generateContent 生成内容的核心逻辑
func (tag *TokenAwareGenerator) generateContent(maxTokens int) (string, int, error) {
	tag.mutex.Lock()
	defer tag.mutex.Unlock()

	// 获取Token分配
	allocation := tag.tokenBudget.GetAllocatedTokens()

	// 获取各组件内容
	components := make(map[string]string)

	// 世界观
	if worldviewTokens, exists := allocation["worldview"]; exists {
		worldview, _ := tag.worldviewManager.GetCurrentWithTokenLimit(worldviewTokens)
		components["worldview"] = worldview
	}

	// 角色信息
	if characterTokens, exists := allocation["character"]; exists {
		characters, _ := tag.characterManager.GetCurrentWithTokenLimit(characterTokens)
		components["characters"] = characters
	}

	// 章节内容
	if chapterTokens, exists := allocation["chapters"]; exists {
		chapters, err := tag.generateChapterContent(chapterTokens)
		if err == nil {
			components["chapters"] = chapters
		}
	}

	// 规划内容
	if planTokens, exists := allocation["plan"]; exists {
		plans, _ := tag.plannerManager.GetPlansWithTokenLimit(planTokens)
		components["plans"] = plans
	}

	// 索引摘要
	if indexTokens, exists := allocation["index"]; exists {
		index, _ := tag.indexReader.GetSummaryWithTokenLimit(indexTokens)
		components["index"] = index
	}

	// 聚合内容
	aggregated := tag.aggregateComponents(components)

	// 确保不超过Token限制
	finalContent, finalTokens := tag.tokenBudget.TruncateToTokenLimit(aggregated, "default")

	return finalContent, finalTokens, nil
}

// generateChapterContent 生成章节内容（简化实现）
func (tag *TokenAwareGenerator) generateChapterContent(maxTokens int) (string, error) {
	// 这里使用简化实现，后续可以集成更复杂的章节读取逻辑
	chapterCount := tag.chapterManager.GetChapterCount()
	if chapterCount == 0 {
		return "暂无章节内容", nil
	}

	// 简单返回章节数量信息
	content := fmt.Sprintf("当前共有 %d 个章节", chapterCount)

	// Token截断
	truncated, tokenCount := tag.tokenBudget.TruncateToTokenLimit(content, "chapters")
	_ = tokenCount

	return truncated, nil
}

// aggregateComponents 聚合组件内容
func (tag *TokenAwareGenerator) aggregateComponents(components map[string]string) string {
	var parts []string

	// 按优先级排序
	priorities := []string{"worldview", "characters", "plans", "chapters", "index"}

	for _, component := range priorities {
		if content, exists := components[component]; exists && content != "" {
			parts = append(parts, tag.formatComponent(component, content))
		}
	}

	return strings.Join(parts, "\n\n")
}

// formatComponent 格式化组件内容
func (tag *TokenAwareGenerator) formatComponent(componentType, content string) string {
	switch componentType {
	case "worldview":
		return "=== 世界观设定 ===\n" + content
	case "characters":
		return "=== 角色信息 ===\n" + content
	case "plans":
		return "=== 章节规划 ===\n" + content
	case "chapters":
		return "=== 章节内容 ===\n" + content
	case "index":
		return "=== 章节摘要 ===\n" + content
	default:
		return content
	}
}

// GenerateContext 生成小说上下文
func (tag *TokenAwareGenerator) GenerateContext() (interface{}, error) {
	allocation := tag.tokenBudget.GetAllocatedTokens()
	return tag.GenerateContextWithBudget(allocation)
}

// GenerateContextWithBudget 基于Token预算生成上下文
func (tag *TokenAwareGenerator) GenerateContextWithBudget(budget map[string]int) (interface{}, error) {
	ctx := context.NewNovelContext()

	// 设置基础信息
	ctx.Title = tag.indexReader.GetTitle()
	if ctx.Title == "" {
		ctx.Title = "未知小说"
	}

	// 获取各组件内容（基于Token预算）
	if tokens, exists := budget["worldview"]; exists {
		worldview, tokenCount := tag.worldviewManager.GetCurrentWithTokenLimit(tokens)
		ctx.Worldview = worldview
		ctx.SetTokenCount("worldview", tokenCount)
	}

	if tokens, exists := budget["character"]; exists {
		characters, tokenCount := tag.characterManager.GetCurrentWithTokenLimit(tokens)
		ctx.Characters = characters
		ctx.SetTokenCount("characters", tokenCount)
	}

	if tokens, exists := budget["chapters"]; exists {
		chapters, err := tag.generateChapterContent(tokens)
		if err == nil {
			ctx.Chapters = chapters
			ctx.SetTokenCount("chapters", tag.tokenBudget.CountTokens(chapters))
		}
	}

	if tokens, exists := budget["plan"]; exists {
		plans, tokenCount := tag.plannerManager.GetPlansWithTokenLimit(tokens)
		ctx.Plan = plans
		ctx.SetTokenCount("plan", tokenCount)
	}

	if tokens, exists := budget["index"]; exists {
		summary, tokenCount := tag.indexReader.GetSummaryWithTokenLimit(tokens)
		ctx.Summary = summary
		ctx.Index = summary
		ctx.SetTokenCount("index", tokenCount)
	}

	ctx.UpdateTimestamp()

	return ctx, nil
}


// 实现ContentGenerator接口

// GetContentType 获取内容类型
func (tag *TokenAwareGenerator) GetContentType() string {
	return "token_aware_aggregated"
}

// GetLastGenerated 获取最后生成时间
func (tag *TokenAwareGenerator) GetLastGenerated() time.Time {
	tag.mutex.RLock()
	defer tag.mutex.RUnlock()
	return tag.lastGenerated
}

// IsContentReady 检查内容是否就绪
func (tag *TokenAwareGenerator) IsContentReady() bool {
	return tag.worldviewManager != nil && tag.characterManager != nil
}

// GetTokenCount 获取最后生成内容的Token数
func (tag *TokenAwareGenerator) GetTokenCount() int {
	tag.mutex.RLock()
	defer tag.mutex.RUnlock()
	
	// 如果有缓存的内容，直接返回缓存的token数
	if tag.lastContent != "" {
		return tag.lastTokenCount
	}
	
	// 否则生成新内容并返回token数
	tag.mutex.RUnlock()
	if _, err := tag.Generate(); err == nil {
		tag.mutex.RLock()
		return tag.lastTokenCount
	}
	tag.mutex.RLock()
	return 0
}

// EstimateTokens 估算Token数
func (tag *TokenAwareGenerator) EstimateTokens() int {
	return tag.tokenBudget.EstimateTokens("") // 简化实现
}

// SetTokenBudget 设置Token预算
func (tag *TokenAwareGenerator) SetTokenBudget(budget *token.TokenBudgetManager) {
	tag.mutex.Lock()
	defer tag.mutex.Unlock()

	tag.tokenBudget = budget

	// 更新所有管理器的Token预算
	if tag.worldviewManager != nil {
		tag.worldviewManager.SetTokenBudget(budget)
	}
	if tag.characterManager != nil {
		tag.characterManager.SetTokenBudget(budget)
	}
	if tag.indexReader != nil {
		tag.indexReader.SetTokenBudget(budget)
	}
	if tag.plannerManager != nil {
		tag.plannerManager.SetTokenBudget(budget)
	}
}


// GetMetrics 获取生成器指标
func (tag *TokenAwareGenerator) GetMetrics() *GeneratorMetrics {
	return tag.metrics
}

// GetConfig 获取配置
func (tag *TokenAwareGenerator) GetConfig() *GeneratorConfig {
	return tag.config
}

// UpdateConfig 更新配置
func (tag *TokenAwareGenerator) UpdateConfig(newConfig *GeneratorConfig) error {
	tag.mutex.Lock()
	defer tag.mutex.Unlock()

	if newConfig.NovelDir != tag.config.NovelDir {
		return content.NewInvalidConfigError("cannot change novel directory", nil)
	}

	// 更新Token预算
	if newConfig.MaxTokens != tag.config.MaxTokens ||
		newConfig.TokenPercentages != tag.config.TokenPercentages {

		tokenBudget, err := token.NewTokenBudgetManager(
			newConfig.MaxTokens, newConfig.TokenPercentages)
		if err != nil {
			return err
		}

		tag.SetTokenBudget(tokenBudget)
	}


	tag.config = newConfig
	return nil
}
