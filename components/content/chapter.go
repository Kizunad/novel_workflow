package content

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// 定义专用错误变量
var (
	ErrNovelDirNotExist    = errors.New("小说目录不存在")
	ErrNoChapterFiles      = errors.New("未找到任何章节文件")
	ErrNoChapters          = errors.New("未找到任何章节")
	ErrReadChaptersFailed  = errors.New("读取章节失败")
)

// ChapterParagraph 章节段落结构
type ChapterParagraph struct {
	ParagraphID int    `json:"paragraph_id"`
	Text        string `json:"text"`
}

// Chapter 章节结构
type Chapter struct {
	ChapterID string             `json:"chapter_id"`
	Title     string             `json:"title"`
	Content   []ChapterParagraph `json:"content"`
}

// ChapterManager 章节管理器，整合读写功能
type ChapterManager struct {
	novelDir string
}

// NewChapterManager 创建新的章节管理器
func NewChapterManager(novelDir string) *ChapterManager {
	return &ChapterManager{
		novelDir: novelDir,
	}
}

// === 读取功能 ===

// ReadChapters 读取所有章节并按编号排序
func (cm *ChapterManager) ReadChapters() ([]*Chapter, error) {
	// 确保目录存在
	if _, err := os.Stat(cm.novelDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("%w: %s", ErrNovelDirNotExist, cm.novelDir)
	}

	// 读取目录中的所有文件
	files, err := os.ReadDir(cm.novelDir)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrReadChaptersFailed, err)
	}

	// 找到所有章节文件
	chapterPattern := regexp.MustCompile(`^example_chapter_(\d+)\.json$`)
	var chapterFiles []string

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		if chapterPattern.MatchString(file.Name()) {
			chapterFiles = append(chapterFiles, file.Name())
		}
	}

	if len(chapterFiles) == 0 {
		return nil, ErrNoChapterFiles
	}

	// 按章节编号排序
	sort.Slice(chapterFiles, func(i, j int) bool {
		numI := cm.extractChapterNumber(chapterFiles[i])
		numJ := cm.extractChapterNumber(chapterFiles[j])
		return numI < numJ // 升序排列
	})

	// 读取所有章节
	var chapters []*Chapter
	for _, filename := range chapterFiles {
		chapter, err := cm.readSingleChapter(filename)
		if err != nil {
			return nil, fmt.Errorf("%w: 读取章节文件 %s 失败: %w", ErrReadChaptersFailed, filename, err)
		}
		chapters = append(chapters, chapter)
	}

	return chapters, nil
}

// GetLatestChapter 获取最新章节
func (cm *ChapterManager) GetLatestChapter() (*Chapter, error) {
	chapters, err := cm.ReadChapters()
	if err != nil {
		return nil, err
	}

	if len(chapters) == 0 {
		return nil, ErrNoChapters
	}

	// 返回最后一个（最新的）章节
	return chapters[len(chapters)-1], nil
}

// GetLatestChapterContent 获取最新章节的完整内容文本
func (cm *ChapterManager) GetLatestChapterContent() (string, error) {
	chapter, err := cm.GetLatestChapter()
	if err != nil {
		return "", err
	}

	return cm.concatenateChapterContent(chapter), nil
}

// GetAllChaptersContent 读取所有章节并拼接成一个字符串
func (cm *ChapterManager) GetAllChaptersContent() (string, error) {
	// 读取所有章节
	chapters, err := cm.ReadChapters()
	if err != nil {
		return "", err
	}
	
	if len(chapters) == 0 {
		return "", ErrNoChapters
	}
	
	var contentBuilder strings.Builder
	
	// 对每个章节调用 concatenateChapterContent 并拼接
	for i, chapter := range chapters {
		chapterContent := cm.concatenateChapterContent(chapter)
		contentBuilder.WriteString(chapterContent)
		
		// 章节间添加分隔符，最后一章不添加
		if i < len(chapters)-1 {
			contentBuilder.WriteString("\n\n---\n\n")
		}
	}
	
	return contentBuilder.String(), nil
}

// concatenateChapterContent 拼接章节内容
func (cm *ChapterManager) concatenateChapterContent(chapter *Chapter) string {
	var contentBuilder strings.Builder
	
	for i, paragraph := range chapter.Content {
		contentBuilder.WriteString(paragraph.Text)
		// 段落间添加换行，但最后一个段落不添加
		if i < len(chapter.Content)-1 {
			contentBuilder.WriteString("\n\n")
		}
	}
	
	return contentBuilder.String()
}

// readSingleChapter 读取单个章节文件
func (cm *ChapterManager) readSingleChapter(filename string) (*Chapter, error) {
	filePath := fmt.Sprintf("%s/%s", cm.novelDir, filename)
	
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	var chapter Chapter
	if err := json.Unmarshal(data, &chapter); err != nil {
		return nil, fmt.Errorf("解析章节 JSON 失败: %w", err)
	}

	return &chapter, nil
}

// extractChapterNumber 从文件名中提取章节编号
func (cm *ChapterManager) extractChapterNumber(filename string) int {
	re := regexp.MustCompile(`example_chapter_(\d+)\.json`)
	matches := re.FindStringSubmatch(filename)
	if len(matches) > 1 {
		if num, err := strconv.Atoi(matches[1]); err == nil {
			return num
		}
	}
	return 0
}

// === 写入功能 ===

