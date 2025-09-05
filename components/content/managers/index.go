package managers

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"time"

	"github.com/Kizunad/modular-workflow-v2/components/content/token"
	content "github.com/Kizunad/modular-workflow-v2/components/content/utils"
)

// ChapterSummary 章节摘要结构
type ChapterSummary struct {
	ChapterID string `json:"chapter_id"`
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	WordCount int    `json:"word_count"`
	Timestamp string `json:"timestamp"`
}

// IndexJSON index.json文件结构
type IndexJSON struct {
	Version    string           `json:"version"`
	LastUpdate string           `json:"last_update"`
	Chapters   int              `json:"total_chapters"`
	Summaries  []ChapterSummary `json:"summaries"`
}

// IndexReader 索引读取器，用于读取小说目录的索引信息
type IndexReader struct {
	*BaseFileManager
	novelDir  string
	titlePath string
	indexPath string
	indexData *IndexJSON
}

// NewIndexReader 创建新的索引读取器
func NewIndexReader(novelDir string) *IndexReader {
	indexPath := filepath.Join(novelDir, "index.json")
	titlePath := filepath.Join(novelDir, "title")
	
	reader := &IndexReader{
		BaseFileManager: NewBaseFileManager(indexPath),
		novelDir:        novelDir,
		titlePath:       titlePath,
		indexPath:       indexPath,
	}
	
	// 尝试加载现有数据
	reader.load()
	
	return reader
}

// NewIndexReaderWithTokenBudget 创建带Token预算的索引读取器
func NewIndexReaderWithTokenBudget(novelDir string, tokenBudget *token.TokenBudgetManager) *IndexReader {
	reader := NewIndexReader(novelDir)
	reader.SetTokenBudget(tokenBudget)
	return reader
}

// load 加载索引和标题数据（内部方法）
func (ir *IndexReader) load() {
	// 加载索引数据
	if content, err := ir.BaseFileManager.Load(); err == nil && content != "" {
		var indexData IndexJSON
		if err := json.Unmarshal([]byte(content), &indexData); err == nil {
			ir.indexData = &indexData
		}
	}
	
	// 加载标题数据
	ir.loadTitle()
}

// loadTitle 加载标题（保留方法签名但简化实现）
func (ir *IndexReader) loadTitle() {
	// 方法保留用于向后兼容，实际逻辑已迁移到GetTitle
}

// GetTitle 获取小说标题
func (ir *IndexReader) GetTitle() string {
	titleManager := NewBaseFileManager(ir.titlePath)
	if content, err := titleManager.Load(); err == nil {
		return strings.TrimSpace(content)
	}
	return ""
}

// SetTitle 设置小说标题
func (ir *IndexReader) SetTitle(title string) error {
	titleManager := NewBaseFileManager(ir.titlePath)
	trimmedTitle := strings.TrimSpace(title)
	
	if err := titleManager.Save(trimmedTitle); err != nil {
		return err
	}
	
	return nil
}

// HasTitle 检查是否存在标题
func (ir *IndexReader) HasTitle() bool {
	titleManager := NewBaseFileManager(ir.titlePath)
	return titleManager.Exists()
}

// GetSummary 获取章节摘要汇总
func (ir *IndexReader) GetSummary() string {
	if ir.indexData == nil || len(ir.indexData.Summaries) == 0 {
		// 尝试重新加载
		ir.load()
		if ir.indexData == nil || len(ir.indexData.Summaries) == 0 {
			return ""
		}
	}
	
	var summaryBuilder strings.Builder
	for i, chapterSummary := range ir.indexData.Summaries {
		if i > 0 {
			summaryBuilder.WriteString("\n\n")
		}
		summaryBuilder.WriteString(chapterSummary.ChapterID)
		summaryBuilder.WriteString(": ")
		summaryBuilder.WriteString(chapterSummary.Summary)
	}
	
	return summaryBuilder.String()
}

// GetSummaryWithTokenLimit 获取限制Token数量的章节摘要
func (ir *IndexReader) GetSummaryWithTokenLimit(maxTokens int) (string, int) {
	summary := ir.GetSummary()
	if summary == "" {
		return "", 0
	}
	
	return ir.TruncateToLimit(summary, maxTokens)
}

// HasSummary 检查是否存在摘要
func (ir *IndexReader) HasSummary() bool {
	return ir.Exists() && ir.indexData != nil && len(ir.indexData.Summaries) > 0
}

// GetChapterCount 获取章节数量
func (ir *IndexReader) GetChapterCount() int {
	if ir.indexData == nil {
		ir.load()
	}
	if ir.indexData == nil {
		return 0
	}
	return ir.indexData.Chapters
}

// GetLatestChapter 获取最新章节摘要
func (ir *IndexReader) GetLatestChapter() *ChapterSummary {
	if ir.indexData == nil || len(ir.indexData.Summaries) == 0 {
		ir.load()
		if ir.indexData == nil || len(ir.indexData.Summaries) == 0 {
			return nil
		}
	}
	
	// 返回最后一个章节
	return &ir.indexData.Summaries[len(ir.indexData.Summaries)-1]
}

// GetChapterSummaries 获取所有章节摘要
func (ir *IndexReader) GetChapterSummaries() []ChapterSummary {
	if ir.indexData == nil {
		ir.load()
	}
	if ir.indexData == nil {
		return nil
	}
	return ir.indexData.Summaries
}

// GetRecentChapters 获取最近N章的摘要
func (ir *IndexReader) GetRecentChapters(count int) []ChapterSummary {
	summaries := ir.GetChapterSummaries()
	if len(summaries) == 0 {
		return nil
	}
	
	if count >= len(summaries) {
		return summaries
	}
	
	start := len(summaries) - count
	return summaries[start:]
}

