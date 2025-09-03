package character_update

import (
	"fmt"
	"time"
)

// CharacterUpdateTask 角色更新任务
type CharacterUpdateTask struct {
	ID            string    `json:"id"`
	NovelPath     string    `json:"novel_path"`
	CharacterName string    `json:"character_name"`
	UpdateContent string    `json:"update_content"`
	Priority      int       `json:"priority"`
	CreatedAt     time.Time `json:"created_at"`
}

// 实现 Task 接口
func (t *CharacterUpdateTask) GetID() string {
	return t.ID
}

func (t *CharacterUpdateTask) GetType() string {
	return "character_update"
}

func (t *CharacterUpdateTask) GetPriority() int {
	return t.Priority
}

func (t *CharacterUpdateTask) GetPayload() interface{} {
	return t
}

// NewCharacterUpdateTask 创建角色更新任务
func NewCharacterUpdateTask(novelPath, characterName, updateContent string) *CharacterUpdateTask {
	return &CharacterUpdateTask{
		ID:            generateTaskID(),
		NovelPath:     novelPath,
		CharacterName: characterName,
		UpdateContent: updateContent,
		Priority:      2, // 中等优先级
		CreatedAt:     time.Now(),
	}
}

// generateTaskID 生成任务ID
func generateTaskID() string {
	return fmt.Sprintf("char_%d", time.Now().UnixNano())
}
