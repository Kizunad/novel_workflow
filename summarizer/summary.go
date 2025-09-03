package summarizer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ChapterSummary 章节摘要
type ChapterSummary struct {
	ChapterID string    `json:"chapter_id"`
	Title     string    `json:"title"`
	Summary   string    `json:"summary"`
	WordCount int       `json:"word_count"`
	Timestamp time.Time `json:"timestamp"`
}

// IndexFile 索引文件结构
type IndexFile struct {
	Version          string           `json:"version"`
	LastUpdate       time.Time        `json:"last_update"`
	TotalChapters    int              `json:"total_chapters"`
	Summaries        []ChapterSummary `json:"summaries"`
	ProcessingStatus ProcessingStatus `json:"processing_status"`
}

// ProcessingStatus 处理状态
type ProcessingStatus struct {
	Pending []string `json:"pending"`
	Failed  []string `json:"failed"`
}

// IndexManager 索引管理器
type IndexManager struct {
	indexPath string
}

// NewIndexManager 创建索引管理器
func NewIndexManager(novelPath string) *IndexManager {
	indexPath := filepath.Join(novelPath, "index.json")
	return &IndexManager{
		indexPath: indexPath,
	}
}

// LoadIndex 加载索引文件
func (im *IndexManager) LoadIndex() (*IndexFile, error) {
	if _, err := os.Stat(im.indexPath); os.IsNotExist(err) {
		// 文件不存在，创建新索引
		return &IndexFile{
			Version:    "1.0",
			LastUpdate: time.Now(),
			Summaries:  make([]ChapterSummary, 0),
			ProcessingStatus: ProcessingStatus{
				Pending: make([]string, 0),
				Failed:  make([]string, 0),
			},
		}, nil
	}

	data, err := os.ReadFile(im.indexPath)
	if err != nil {
		return nil, fmt.Errorf("读取索引文件失败: %w", err)
	}

	var index IndexFile
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("解析索引文件失败: %w", err)
	}

	return &index, nil
}

// SaveIndex 保存索引文件
func (im *IndexManager) SaveIndex(index *IndexFile) error {
	index.LastUpdate = time.Now()
	index.TotalChapters = len(index.Summaries)

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化索引失败: %w", err)
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(im.indexPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	if err := os.WriteFile(im.indexPath, data, 0644); err != nil {
		return fmt.Errorf("写入索引文件失败: %w", err)
	}

	return nil
}

// UpdateSummary 更新章节摘要
func (im *IndexManager) UpdateSummary(summary ChapterSummary) error {
	index, err := im.LoadIndex()
	if err != nil {
		return fmt.Errorf("加载索引失败: %w", err)
	}

	// 查找并更新现有摘要，或添加新摘要
	found := false
	for i, existing := range index.Summaries {
		if existing.ChapterID == summary.ChapterID {
			index.Summaries[i] = summary
			found = true
			break
		}
	}

	if !found {
		index.Summaries = append(index.Summaries, summary)
	}

	return im.SaveIndex(index)
}

// GetSummaries 获取所有摘要
func (im *IndexManager) GetSummaries() ([]ChapterSummary, error) {
	index, err := im.LoadIndex()
	if err != nil {
		return nil, err
	}
	return index.Summaries, nil
}

// GetRecentSummaries 获取最近N个章节的摘要
func (im *IndexManager) GetRecentSummaries(count int) ([]ChapterSummary, error) {
	summaries, err := im.GetSummaries()
	if err != nil {
		return nil, err
	}

	if len(summaries) <= count {
		return summaries, nil
	}

	return summaries[len(summaries)-count:], nil
}
