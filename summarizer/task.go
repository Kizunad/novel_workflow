package summarizer

import (
	"fmt"
	"time"
)

// SummarizeTask 摘要任务
type SummarizeTask struct {
	ID          string    `json:"id"`
	NovelPath   string    `json:"novel_path"`
	Content     string    `json:"content"`
	ChapterPath string    `json:"chapter_path,omitempty"`
	Priority    int       `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
}

// 实现 Task 接口
func (t *SummarizeTask) GetID() string {
	return t.ID
}

func (t *SummarizeTask) GetType() string {
	return "summarize"
}

func (t *SummarizeTask) GetPriority() int {
	return t.Priority
}

func (t *SummarizeTask) GetPayload() interface{} {
	return t
}

// NewSummarizeTask 创建摘要任务
func NewSummarizeTask(novelPath, content string) *SummarizeTask {
	return &SummarizeTask{
		ID:        generateTaskID(),
		NovelPath: novelPath,
		Content:   content,
		Priority:  1,
		CreatedAt: time.Now(),
	}
}

// generateTaskID 生成任务ID
func generateTaskID() string {
	return fmt.Sprintf("sum_%d", time.Now().UnixNano())
}