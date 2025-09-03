package worldview_summarizer

import (
	"fmt"
	"time"
)

// WorldviewSummarizerTask 世界观总结任务
type WorldviewSummarizerTask struct {
	ID            string    `json:"id"`
	NovelPath     string    `json:"novel_path"`
	UpdateContent string    `json:"update_content"`
	Priority      int       `json:"priority"`
	CreatedAt     time.Time `json:"created_at"`
}

// 实现 Task 接口
func (t *WorldviewSummarizerTask) GetID() string {
	return t.ID
}

func (t *WorldviewSummarizerTask) GetType() string {
	return "worldview_summarizer"
}

func (t *WorldviewSummarizerTask) GetPriority() int {
	return t.Priority
}

func (t *WorldviewSummarizerTask) GetPayload() interface{} {
	return t
}

// NewWorldviewSummarizerTask 创建世界观总结任务
func NewWorldviewSummarizerTask(novelPath, updateContent string) *WorldviewSummarizerTask {
	return &WorldviewSummarizerTask{
		ID:            generateTaskID(),
		NovelPath:     novelPath,
		UpdateContent: updateContent,
		Priority:      2, // 中等优先级
		CreatedAt:     time.Now(),
	}
}

// generateTaskID 生成任务ID
func generateTaskID() string {
	return fmt.Sprintf("worldview_%d", time.Now().UnixNano())
}
