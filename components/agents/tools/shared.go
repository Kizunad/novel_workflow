package tools

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/bytedance/sonic"
)

// ToolResponse 统一的工具响应结构
type ToolResponse struct {
	Status  string      `json:"status"`
	Error   string      `json:"error,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// ResponseStatus 响应状态常量
const (
	StatusSuccess = "success"
	StatusError   = "error"
)

// BuildSuccessResponse 构建成功响应
func BuildSuccessResponse(data interface{}, message ...string) (string, error) {
	response := &ToolResponse{
		Status: StatusSuccess,
		Data:   data,
	}

	if len(message) > 0 && message[0] != "" {
		response.Message = message[0]
	}

	return marshalResponse(response)
}

// BuildErrorResponse 构建错误响应
func BuildErrorResponse(err error, message ...string) (string, error) {
	response := &ToolResponse{
		Status: StatusError,
		Error:  err.Error(),
	}

	if len(message) > 0 && message[0] != "" {
		response.Message = message[0]
	}

	// 错误响应总是成功序列化，不返回序列化错误
	responseJSON, _ := marshalResponse(response)
	return responseJSON, nil
}

// SafeParseJSON 安全解析JSON，提供详细错误信息
func SafeParseJSON(jsonStr string, target interface{}) error {
	if strings.TrimSpace(jsonStr) == "" {
		return fmt.Errorf("empty JSON string")
	}

	if err := sonic.Unmarshal([]byte(jsonStr), target); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	return nil
}

// ValidateFilePath 验证文件路径安全性，防止路径遍历攻击
func ValidateFilePath(basePath, userPath string) error {
	if userPath == "" {
		return fmt.Errorf("file path cannot be empty")
	}

	// 清理路径，移除 . 和 .. 等相对路径元素
	cleanPath := filepath.Clean(userPath)

	// 检查是否包含危险的路径遍历模式
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path traversal attempt detected: %s", userPath)
	}

	// 如果提供了基础路径，确保最终路径在基础路径内
	if basePath != "" {
		fullPath := filepath.Join(basePath, cleanPath)
		absBasePath, err := filepath.Abs(basePath)
		if err != nil {
			return fmt.Errorf("invalid base path: %w", err)
		}

		absFullPath, err := filepath.Abs(fullPath)
		if err != nil {
			return fmt.Errorf("invalid file path: %w", err)
		}

		if !strings.HasPrefix(absFullPath, absBasePath) {
			return fmt.Errorf("path outside base directory: %s", userPath)
		}
	}

	return nil
}

// ValidateStringParam 验证字符串参数
func ValidateStringParam(param, paramName string, required bool) error {
	trimmed := strings.TrimSpace(param)

	if required && trimmed == "" {
		return fmt.Errorf("%s is required and cannot be empty", paramName)
	}

	// 检查常见的注入攻击模式
	dangerousPatterns := []string{
		"../", "..\\",
		"<script", "</script>",
		"javascript:", "data:",
		"${", "#{",
	}

	lowerParam := strings.ToLower(trimmed)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(lowerParam, pattern) {
			return fmt.Errorf("potentially dangerous content detected in %s", paramName)
		}
	}

	return nil
}

// ValidateIntParam 验证整数参数
func ValidateIntParam(param int, paramName string, min, max int) error {
	if param < min {
		return fmt.Errorf("%s must be at least %d, got %d", paramName, min, param)
	}

	if max > 0 && param > max {
		return fmt.Errorf("%s must be at most %d, got %d", paramName, max, param)
	}

	return nil
}

// marshalResponse 私有函数，统一序列化响应
func marshalResponse(response *ToolResponse) (string, error) {
	responseJSON, err := sonic.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal response: %w", err)
	}

	return string(responseJSON), nil
}

// NovelChapter 表示小说章节结构
type NovelChapter struct {
	ChapterID string     `json:"chapter_id"`
	Title     string     `json:"title"`
	Content   []Paragraph `json:"content"`
}

// Paragraph 表示段落结构
type Paragraph struct {
	ParagraphID int    `json:"paragraph_id"`
	Text        string `json:"text"`
}

// ParseNovelChapter 解析小说章节JSON，返回标题+内容文本
func ParseNovelChapter(jsonStr string) (string, error) {
	var chapter NovelChapter
	if err := SafeParseJSON(jsonStr, &chapter); err != nil {
		return "", fmt.Errorf("failed to parse novel chapter: %w", err)
	}

	if chapter.Title == "" {
		return "", fmt.Errorf("chapter title is empty")
	}

	var contentBuilder strings.Builder
	for _, paragraph := range chapter.Content {
		if paragraph.Text != "" {
			contentBuilder.WriteString(paragraph.Text)
			contentBuilder.WriteString("\n\n")
		}
	}

	result := chapter.Title + "\n\n" + strings.TrimSpace(contentBuilder.String())
	return result, nil
}
