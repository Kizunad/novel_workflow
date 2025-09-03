package managers

import (
	"path/filepath"
	"strings"

	"github.com/Kizunad/modular-workflow-v2/components/content/token"
	content "github.com/Kizunad/modular-workflow-v2/components/content/utils"
)

// CharacterManager 角色管理器
type CharacterManager struct {
	*BaseFileManager
	novelDir string
}

// NewCharacterManager 创建角色管理器
func NewCharacterManager(novelDir string) *CharacterManager {
	characterPath := filepath.Join(novelDir, "character.md")

	manager := &CharacterManager{
		BaseFileManager: NewBaseFileManager(characterPath),
		novelDir:        novelDir,
	}

	// 尝试加载现有内容
	manager.load()

	return manager
}

// NewCharacterManagerWithTokenBudget 创建带Token预算的角色管理器
func NewCharacterManagerWithTokenBudget(novelDir string, tokenBudget *token.TokenBudgetManager) *CharacterManager {
	manager := NewCharacterManager(novelDir)
	manager.SetTokenBudget(tokenBudget)
	return manager
}

// load 加载角色文件（内部方法）
func (cm *CharacterManager) load() {
	// 尝试加载文件内容，忽略错误
	content, _ := cm.BaseFileManager.Load()
	if content != "" {
		cm.BaseFileManager.current = strings.TrimSpace(content)
	}
}

// GetCurrentWithTokenLimit 获取限制Token数量的当前角色信息
func (cm *CharacterManager) GetCurrentWithTokenLimit(maxTokens int) (string, int) {
	current := cm.GetCurrent()
	if current == "" {
		return "", 0
	}

	// 如果有TokenBudget，使用正确的组件名
	if cm.GetTokenBudget() != nil {
		return cm.GetTokenBudget().TruncateToTokenLimit(current, "character")
	}
	
	// 否则使用基础的截断逻辑
	return cm.TruncateToLimit(current, maxTokens)
}

// UpdateCharacter 更新角色信息（对外接口，保持向后兼容）
func (cm *CharacterManager) UpdateCharacter(characterInfo string) error {
	trimmed := strings.TrimSpace(characterInfo)
	return cm.Update(trimmed)
}

// GetCharacterPath 获取角色文件路径
func (cm *CharacterManager) GetCharacterPath() string {
	return cm.GetPath()
}

// HasCharacters 检查是否有角色设定
func (cm *CharacterManager) HasCharacters() bool {
	return cm.Exists() && cm.GetCurrent() != ""
}

// TODO: 逻辑问题
// GetCharacterCount 获取角色数量（简单统计）
func (cm *CharacterManager) GetCharacterCount() int {
	current := cm.GetCurrent()
	if current == "" {
		return 0
	}

	// 简单计算：以"##"开头的行作为角色
	lines := strings.Split(current, "\n")
	count := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "##") {
			count++
		}
	}

	if count == 0 && current != "" {
		// 如果没有标准格式但有内容，认为至少有1个角色
		return 1
	}

	return count
}

// GetCharacterSummary 获取角色摘要（前150个字符）
func (cm *CharacterManager) GetCharacterSummary() string {
	current := cm.GetCurrent()
	if len(current) <= 150 {
		return current
	}
	return current[:150] + "..."
}

// GetCharacterMetadata 获取角色元数据
func (cm *CharacterManager) GetCharacterMetadata() map[string]interface{} {
	info := cm.GetFileInfo()

	metadata := map[string]interface{}{
		"path":            info.Path,
		"exists":          info.Exists,
		"size":            info.Size,
		"mod_time":        info.ModTime,
		"token_count":     cm.GetTokenCount(),
		"has_characters":  cm.HasCharacters(),
		"character_count": cm.GetCharacterCount(),
	}

	if cm.GetTokenBudget() != nil {
		estimated := cm.EstimateTokens()
		budget := cm.GetTokenBudget().GetTokenAllocation("character")
		metadata["token_estimate"] = estimated
		metadata["token_budget"] = budget
		metadata["within_budget"] = estimated <= budget
	}

	return metadata
}

// ValidateCharacter 验证角色内容
func (cm *CharacterManager) ValidateCharacter(characterInfo string) error {
	// 调用基础验证
	if err := cm.ValidateContent(characterInfo); err != nil {
		return err
	}

	// 角色特定验证
	if len(characterInfo) > 8000 { // 角色信息不应过长
		return content.NewInvalidConfigError("character content is too long", nil)
	}

	return nil
}

// AddCharacter 添加新角色
func (cm *CharacterManager) AddCharacter(name, description string) error {
	if name == "" {
		return content.NewInvalidConfigError("character name cannot be empty", nil)
	}

	current := cm.GetCurrent()

	// 构建角色条目
	characterEntry := "\n\n## " + name + "\n" + description

	var updated string
	if current == "" {
		// 如果是第一个角色，添加标题
		updated = "# 角色设定" + characterEntry
	} else {
		updated = current + characterEntry
	}

	// 验证更新后的内容
	if err := cm.ValidateCharacter(updated); err != nil {
		return err
	}

	return cm.Update(updated)
}

// UpdateCharacterByName 根据名称更新特定角色
func (cm *CharacterManager) UpdateCharacterByName(name, newDescription string) error {
	if name == "" {
		return content.NewInvalidConfigError("character name cannot be empty", nil)
	}

	current := cm.GetCurrent()
	if current == "" {
		// 如果没有角色，直接添加
		return cm.AddCharacter(name, newDescription)
	}

	lines := strings.Split(current, "\n")
	var updated []string
	inTargetCharacter := false

	for i, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		// 检查是否是角色标题
		if strings.HasPrefix(trimmedLine, "##") {
			characterName := strings.TrimSpace(strings.TrimPrefix(trimmedLine, "##"))
			if characterName == name {
				// 找到目标角色
				inTargetCharacter = true
				updated = append(updated, line) // 保持原标题
				continue
			} else {
				inTargetCharacter = false
			}
		}

		// 如果在目标角色内，并且遇到下一个角色或到达文件末尾
		if inTargetCharacter {
			// 如果是下一个角色标题，结束当前角色编辑
			if strings.HasPrefix(trimmedLine, "##") {
				// 添加新描述
				updated = append(updated, newDescription)
				updated = append(updated, line) // 添加下一个角色标题
				inTargetCharacter = false
				continue
			}

			// 如果是最后一行，添加新描述
			if i == len(lines)-1 {
				updated = append(updated, newDescription)
				continue
			}

			// 跳过旧内容，除非是空行
			if trimmedLine == "" {
				updated = append(updated, line)
			}
			continue
		}

		updated = append(updated, line)
	}

	// 如果遍历结束时还在目标角色内，添加新描述
	if inTargetCharacter {
		updated = append(updated, newDescription)
	}

	result := strings.Join(updated, "\n")

	// 验证更新后的内容
	if err := cm.ValidateCharacter(result); err != nil {
		return err
	}

	return cm.Update(result)
}

// ClearCharacters 清空角色信息
func (cm *CharacterManager) ClearCharacters() error {
	return cm.Update("")
}

// ResetToDefault 重置为默认角色模板
func (cm *CharacterManager) ResetToDefault() error {
	defaultCharacters := `# 角色设定

## 主角
- 姓名：
- 年龄：
- 性格：
- 背景：
- 能力：
- 目标：

## 重要配角
### 角色A
- 描述：

### 角色B
- 描述：
`

	return cm.Update(defaultCharacters)
}
