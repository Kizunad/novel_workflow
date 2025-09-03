package context

import (
	"fmt"
	"time"
)

// NovelContext 小说上下文数据结构
type NovelContext struct {
	// 基础信息
	Title   string `json:"title"`
	Summary string `json:"summary"`

	// 设定信息
	Worldview  string `json:"worldview"`
	Characters string `json:"characters"`

	// 内容信息
	Chapters string `json:"chapters"`
	Plan     string `json:"plan"`
	Index    string `json:"index"`

	// 元数据
	Metadata *ContextMetadata `json:"metadata"`
}

// ContextMetadata 上下文元数据
type ContextMetadata struct {
	// Token统计
	TokenCounts map[string]int `json:"token_counts"`
	TotalTokens int            `json:"total_tokens"`

	// 文件信息
	FilePaths    map[string]string    `json:"file_paths"`
	FileModTimes map[string]time.Time `json:"file_mod_times"`
	LastUpdated  time.Time            `json:"last_updated"`
}

// NewNovelContext 创建新的小说上下文
func NewNovelContext() *NovelContext {
	return &NovelContext{
		Metadata: &ContextMetadata{
			TokenCounts:  make(map[string]int),
			FilePaths:    make(map[string]string),
			FileModTimes: make(map[string]time.Time),
			LastUpdated:  time.Now(),
		},
	}
}

// SetTokenCount 设置组件Token数
func (nc *NovelContext) SetTokenCount(component string, count int) {
	if nc.Metadata == nil {
		nc.Metadata = &ContextMetadata{
			TokenCounts: make(map[string]int),
		}
	}
	nc.Metadata.TokenCounts[component] = count

	// 重新计算总Token数
	nc.recalculateTotalTokens()
}

// GetTokenCount 获取组件Token数
func (nc *NovelContext) GetTokenCount(component string) int {
	if nc.Metadata == nil || nc.Metadata.TokenCounts == nil {
		return 0
	}
	return nc.Metadata.TokenCounts[component]
}

// recalculateTotalTokens 重新计算总Token数
func (nc *NovelContext) recalculateTotalTokens() {
	if nc.Metadata == nil || nc.Metadata.TokenCounts == nil {
		nc.Metadata.TotalTokens = 0
		return
	}

	total := 0
	for _, count := range nc.Metadata.TokenCounts {
		total += count
	}
	nc.Metadata.TotalTokens = total
}

// SetFilePath 设置文件路径
func (nc *NovelContext) SetFilePath(component, path string) {
	if nc.Metadata == nil {
		nc.Metadata = &ContextMetadata{
			FilePaths: make(map[string]string),
		}
	}
	nc.Metadata.FilePaths[component] = path
}

// GetFilePath 获取文件路径
func (nc *NovelContext) GetFilePath(component string) string {
	if nc.Metadata == nil || nc.Metadata.FilePaths == nil {
		return ""
	}
	return nc.Metadata.FilePaths[component]
}

// SetFileModTime 设置文件修改时间
func (nc *NovelContext) SetFileModTime(component string, modTime time.Time) {
	if nc.Metadata == nil {
		nc.Metadata = &ContextMetadata{
			FileModTimes: make(map[string]time.Time),
		}
	}
	nc.Metadata.FileModTimes[component] = modTime
}

// GetFileModTime 获取文件修改时间
func (nc *NovelContext) GetFileModTime(component string) time.Time {
	if nc.Metadata == nil || nc.Metadata.FileModTimes == nil {
		return time.Time{}
	}
	return nc.Metadata.FileModTimes[component]
}

// UpdateTimestamp 更新时间戳
func (nc *NovelContext) UpdateTimestamp() {
	if nc.Metadata == nil {
		nc.Metadata = &ContextMetadata{}
	}
	nc.Metadata.LastUpdated = time.Now()
}


// GetContentAsMap 获取内容映射
func (nc *NovelContext) GetContentAsMap() map[string]string {
	return map[string]string{
		"title":      nc.Title,
		"summary":    nc.Summary,
		"worldview":  nc.Worldview,
		"characters": nc.Characters,
		"chapters":   nc.Chapters,
		"plan":       nc.Plan,
		"index":      nc.Index,
	}
}

// SetFromMap 从映射设置内容
func (nc *NovelContext) SetFromMap(content map[string]string) {
	if title, exists := content["title"]; exists {
		nc.Title = title
	}
	if summary, exists := content["summary"]; exists {
		nc.Summary = summary
	}
	if worldview, exists := content["worldview"]; exists {
		nc.Worldview = worldview
	}
	if characters, exists := content["characters"]; exists {
		nc.Characters = characters
	}
	if chapters, exists := content["chapters"]; exists {
		nc.Chapters = chapters
	}
	if plan, exists := content["plan"]; exists {
		nc.Plan = plan
	}
	if index, exists := content["index"]; exists {
		nc.Index = index
	}
}

// FormatContext 格式化上下文为字符串（用于工作流）
func (nc *NovelContext) FormatContext() string {
	return fmt.Sprintf(`章节标题: %s

章节摘要:
%s

世界观:
%s

角色信息:
%s

规划内容:
%s

当前章节:
%s

索引信息:
%s`, nc.Title, nc.Summary, nc.Worldview, nc.Characters, nc.Plan, nc.Chapters, nc.Index)
}

// FormatLimitedContext 格式化限制版本的上下文
func (nc *NovelContext) FormatLimitedContext() string {
	return fmt.Sprintf(`章节标题: %s

章节摘要:
%s

世界观:
%s

角色信息:
%s

最新章节(Token限制版):
%s`, nc.Title, nc.Summary, nc.Worldview, nc.Characters, nc.Chapters)
}

// Clone 克隆上下文
func (nc *NovelContext) Clone() *NovelContext {
	clone := &NovelContext{
		Title:      nc.Title,
		Summary:    nc.Summary,
		Worldview:  nc.Worldview,
		Characters: nc.Characters,
		Chapters:   nc.Chapters,
		Plan:       nc.Plan,
		Index:      nc.Index,
	}

	// 克隆元数据
	if nc.Metadata != nil {
		clone.Metadata = &ContextMetadata{
			TotalTokens: nc.Metadata.TotalTokens,
			LastUpdated: nc.Metadata.LastUpdated,
		}

		// 深拷贝maps
		if nc.Metadata.TokenCounts != nil {
			clone.Metadata.TokenCounts = make(map[string]int)
			for k, v := range nc.Metadata.TokenCounts {
				clone.Metadata.TokenCounts[k] = v
			}
		}

		if nc.Metadata.FilePaths != nil {
			clone.Metadata.FilePaths = make(map[string]string)
			for k, v := range nc.Metadata.FilePaths {
				clone.Metadata.FilePaths[k] = v
			}
		}

		if nc.Metadata.FileModTimes != nil {
			clone.Metadata.FileModTimes = make(map[string]time.Time)
			for k, v := range nc.Metadata.FileModTimes {
				clone.Metadata.FileModTimes[k] = v
			}
		}
	}

	return clone
}

// IsEmpty 检查上下文是否为空
func (nc *NovelContext) IsEmpty() bool {
	return nc.Title == "" && nc.Summary == "" && nc.Worldview == "" &&
		nc.Characters == "" && nc.Chapters == "" && nc.Plan == "" && nc.Index == ""
}
