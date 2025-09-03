package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/Kizunad/modular-workflow-v2/config"
)

// ChapterIDInput 章节ID输入参数
type ChapterIDInput struct {
	ChapterID string `json:"chapter_id" jsonschema:"required,description=章节ID标识符"`
}

// NovelReadChapterTool 小说章节读取工具
type NovelReadChapterTool struct{}

// NewNovelReadChapterTool 创建新的章节读取工具
func NewNovelReadChapterTool() *NovelReadChapterTool {
	return &NovelReadChapterTool{}
}

// Info 工具信息描述
func (n *NovelReadChapterTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "novel_read_chapter",
		Desc: "根据章节ID读取指定章节的完整内容",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"chapter_id": {
				Type:     schema.String,
				Desc:     "章节ID，例如：001、002或example_chapter_1.json",
				Required: true,
			},
		}),
	}, nil
}

// InvokableRun 执行章节读取操作
func (n *NovelReadChapterTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	start := time.Now()

	// 解析输入参数
	var input ChapterIDInput
	if err := SafeParseJSON(argumentsInJSON, &input); err != nil {
		return BuildErrorResponse(fmt.Errorf("failed to parse arguments: %w", err))
	}

	if err := ValidateStringParam(input.ChapterID, "chapter_id", true); err != nil {
		return BuildErrorResponse(err)
	}

	// 从全局配置获取小说路径
	globalConfig := config.GetGlobal()
	novelPath, err := globalConfig.Novel.GetAbsolutePath()
	if err != nil {
		return BuildErrorResponse(fmt.Errorf("获取小说路径失败: %w", err))
	}
	
	// 章节文件命名模式
	const chapter_naming_pattern = "example_chapter_%s.json"
	
	// 根据章节ID构建文件路径
	filename := fmt.Sprintf(chapter_naming_pattern, input.ChapterID)
	filePath := filepath.Join(novelPath, filename)

	// 验证文件路径
	if err := ValidateFilePath("", filePath); err != nil {
		return BuildErrorResponse(fmt.Errorf("invalid file path: %w", err))
	}

	// 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		return BuildErrorResponse(fmt.Errorf("failed to read chapter file %s: %w", filePath, err))
	}

	// 解析章节内容
	result, err := ParseNovelChapter(string(content))
	if err != nil {
		return BuildErrorResponse(fmt.Errorf("failed to parse chapter: %w", err))
	}

	elapsed := time.Since(start)

	// 构建返回结果
	data := map[string]interface{}{
		"chapter_id":      input.ChapterID,
		"file_path":       filePath,
		"content":         result,
		"process_time_ms": elapsed.Milliseconds(),
	}

	return BuildSuccessResponse(data)
}
