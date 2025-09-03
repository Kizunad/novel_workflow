package content

import (
	"errors"
	"fmt"
)

// LimitedGenerator 限制内容生成器，只读取最新的N章
type LimitedGenerator struct {
	novelDir string
	maxChapters int // 最大章节数
}

// NewLimitedGenerator 创建限制内容生成器
func NewLimitedGenerator(novelDir string, maxChapters int) *LimitedGenerator {
	return &LimitedGenerator{
		novelDir: novelDir,
		maxChapters: maxChapters,
	}
}

// Generate 生成限制的内容（只读取最新的N章）
func (lg *LimitedGenerator) Generate() (string, error) {
	// 创建章节管理器
	manager := NewChapterManager(lg.novelDir)
	
	// 获取所有章节
	chapters, err := manager.ReadChapters()
	if err != nil {
		// 如果是没有章节的错误，返回空字符串（正常情况）
		if errors.Is(err, ErrNoChapterFiles) || errors.Is(err, ErrNoChapters) {
			return "", nil
		}
		return "", err
	}

	// 如果章节数不超过限制，直接返回所有章节
	if len(chapters) <= lg.maxChapters {
		return lg.joinChapters(chapters), nil
	}

	// 取最新的maxChapters章（chapters已经按编号排序了）
	latestChapters := chapters[len(chapters)-lg.maxChapters:]
	
	return lg.joinChapters(latestChapters), nil
}

// joinChapters 拼接章节内容
func (lg *LimitedGenerator) joinChapters(chapters []*Chapter) string {
	var result string
	for i, chapter := range chapters {
		if i > 0 {
			result += "\n\n---\n\n"
		}
		result += fmt.Sprintf("=== %s ===\n", chapter.Title)
		
		// 拼接段落内容
		for _, paragraph := range chapter.Content {
			result += paragraph.Text + "\n"
		}
	}
	return result
}