package managers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Kizunad/modular-workflow-v2/components/content/token"
	content "github.com/Kizunad/modular-workflow-v2/components/content/utils"
)

// BaseFileManager 基础文件管理器实现
type BaseFileManager struct {
	filePath     string
	current      string
	lastModTime  time.Time
	tokenCounter token.TokenCounter
	tokenBudget  *token.TokenBudgetManager
}

// NewBaseFileManager 创建基础文件管理器
func NewBaseFileManager(filePath string) *BaseFileManager {
	// 确保父目录存在
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		// 忽略错误，在实际操作时再处理
	}
	
	return &BaseFileManager{
		filePath:     filePath,
		tokenCounter: token.NewSimpleTokenCounter(),
	}
}

// Load 加载文件内容
func (bfm *BaseFileManager) Load() (string, error) {
	if !bfm.Exists() {
		return "", content.NewFileNotFoundError(bfm.filePath, nil)
	}
	
	data, err := os.ReadFile(bfm.filePath)
	if err != nil {
		return "", content.NewFileReadError(bfm.filePath, err)
	}
	
	content := strings.TrimSpace(string(data))
	bfm.current = content
	
	// 更新修改时间
	if stat, err := os.Stat(bfm.filePath); err == nil {
		bfm.lastModTime = stat.ModTime()
	}
	
	return content, nil
}

// Save 保存文件内容
func (bfm *BaseFileManager) Save(content string) error {
	// 确保父目录存在
	dir := filepath.Dir(bfm.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to write file %s: %w", bfm.filePath, err)
	}
	
	// 写入文件
	err := os.WriteFile(bfm.filePath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", bfm.filePath, err)
	}
	
	// 更新内部状态
	bfm.current = content
	if stat, err := os.Stat(bfm.filePath); err == nil {
		bfm.lastModTime = stat.ModTime()
	}
	
	return nil
}

// Exists 检查文件是否存在
func (bfm *BaseFileManager) Exists() bool {
	_, err := os.Stat(bfm.filePath)
	return err == nil
}

// GetPath 获取文件路径
func (bfm *BaseFileManager) GetPath() string {
	return bfm.filePath
}

// GetTokenCount 获取当前内容的Token数量
func (bfm *BaseFileManager) GetTokenCount() int {
	if bfm.current == "" {
		// 如果当前内容为空，尝试加载
		if content, err := bfm.Load(); err == nil {
			return bfm.tokenCounter.Count(content)
		}
		return 0
	}
	return bfm.tokenCounter.Count(bfm.current)
}

// EstimateTokens 估算Token数量
func (bfm *BaseFileManager) EstimateTokens() int {
	if bfm.current == "" {
		if content, err := bfm.Load(); err == nil {
			return bfm.tokenCounter.EstimateTokens(content)
		}
		return 0
	}
	return bfm.tokenCounter.EstimateTokens(bfm.current)
}

// GetModTime 获取文件修改时间
func (bfm *BaseFileManager) GetModTime() (time.Time, error) {
	stat, err := os.Stat(bfm.filePath)
	if err != nil {
		return time.Time{}, content.NewFileNotFoundError(bfm.filePath, err)
	}
	return stat.ModTime(), nil
}

// IsModified 检查文件是否被修改
func (bfm *BaseFileManager) IsModified() bool {
	modTime, err := bfm.GetModTime()
	if err != nil {
		return false
	}
	return modTime.After(bfm.lastModTime)
}

// GetCurrent 获取当前内容
func (bfm *BaseFileManager) GetCurrent() string {
	// 如果文件被修改，重新加载
	if bfm.IsModified() || bfm.current == "" {
		if content, err := bfm.Load(); err == nil {
			return content
		}
	}
	return bfm.current
}

// Update 更新内容
func (bfm *BaseFileManager) Update(content string) error {
	return bfm.Save(content)
}

// SetTokenBudget 设置Token预算管理器
func (bfm *BaseFileManager) SetTokenBudget(budget *token.TokenBudgetManager) {
	bfm.tokenBudget = budget
}

// GetTokenBudget 获取Token预算管理器
func (bfm *BaseFileManager) GetTokenBudget() *token.TokenBudgetManager {
	return bfm.tokenBudget
}

