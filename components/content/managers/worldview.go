package managers

import (
	"fmt"
	"path/filepath"

	"github.com/Kizunad/modular-workflow-v2/components/content/token"
)

// WorldviewManager 世界观管理器
type WorldviewManager struct {
	*BaseFileManager
	novelDir string
}

// NewWorldviewManager 创建世界观管理器
func NewWorldviewManager(novelDir string) *WorldviewManager {
	worldviewPath := filepath.Join(novelDir, "worldview.md")
	
	manager := &WorldviewManager{
		BaseFileManager: NewBaseFileManager(worldviewPath),
		novelDir:        novelDir,
	}
	
	// 尝试加载现有内容
	manager.load()
	
	return manager
}

// NewWorldviewManagerWithTokenBudget 创建带Token预算的世界观管理器
func NewWorldviewManagerWithTokenBudget(novelDir string, tokenBudget *token.TokenBudgetManager) *WorldviewManager {
	manager := NewWorldviewManager(novelDir)
	manager.SetTokenBudget(tokenBudget)
	return manager
}

// load 加载世界观文件（内部方法）
func (wm *WorldviewManager) load() {
	// 尝试加载文件内容，忽略错误
	content, _ := wm.BaseFileManager.Load()
	if content == "" {
		// 如果文件不存在或为空，设置默认内容
		wm.BaseFileManager.current = ""
	}
}

// GetCurrentWithTokenLimit 获取限制Token数量的当前世界观
func (wm *WorldviewManager) GetCurrentWithTokenLimit(maxTokens int) (string, int) {
	current := wm.GetCurrent()
	if current == "" {
		return "", 0
	}
	
	// 如果有TokenBudget，使用正确的组件名
	if wm.GetTokenBudget() != nil {
		return wm.GetTokenBudget().TruncateToTokenLimit(current, "worldview")
	}
	
	// 否则使用基础的截断逻辑
	return wm.TruncateToLimit(current, maxTokens)
}

// UpdateWorldview 更新世界观（对外接口，保持向后兼容）
func (wm *WorldviewManager) UpdateWorldview(newWorldview string) error {
	return wm.Update(newWorldview)
}

// GetWorldviewPath 获取世界观文件路径
func (wm *WorldviewManager) GetWorldviewPath() string {
	return wm.GetPath()
}

// HasWorldview 检查是否有世界观设定
func (wm *WorldviewManager) HasWorldview() bool {
	return wm.Exists() && wm.GetCurrent() != ""
}

// GetWorldviewSummary 获取世界观摘要（前100个字符）
func (wm *WorldviewManager) GetWorldviewSummary() string {
	current := wm.GetCurrent()
	if len(current) <= 100 {
		return current
	}
	return current[:100] + "..."
}

// GetWorldviewMetadata 获取世界观元数据
func (wm *WorldviewManager) GetWorldviewMetadata() map[string]interface{} {
	info := wm.GetFileInfo()
	
	metadata := map[string]interface{}{
		"path":         info.Path,
		"exists":       info.Exists,
		"size":         info.Size,
		"mod_time":     info.ModTime,
		"token_count":  wm.GetTokenCount(),
		"has_content":  wm.HasWorldview(),
	}
	
	if wm.GetTokenBudget() != nil {
		estimated := wm.EstimateTokens()
		budget := wm.GetTokenBudget().GetTokenAllocation("worldview")
		metadata["token_estimate"] = estimated
		metadata["token_budget"] = budget
		metadata["within_budget"] = estimated <= budget
	}
	
	return metadata
}

// ValidateWorldview 验证世界观内容
func (wm *WorldviewManager) ValidateWorldview(content string) error {
	// 调用基础验证
	if err := wm.ValidateContent(content); err != nil {
		return err
	}
	
	// 世界观特定验证
	if len(content) > 10000 { // 世界观不应过长
		return fmt.Errorf("worldview content is too long")
	}
	
	return nil
}

// AppendToWorldview 追加内容到世界观
func (wm *WorldviewManager) AppendToWorldview(additionalContent string) error {
	if additionalContent == "" {
		return nil
	}
	
	current := wm.GetCurrent()
	if current == "" {
		return wm.Update(additionalContent)
	}
	
	updated := current + "\n\n" + additionalContent
	
	// 验证更新后的内容
	if err := wm.ValidateWorldview(updated); err != nil {
		return err
	}
	
	return wm.Update(updated)
}

// ClearWorldview 清空世界观
func (wm *WorldviewManager) ClearWorldview() error {
	return wm.Update("")
}

// ResetToDefault 重置为默认世界观模板
func (wm *WorldviewManager) ResetToDefault() error {
	defaultWorldview := `# 世界观设定

## 基础设定
- 时代背景：
- 地理环境：
- 社会结构：

## 魔法/科技系统
- 基本规则：
- 限制条件：
- 发展水平：

## 重要设定
- 政治体系：
- 经济体系：
- 宗教信仰：

## 其他设定
- 特殊规则：
- 历史背景：
`
	
	return wm.Update(defaultWorldview)
}