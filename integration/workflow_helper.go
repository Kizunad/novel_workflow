package integration

import (
	"fmt"

	"github.com/Kizunad/modular-workflow-v2/components/content"
	"github.com/Kizunad/modular-workflow-v2/components/content/managers"
	"github.com/Kizunad/modular-workflow-v2/components/content/token"
	"github.com/Kizunad/modular-workflow-v2/logger"
)

// WorkflowHelper 工作流辅助器
// 提供增强的上下文生成功能，同时保持与现有workflow的兼容性
type WorkflowHelper struct {
	contentBridge *ContentBridge
	logger        *logger.ZapLogger
	fallbackMode  bool
}

// NewWorkflowHelper 创建工作流辅助器
func NewWorkflowHelper(
	novelDir string,
	logger *logger.ZapLogger,
	maxTokens int,
	percentages *token.TokenPercentages) (*WorkflowHelper, error) {

	// 尝试创建Token感知的内容桥接器
	contentBridge, err := NewContentBridge(novelDir, logger, maxTokens, percentages)
	if err != nil {
		if logger != nil {
			logger.Warn("创建Token感知内容桥接器失败，使用回退模式: " + err.Error())
		}
		return &WorkflowHelper{
			logger:       logger,
			fallbackMode: true,
		}, nil
	}

	return &WorkflowHelper{
		contentBridge: contentBridge,
		logger:        logger,
		fallbackMode:  false,
	}, nil
}

// GetEnhancedDynamicContext 获取增强的动态上下文
// 这个函数可以直接替换workflow中的dynamicContext函数
func (wh *WorkflowHelper) GetEnhancedDynamicContext(
	generator *content.Generator,
	worldviewManager *managers.WorldviewManager,
	characterManager *managers.CharacterManager) map[string]any {

	// 如果Token感知系统可用，优先使用
	if !wh.fallbackMode && wh.contentBridge != nil {
		if wh.logger != nil {
			wh.logger.Debug("使用Token感知的上下文生成")
		}
		return wh.contentBridge.GetDynamicContext()
	}

	// 回退到原有实现
	if wh.logger != nil {
		wh.logger.Debug("使用传统上下文生成（回退模式）")
	}
	return wh.getFallbackDynamicContext(generator, worldviewManager, characterManager)
}

// GetEnhancedLimitedContext 获取增强的限制上下文
func (wh *WorkflowHelper) GetEnhancedLimitedContext(
	limitedGenerator *content.LimitedGenerator,
	generator *content.Generator,
	worldviewManager *managers.WorldviewManager,
	characterManager *managers.CharacterManager) (string, error) {

	// 如果Token感知系统可用，优先使用
	if !wh.fallbackMode && wh.contentBridge != nil {
		if wh.logger != nil {
			wh.logger.Debug("使用Token感知的限制上下文生成")
		}
		return wh.contentBridge.GetLimitedContext()
	}

	// 回退到原有实现
	if wh.logger != nil {
		wh.logger.Debug("使用传统限制上下文生成（回退模式）")
	}
	return wh.getFallbackLimitedContext(limitedGenerator, generator, worldviewManager, characterManager)
}

// getFallbackDynamicContext 回退的动态上下文实现
func (wh *WorkflowHelper) getFallbackDynamicContext(
	generator *content.Generator,
	worldviewManager *managers.WorldviewManager,
	characterManager *managers.CharacterManager) map[string]any {

	// 获取章节标题和摘要
	title := ""
	var summary string
	if generator != nil {
		indexReader := managers.NewIndexReader(generator.GetNovelDir())
		title = indexReader.GetTitle()
		if title == "" {
			title = "无章节标题"
		}
		// 获取章节摘要
		summary = indexReader.GetSummary()
		if summary == "" {
			summary = "暂无章节摘要"
		}
	}

	// 获取世界观信息
	worldview := ""
	if worldviewManager != nil {
		worldview = worldviewManager.GetCurrent()
	}
	if worldview == "" {
		if wh.logger != nil {
			wh.logger.Info("世界观文件不存在")
		}
		worldview = "暂无世界观设定"
	}

	// 获取角色信息
	characters := ""
	if characterManager != nil {
		characters = characterManager.GetCurrent()
	}

	// 获取章节内容
	chapter := ""
	if generator != nil {
		content, err := generator.Generate()
		if err != nil {
			if wh.logger != nil {
				wh.logger.Error(fmt.Sprintf("获取小说上下文失败: %v", err))
			}
		} else {
			chapter = content
		}
	}

	return map[string]any{
		"title":      title,
		"summary":    summary,
		"worldview":  worldview,
		"characters": characters,
		"chapter":    chapter,
		"context":    fmt.Sprintf("章节标题: %s\n\n章节摘要:\n%s\n\n世界观:\n%s\n\n角色信息:\n%s\n\n当前章节:\n%s", title, summary, worldview, characters, chapter),
	}
}

// getFallbackLimitedContext 回退的限制上下文实现
func (wh *WorkflowHelper) getFallbackLimitedContext(
	limitedGenerator *content.LimitedGenerator,
	generator *content.Generator,
	worldviewManager *managers.WorldviewManager,
	characterManager *managers.CharacterManager) (string, error) {

	if limitedGenerator == nil {
		return "", fmt.Errorf("LimitedGenerator未配置")
	}

	// 获取前两章内容
	limitedContent, err := limitedGenerator.Generate()
	if err != nil {
		return "", fmt.Errorf("生成限制上下文失败: %w", err)
	}

	// 获取动态上下文信息
	dynamicCtx := wh.getFallbackDynamicContext(generator, worldviewManager, characterManager)
	title := fmt.Sprintf("%v", dynamicCtx["title"])
	summary := fmt.Sprintf("%v", dynamicCtx["summary"])
	worldview := fmt.Sprintf("%v", dynamicCtx["worldview"])
	characters := fmt.Sprintf("%v", dynamicCtx["characters"])

	return fmt.Sprintf("章节标题: %s\n\n章节摘要:\n%s\n\n世界观:\n%s\n\n角色信息:\n%s\n\n最新章节(仅前两章):\n%s", title, summary, worldview, characters, limitedContent), nil
}

// UpdateTokenBudget 更新Token预算（如果支持）
func (wh *WorkflowHelper) UpdateTokenBudget(maxTokens int, percentages *token.TokenPercentages) error {
	if wh.fallbackMode || wh.contentBridge == nil {
		if wh.logger != nil {
			wh.logger.Info("当前使用回退模式，Token预算设置被忽略")
		}
		return nil
	}

	return wh.contentBridge.UpdateTokenBudget(maxTokens, percentages)
}

// IsTokenAware 检查是否启用了Token感知功能
func (wh *WorkflowHelper) IsTokenAware() bool {
	return !wh.fallbackMode && wh.contentBridge != nil
}

// GetStatus 获取状态信息
func (wh *WorkflowHelper) GetStatus() map[string]interface{} {
	status := map[string]interface{}{
		"fallback_mode": wh.fallbackMode,
		"token_aware":   wh.IsTokenAware(),
	}

	if wh.IsTokenAware() {
		// 可以添加Token使用统计等信息
		status["features"] = []string{"token_awareness", "context_caching", "smart_truncation"}
	} else {
		status["features"] = []string{"basic_context"}
	}

	return status
}