// WriteChapter 写入新章节
func (cm *ChapterManager) WriteChapter(content string) (string, error) {
	// 获取下一个章节索引
	chapterIndex, err := cm.getNextChapterIndex()
	if err != nil {
		return "", err
	}
	
	// 分割内容为段落
	paragraphs := cm.splitContentToParagraphs(content)
	if len(paragraphs) == 0 {
		return "", fmt.Errorf("无法从内容中提取有效段落")
	}
	
	// 构建章节数据
	chapterID := fmt.Sprintf("%03d", chapterIndex)
	chapterContent := make([]ChapterParagraph, len(paragraphs))
	
	for i, paragraph := range paragraphs {
		chapterContent[i] = ChapterParagraph{
			ParagraphID: i + 1,
			Text:        paragraph,
		}
	}
	
	chapter := Chapter{
		ChapterID: chapterID,
		Title:     fmt.Sprintf("第%d章", chapterIndex),
		Content:   chapterContent,
	}
	
	// 生成文件路径
	filename := fmt.Sprintf("example_chapter_%d.json", chapterIndex)
	filepath := filepath.Join(cm.novelDir, filename)
	
	// 序列化为JSON
	jsonData, err := json.MarshalIndent(chapter, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化章节数据失败: %w", err)
	}
	
	// 写入文件
	if err := os.WriteFile(filepath, jsonData, 0644); err != nil {
		return "", fmt.Errorf("写入章节文件失败: %w", err)
	}
	
	return filepath, nil
}

// WriteChapterWithInfo 写入章节并返回详细信息
func (cm *ChapterManager) WriteChapterWithInfo(content string) (string, *Chapter, error) {
	filepath, err := cm.WriteChapter(content)
	if err != nil {
		return "", nil, err
	}
	
	// 读取写入的文件以返回章节信息
	data, err := os.ReadFile(filepath)
	if err != nil {
		return filepath, nil, fmt.Errorf("读取已写入章节失败: %w", err)
	}
	
	var chapter Chapter
	if err := json.Unmarshal(data, &chapter); err != nil {
		return filepath, nil, fmt.Errorf("解析章节数据失败: %w", err)
	}
	
	return filepath, &chapter, nil
}

// WriteContentToDirectory 将内容写入指定目录
func (cm *ChapterManager) WriteContentToDirectory(targetDir, content string) error {
	// 确保目录存在
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("创建小说章节目录失败: %w", err)
	}
	
	// 创建临时的ChapterManager实例指向目标目录
	tempManager := &ChapterManager{novelDir: targetDir}
	
	// 写入章节
	filepath, err := tempManager.WriteChapter(content)
	if err != nil {
		return fmt.Errorf("写入章节失败: %w", err)
	}
	
	// 记录成功信息（可选，用于调试）
	fmt.Printf("成功将内容写入: %s\n", filepath)
	
	return nil
}

// === 私有辅助方法 ===

// splitContentToParagraphs 将长文本分割为段落
func (cm *ChapterManager) splitContentToParagraphs(content string) []string {
	// 清理内容
	content = strings.TrimSpace(content)
	
	// 按双换行符分割段落
	paragraphs := regexp.MustCompile(`\n\s*\n`).Split(content, -1)
	
	var validParagraphs []string
	for _, paragraph := range paragraphs {
		paragraph = strings.TrimSpace(paragraph)
		// 过滤掉空段落和过短段落
		if len(paragraph) > 10 {
			validParagraphs = append(validParagraphs, paragraph)
		}
	}
	
	// 如果没有找到有效的段落分割，则按句号分割
	if len(validParagraphs) <= 1 {
		return cm.splitBySentences(content)
	}
	
	return validParagraphs
}

// splitBySentences 按句号分割内容为段落
func (cm *ChapterManager) splitBySentences(content string) []string {
	// 按句号、感叹号、问号分割
	sentences := regexp.MustCompile(`[。！？]\s*`).Split(content, -1)
	
	var paragraphs []string
	var currentParagraph string
	
	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if len(sentence) == 0 {
			continue
		}
		
		// 每2-3句组成一个段落
		if currentParagraph == "" {
			currentParagraph = sentence
		} else {
			currentParagraph += "。" + sentence
		}
		
		// 如果当前段落长度超过200字符，就作为一个段落
		if len(currentParagraph) > 200 {
			paragraphs = append(paragraphs, currentParagraph)
			currentParagraph = ""
		}
	}
	
	// 添加最后一个段落
	if currentParagraph != "" {
		paragraphs = append(paragraphs, currentParagraph)
	}
	
	return paragraphs
}

// getNextChapterIndex 获取下一个章节索引
func (cm *ChapterManager) getNextChapterIndex() (int, error) {
	// 确保小说目录存在
	if err := os.MkdirAll(cm.novelDir, 0755); err != nil {
		return 0, fmt.Errorf("创建小说目录失败: %w", err)
	}
	
	// 读取目录中的所有文件
	files, err := os.ReadDir(cm.novelDir)
	if err != nil {
		return 0, fmt.Errorf("读取小说目录失败: %w", err)
	}
	
	maxIndex := 0
	chapterPattern := regexp.MustCompile(`^example_chapter_(\d+)\.json$`)
	
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		
		matches := chapterPattern.FindStringSubmatch(file.Name())
		if len(matches) == 2 {
			index, err := strconv.Atoi(matches[1])
			if err == nil && index > maxIndex {
				maxIndex = index
			}
		}
	}
	
	return maxIndex + 1, nil
}