// GetRecentSummary 获取最近N章的摘要文本
func (ir *IndexReader) GetRecentSummary(count int) string {
	recentChapters := ir.GetRecentChapters(count)
	if len(recentChapters) == 0 {
		return ""
	}
	
	var summaryBuilder strings.Builder
	for i, chapterSummary := range recentChapters {
		if i > 0 {
			summaryBuilder.WriteString("\n\n")
		}
		summaryBuilder.WriteString(chapterSummary.ChapterID)
		summaryBuilder.WriteString(": ")
		summaryBuilder.WriteString(chapterSummary.Summary)
	}
	
	return summaryBuilder.String()
}

// GetIndexMetadata 获取索引元数据
func (ir *IndexReader) GetIndexMetadata() map[string]interface{} {
	info := ir.GetFileInfo()
	
	metadata := map[string]interface{}{
		"index_path":     info.Path,
		"title_path":     ir.titlePath,
		"index_exists":   info.Exists,
		"title_exists":   ir.HasTitle(),
		"chapter_count":  ir.GetChapterCount(),
		"has_summaries":  ir.HasSummary(),
		"mod_time":       info.ModTime,
		"token_count":    ir.GetTokenCount(),
	}
	
	if ir.GetTokenBudget() != nil {
		estimated := ir.EstimateTokens()
		budget := ir.GetTokenBudget().GetTokenAllocation("index")
		metadata["token_estimate"] = estimated
		metadata["token_budget"] = budget
		metadata["within_budget"] = estimated <= budget
	}
	
	if ir.indexData != nil {
		metadata["version"] = ir.indexData.Version
		metadata["last_update"] = ir.indexData.LastUpdate
	}
	
	return metadata
}

// RefreshIndex 刷新索引数据（重新从文件加载）
func (ir *IndexReader) RefreshIndex() error {
	// 检查文件是否被修改
	if ir.IsModified() {
		ir.load()
	}
	return nil
}

// ValidateIndex 验证索引数据
func (ir *IndexReader) ValidateIndex() error {
	if ir.indexData == nil {
		return content.NewFileNotFoundError(ir.indexPath, nil)
	}
	
	// 检查版本
	if ir.indexData.Version == "" {
		return content.NewInvalidConfigError("index version is empty", nil)
	}
	
	// 检查章节数量一致性
	if ir.indexData.Chapters != len(ir.indexData.Summaries) {
		return content.NewInvalidConfigError("chapter count mismatch in index", nil)
	}
	
	return nil
}

// GetIndexContent 获取索引原始内容（用于TokenAware接口）
func (ir *IndexReader) GetIndexContent() string {
	content, _ := ir.BaseFileManager.Load()
	return content
}

// FormatIndexSummary 格式化索引摘要（用于上下文）
func (ir *IndexReader) FormatIndexSummary() string {
	title := ir.GetTitle()
	if title == "" {
		title = "未知小说"
	}
	
	summary := ir.GetSummary()
	if summary == "" {
		summary = "暂无章节摘要"
	}
	
	return "《" + title + "》\n\n" + summary
}

// IndexManager 索引管理器，提供写入功能，兼容旧的 summarizer.IndexManager 接口
type IndexManager struct {
	*IndexReader
}

// NewIndexManager 创建索引管理器，兼容旧的 summarizer.NewIndexManager 接口
func NewIndexManager(novelDir string) *IndexManager {
	return &IndexManager{
		IndexReader: NewIndexReader(novelDir),
	}
}

// GetIndexPath 获取索引文件路径，兼容旧的 summarizer.IndexManager 接口
func (im *IndexManager) GetIndexPath() string {
	return im.indexPath
}

// UpdateSummary 更新章节摘要到索引文件，兼容旧的 summarizer.IndexManager 接口
func (im *IndexManager) UpdateSummary(summary ChapterSummary) error {
	// 重新加载最新数据
	im.load()
	
	// 初始化索引数据结构
	if im.indexData == nil {
		im.indexData = &IndexJSON{
			Version:    "1.0",
			LastUpdate: time.Now().Format(time.RFC3339),
			Chapters:   0,
			Summaries:  make([]ChapterSummary, 0),
		}
	}
	
	// 查找是否已存在相同章节ID的摘要
	existingIndex := -1
	for i, existingSummary := range im.indexData.Summaries {
		if existingSummary.ChapterID == summary.ChapterID {
			existingIndex = i
			break
		}
	}
	
	// 处理时间戳格式兼容性（summary_crud_tool.go 使用 time.Time，而这里使用 string）
	timestampStr := summary.Timestamp
	if timestampStr == "" {
		timestampStr = time.Now().Format(time.RFC3339)
	}
	
	// 创建标准化的摘要对象
	standardSummary := ChapterSummary{
		ChapterID: summary.ChapterID,
		Title:     summary.Title,
		Summary:   summary.Summary,
		WordCount: summary.WordCount,
		Timestamp: timestampStr,
	}
	
	// 更新或添加摘要
	if existingIndex >= 0 {
		// 更新现有摘要
		im.indexData.Summaries[existingIndex] = standardSummary
	} else {
		// 添加新摘要
		im.indexData.Summaries = append(im.indexData.Summaries, standardSummary)
		im.indexData.Chapters = len(im.indexData.Summaries)
	}
	
	// 更新最后修改时间
	im.indexData.LastUpdate = time.Now().Format(time.RFC3339)
	
	// 序列化为JSON
	jsonData, err := json.MarshalIndent(im.indexData, "", "  ")
	if err != nil {
		return content.NewInvalidConfigError("failed to marshal index data", err)
	}
	
	// 保存到文件
	if err := im.BaseFileManager.Save(string(jsonData)); err != nil {
		return err
	}
	
	return nil
}