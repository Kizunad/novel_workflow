package managers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// ChapterManager 章节管理器（兼容性包装）
// 这是一个临时的兼容层，用于支持现有的章节计数功能
// 完整的ChapterManager重构将在下一阶段进行
type ChapterManager struct {
	novelDir string
}

// NewChapterManager 创建章节管理器
func NewChapterManager(novelDir string) *ChapterManager {
	return &ChapterManager{
		novelDir: novelDir,
	}
}

// GetChapterCount 获取章节数量（简化实现）
func (cm *ChapterManager) GetChapterCount() int {
	count, _ := cm.countChapterFiles()
	return count
}

// countChapterFiles 计算章节文件数量
func (cm *ChapterManager) countChapterFiles() (int, error) {
	// 检查目录是否存在
	if _, err := os.Stat(cm.novelDir); os.IsNotExist(err) {
		return 0, nil
	}
	
	// 读取目录中的文件
	files, err := os.ReadDir(cm.novelDir)
	if err != nil {
		return 0, err
	}
	
	// 章节文件名模式：example_chapter_N.json
	chapterPattern := regexp.MustCompile(`^example_chapter_(\d+)\.json$`)
	maxChapterNum := 0
	
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		
		matches := chapterPattern.FindStringSubmatch(file.Name())
		if len(matches) == 2 {
			if num, err := strconv.Atoi(matches[1]); err == nil {
				if num > maxChapterNum {
					maxChapterNum = num
				}
			}
		}
	}
	
	return maxChapterNum, nil
}

// HasChapters 检查是否有章节文件
func (cm *ChapterManager) HasChapters() bool {
	count := cm.GetChapterCount()
	return count > 0
}

// GetChapterFiles 获取所有章节文件名
func (cm *ChapterManager) GetChapterFiles() []string {
	files, err := os.ReadDir(cm.novelDir)
	if err != nil {
		return []string{}
	}
	
	chapterPattern := regexp.MustCompile(`^example_chapter_\d+\.json$`)
	var chapterFiles []string
	
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		
		if chapterPattern.MatchString(file.Name()) {
			chapterFiles = append(chapterFiles, file.Name())
		}
	}
	
	return chapterFiles
}

// GetLatestChapterPath 获取最新章节文件路径
func (cm *ChapterManager) GetLatestChapterPath() string {
	count := cm.GetChapterCount()
	if count == 0 {
		return ""
	}
	
	fileName := "example_chapter_" + strconv.Itoa(count) + ".json"
	return filepath.Join(cm.novelDir, fileName)
}

// GetChapterPath 获取指定编号的章节文件路径
func (cm *ChapterManager) GetChapterPath(chapterNum int) string {
	if chapterNum <= 0 {
		return ""
	}
	
	fileName := "example_chapter_" + strconv.Itoa(chapterNum) + ".json"
	return filepath.Join(cm.novelDir, fileName)
}

// ValidateChapterStructure 验证章节目录结构
func (cm *ChapterManager) ValidateChapterStructure() error {
	if _, err := os.Stat(cm.novelDir); os.IsNotExist(err) {
		return os.MkdirAll(cm.novelDir, 0755)
	}
	
	return nil
}

// ChapterData 章节数据结构
type ChapterData struct {
	ChapterID string `json:"chapter_id"`
	Title     string `json:"title"`
	Content   []struct {
		ParagraphID int    `json:"paragraph_id"`
		Text        string `json:"text"`
	} `json:"content"`
}

// GetLatestChapterContent 获取最新章节内容
func (cm *ChapterManager) GetLatestChapterContent() (string, error) {
	latestPath := cm.GetLatestChapterPath()
	if latestPath == "" {
		return "", nil // 没有章节文件
	}

	// 读取JSON文件
	data, err := os.ReadFile(latestPath)
	if err != nil {
		return "", err
	}

	// 解析JSON
	var chapter ChapterData
	if err := json.Unmarshal(data, &chapter); err != nil {
		return "", err
	}

	// 拼接段落内容
	var contentBuilder strings.Builder
	for i, paragraph := range chapter.Content {
		contentBuilder.WriteString(paragraph.Text)
		// 段落间添加换行，但最后一个段落不添加
		if i < len(chapter.Content)-1 {
			contentBuilder.WriteString("\n\n")
		}
	}

	return contentBuilder.String(), nil
}

// GetChapterContent 获取指定章节内容
func (cm *ChapterManager) GetChapterContent(chapterNum int) (string, error) {
	chapterPath := cm.GetChapterPath(chapterNum)
	if chapterPath == "" {
		return "", nil
	}

	// 读取JSON文件
	data, err := os.ReadFile(chapterPath)
	if err != nil {
		return "", err
	}

	// 解析JSON
	var chapter ChapterData
	if err := json.Unmarshal(data, &chapter); err != nil {
		return "", err
	}

	// 拼接段落内容
	var contentBuilder strings.Builder
	for i, paragraph := range chapter.Content {
		contentBuilder.WriteString(paragraph.Text)
		if i < len(chapter.Content)-1 {
			contentBuilder.WriteString("\n\n")
		}
	}

	return contentBuilder.String(), nil
}

// GetChapterMetadata 获取章节元数据
func (cm *ChapterManager) GetChapterMetadata() map[string]interface{} {
	metadata := map[string]interface{}{
		"novel_dir":      cm.novelDir,
		"chapter_count":  cm.GetChapterCount(),
		"has_chapters":   cm.HasChapters(),
		"chapter_files":  cm.GetChapterFiles(),
	}
	
	if latestPath := cm.GetLatestChapterPath(); latestPath != "" {
		if stat, err := os.Stat(latestPath); err == nil {
			metadata["latest_chapter"] = map[string]interface{}{
				"path":     latestPath,
				"size":     stat.Size(),
				"mod_time": stat.ModTime(),
			}
		}
	}
	
	return metadata
}