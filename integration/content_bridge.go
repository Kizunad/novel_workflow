package integration

import (
	"github.com/Kizunad/modular-workflow-v2/components/content"
	"github.com/Kizunad/modular-workflow-v2/components/content/managers"
	"github.com/Kizunad/modular-workflow-v2/components/content/token"
	"github.com/Kizunad/modular-workflow-v2/logger"
)

// ContentBridge 内容系统桥接器（简化版）
// 用于在新旧内容系统之间提供兼容层
type ContentBridge struct {
	logger *logger.ZapLogger
}

// NewContentBridge 创建内容桥接器
func NewContentBridge(
	novelDir string,
	logger *logger.ZapLogger,
	maxTokens int,
	percentages *token.TokenPercentages) (*ContentBridge, error) {

	return &ContentBridge{
		logger: logger,
	}, nil
}

// CreateCompatibleManagers 创建兼容的管理器
func (cb *ContentBridge) CreateCompatibleManagers(novelDir string) (
	*managers.WorldviewManager,
	*managers.CharacterManager,
	*content.Generator,
	*content.LimitedGenerator,
	error) {

	// 对于现有的简单类型，我们直接使用原来的实现
	// 因为它们的接口已经足够简单，不需要Token感知
	worldviewManager := managers.NewWorldviewManager(novelDir)
	characterManager := managers.NewCharacterManager(novelDir)
	generator := content.NewGenerator(novelDir)
	limitedGenerator := content.NewLimitedGenerator(novelDir, 2) // 保持原有的2章限制

	return worldviewManager, characterManager, generator, limitedGenerator, nil
}

// GetDynamicContext 获取动态上下文（供workflow使用）
func (cb *ContentBridge) GetDynamicContext() map[string]any {
	// 简化实现，返回基本上下文
	return map[string]any{
		"title":      "Token感知系统（开发中）",
		"summary":    "Token感知上下文生成",
		"worldview":  "暂无世界观设定",
		"characters": "暂无角色信息",
		"chapter":    "Token感知章节内容",
		"context":    "Token感知系统正在运行",
	}
}

// GetLimitedContext 获取限制上下文（供workflow使用）
func (cb *ContentBridge) GetLimitedContext() (string, error) {
	return "Token感知限制上下文（简化版本）", nil
}

// UpdateTokenBudget 更新Token预算
func (cb *ContentBridge) UpdateTokenBudget(maxTokens int, percentages *token.TokenPercentages) error {
	if cb.logger != nil {
		cb.logger.Info("Token预算更新（简化实现）")
	}
	return nil
}