// TruncateToLimit 截断内容到指定Token限制
func (bfm *BaseFileManager) TruncateToLimit(text string, limit int) (string, int) {
	if bfm.tokenBudget != nil {
		return bfm.tokenBudget.TruncateToTokenLimit(text, "default")
	}
	
	// 简单的截断逻辑
	currentTokens := bfm.tokenCounter.Count(text)
	if currentTokens <= limit {
		return text, currentTokens
	}
	
	return bfm.simpleTokenTruncate(text, limit)
}

// simpleTokenTruncate 简单的Token截断
func (bfm *BaseFileManager) simpleTokenTruncate(text string, limit int) (string, int) {
	if limit <= 0 {
		return "", 0
	}
	
	// 先尝试按行截断
	lines := strings.Split(text, "\n")
	var result []string
	totalTokens := 0
	
	for _, line := range lines {
		lineTokens := bfm.tokenCounter.Count(line)
		if totalTokens+lineTokens > limit {
			// 如果当前行超出限制，尝试按词截断
			if totalTokens < limit {
				remainingTokens := limit - totalTokens
				truncatedLine := bfm.truncateLineByWords(line, remainingTokens)
				if truncatedLine != "" {
					result = append(result, truncatedLine)
					totalTokens += bfm.tokenCounter.Count(truncatedLine)
				}
			}
			break
		}
		result = append(result, line)
		totalTokens += lineTokens
	}
	
	finalText := strings.Join(result, "\n")
	return finalText, totalTokens
}

// truncateLineByWords 按词截断单行文本
func (bfm *BaseFileManager) truncateLineByWords(line string, limit int) string {
	if limit <= 0 {
		return ""
	}
	
	// 先尝试按空格分词
	words := strings.Fields(line)
	if len(words) > 1 {
		var result []string
		totalTokens := 0
		
		for _, word := range words {
			wordTokens := bfm.tokenCounter.Count(word)
			if totalTokens+wordTokens > limit {
				break
			}
			result = append(result, word)
			totalTokens += wordTokens
		}
		
		if len(result) > 0 {
			return strings.Join(result, " ")
		}
	}
	
	// 如果只有一个词或没有成功分词，尝试按字符截断
	return bfm.truncateLineByCharacters(line, limit)
}

// truncateLineByCharacters 按字符截断单行文本
func (bfm *BaseFileManager) truncateLineByCharacters(line string, limit int) string {
	if limit <= 0 {
		return ""
	}
	
	runes := []rune(line)
	
	// 二分搜索找到合适的截断点
	left, right := 0, len(runes)
	bestEnd := 0
	
	for left <= right {
		mid := (left + right) / 2
		if mid == 0 {
			left = mid + 1
			continue
		}
		
		candidate := string(runes[:mid])
		tokenCount := bfm.tokenCounter.Count(candidate)
		
		if tokenCount <= limit {
			bestEnd = mid
			left = mid + 1
		} else {
			right = mid - 1
		}
	}
	
	if bestEnd == 0 {
		return ""
	}
	
	return string(runes[:bestEnd])
}

// GetFileInfo 获取文件信息
func (bfm *BaseFileManager) GetFileInfo() *FileInfo {
	return GetFileInfo(bfm.filePath)
}

// EnsureFileExists 确保文件存在（如果不存在则创建空文件）
func (bfm *BaseFileManager) EnsureFileExists() error {
	if bfm.Exists() {
		return nil
	}
	
	return bfm.Save("")
}

// BackupFile 备份文件
func (bfm *BaseFileManager) BackupFile() error {
	if !bfm.Exists() {
		return content.NewFileNotFoundError(bfm.filePath, nil)
	}
	
	backupPath := bfm.filePath + ".backup." + time.Now().Format("20060102150405")
	
	data, err := os.ReadFile(bfm.filePath)
	if err != nil {
		return content.NewFileReadError(bfm.filePath, err)
	}
	
	err = os.WriteFile(backupPath, data, 0644)
	if err != nil {
		return content.NewFileWriteError(backupPath, err)
	}
	
	return nil
}

// ValidateContent 验证内容
func (bfm *BaseFileManager) ValidateContent(content string) error {
	// 基础验证：检查内容是否过大
	if bfm.tokenBudget != nil {
		tokenCount := bfm.tokenCounter.Count(content)
		// 这里可以添加更多验证逻辑
		_ = tokenCount
	}
	
	return nil
}