package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/Kizunad/modular-workflow-v2/providers"
	"github.com/Kizunad/modular-workflow-v2/components/content/managers"
)

// SummaryCRUDTool 摘要增删改查工具，基于summarizer/processor.go重构
type SummaryCRUDTool struct {
	novelDir   string
	llmManager *providers.Manager
	model      string
}

// NewSummaryCRUDTool 创建摘要CRUD工具
func NewSummaryCRUDTool(novelDir string, llmManager *providers.Manager) *SummaryCRUDTool {
	return &SummaryCRUDTool{
		novelDir:   novelDir,
		llmManager: llmManager,
		model:      "qwen3:4b", // 使用 Ollama 本地模型
	}
}

// Info 实现BaseTool接口
func (t *SummaryCRUDTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "summary_crud",
		Desc: "章节摘要管理工具，用于创建、读取、更新章节摘要。支持AI智能提取章节关键信息并生成结构化摘要",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action": {
				Type:     schema.String,
				Desc:     "操作类型: create/read/update/extract_info",
				Required: true,
			},
			"chapter_content": {
				Type:     schema.String,
				Desc:     "章节内容",
				Required: false,
			},
			"chapter_id": {
				Type:     schema.String,
				Desc:     "章节ID",
				Required: false,
			},
			"summary_content": {
				Type:     schema.String,
				Desc:     "摘要内容",
				Required: false,
			},
		}),
	}, nil
}

// InvokableRun 实现InvokableTool接口
func (t *SummaryCRUDTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// 解析输入参数
	var input map[string]interface{}
	if err := sonic.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", compose.NewInterruptAndRerunErr("JSON参数解析失败，请检查格式并重新调用。原始参数: " + argumentsInJSON + "，错误: " + err.Error())
	}

	// 调用内部方法处理
	return t.invoke(ctx, input)
}

// invoke 内部调用方法
func (t *SummaryCRUDTool) invoke(ctx context.Context, input map[string]any) (string, error) {
	action, ok := input["action"].(string)
	if !ok {
		return "", compose.NewInterruptAndRerunErr("缺少必要参数 action（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	switch action {
	case "create":
		return t.handleCreate(ctx, input)
	case "read":
		return t.handleRead(input)
	case "update":
		return t.handleUpdate(input)
	case "extract_info":
		return t.handleExtractInfo(input)
	default:
		return "", compose.NewInterruptAndRerunErr("不支持的操作类型: " + action + "，支持的操作: create/read/update/extract_info，当前参数: " + fmt.Sprintf("%v", input))
	}
}

// handleCreate 处理创建摘要
func (t *SummaryCRUDTool) handleCreate(ctx context.Context, input map[string]any) (string, error) {
	chapterContent, contentOk := input["chapter_content"].(string)
	if !contentOk || chapterContent == "" {
		return "", compose.NewInterruptAndRerunErr("创建摘要需要提供有效的 chapter_content 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	// 提取章节信息
	chapterInfo, err := t.extractChapterInfo(chapterContent)
	if err != nil {
		return "", fmt.Errorf("提取章节信息失败: %w", err)
	}

	// 获取 Ollama 模型
	model, err := t.llmManager.GetOllamaModel(ctx, providers.WithModel(t.model))
	if err != nil {
		return "", fmt.Errorf("获取 Ollama 模型失败: %w", err)
	}

	// 生成摘要
	summary, err := t.generateSummary(ctx, model, chapterContent, chapterInfo)
	if err != nil {
		return "", fmt.Errorf("生成摘要失败: %w", err)
	}

	// 更新索引
	indexManager := managers.NewIndexManager(t.novelDir)
	if err := indexManager.UpdateSummary(*summary); err != nil {
		return "", fmt.Errorf("更新索引失败: %w", err)
	}

	return t.successResponse("摘要创建成功", summary), nil
}

// handleRead 处理读取摘要
func (t *SummaryCRUDTool) handleRead(input map[string]any) (string, error) {
	chapterID, idOk := input["chapter_id"].(string)
	if !idOk || chapterID == "" {
		return "", compose.NewInterruptAndRerunErr("读取摘要需要提供有效的 chapter_id 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	// 从索引管理器读取摘要
	indexManager := managers.NewIndexManager(t.novelDir)

	// 这里需要实现从索引中读取指定章节摘要的逻辑
	// 由于原始IndexManager可能不支持按ID读取，我们返回一个占位实现
	result := map[string]interface{}{
		"chapter_id": chapterID,
		"message":    "读取功能需要扩展IndexManager实现",
		"index_path": indexManager.GetIndexPath(),
	}

	return t.successResponse("摘要读取请求处理完成", result), nil
}

// handleUpdate 处理更新摘要
func (t *SummaryCRUDTool) handleUpdate(input map[string]any) (string, error) {
	chapterID, idOk := input["chapter_id"].(string)
	if !idOk || chapterID == "" {
		return "", compose.NewInterruptAndRerunErr("更新摘要需要提供有效的 chapter_id 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	summaryContent, contentOk := input["summary_content"].(string)
	if !contentOk || summaryContent == "" {
		return "", compose.NewInterruptAndRerunErr("更新摘要需要提供有效的 summary_content 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	// 创建更新的摘要对象
	summary := &managers.ChapterSummary{
		ChapterID: chapterID,
		Title:     fmt.Sprintf("章节 %s", chapterID),
		Summary:   summaryContent,
		WordCount: len(summaryContent),
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// 更新索引
	indexManager := managers.NewIndexManager(t.novelDir)
	if err := indexManager.UpdateSummary(*summary); err != nil {
		return "", fmt.Errorf("更新索引失败: %w", err)
	}

	return t.successResponse("摘要更新成功", summary), nil
}

// handleExtractInfo 处理提取章节信息
func (t *SummaryCRUDTool) handleExtractInfo(input map[string]any) (string, error) {
	chapterContent, contentOk := input["chapter_content"].(string)
	if !contentOk || chapterContent == "" {
		return "", compose.NewInterruptAndRerunErr("提取章节信息需要提供有效的 chapter_content 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	// 提取章节信息
	chapterInfo, err := t.extractChapterInfo(chapterContent)
	if err != nil {
		return "", fmt.Errorf("提取章节信息失败: %w", err)
	}

	return t.successResponse("章节信息提取成功", chapterInfo), nil
}

// SummaryChapterInfo 摘要章节信息
type SummaryChapterInfo struct {
	ChapterID string `json:"chapter_id"`
	Title     string `json:"title"`
	WordCount int    `json:"word_count"`
}

// extractChapterInfo 从内容中提取章节信息
func (t *SummaryCRUDTool) extractChapterInfo(content string) (*SummaryChapterInfo, error) {
	// 简单的章节信息提取逻辑
	lines := strings.Split(content, "\n")

	info := &SummaryChapterInfo{
		WordCount: len(strings.ReplaceAll(content, " ", "")), // 简单字符数统计
	}

	// 尝试从第一行提取标题
	if len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		if strings.HasPrefix(firstLine, "===") && strings.HasSuffix(firstLine, "===") {
			info.Title = strings.TrimSpace(strings.Trim(firstLine, "="))
		} else {
			info.Title = "未知章节"
		}
	}

	// 直接使用title作为章节ID，或生成基于时间的ID
	if info.Title != "" && info.Title != "未知章节" {
		info.ChapterID = info.Title
	} else {
		info.ChapterID = fmt.Sprintf("auto_%d", time.Now().Unix())
	}

	return info, nil
}

// generateSummary 生成章节摘要
func (t *SummaryCRUDTool) generateSummary(ctx context.Context, model model.ToolCallingChatModel, content string, info *SummaryChapterInfo) (*managers.ChapterSummary, error) {
	// 创建摘要生成模板
	template := prompt.FromMessages(
		schema.Jinja2,
		schema.SystemMessage(`你是一个专业的小说内容分析师。请仔细阅读提供的章节内容，提取关键信息并生成简洁摘要。

要求：
1. 提取2-3个关键事件
2. 识别重要角色
3. 主要地点
4. 情节要点
5. 保持客观简洁

请按照以下格式返回摘要（每行一个要素，用冒号分隔）：
关键事件: 事件1；事件2；事件3
主要角色: 角色1, 角色2, 角色3  
重要地点: 地点1, 地点2
情节进展: 简述本章推进的主要情节`),
		schema.UserMessage("章节内容：\n{{content}}\n\n请分析并生成摘要："),
	)

	// 构建输入
	input := map[string]any{
		"content": content,
	}

	// 格式化提示
	messages, err := template.Format(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("格式化提示失败: %w", err)
	}

	// 调用模型生成摘要
	response, err := model.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("模型生成失败: %w", err)
	}

	// 清理响应内容，去除思考标签和其他无关内容
	summaryContent := t.cleanSummaryContent(response.Content)
	if summaryContent == "" {
		summaryContent = "内容更新"
	}

	// 构建最终摘要
	summary := &managers.ChapterSummary{
		ChapterID: info.ChapterID,
		Title:     info.Title,
		Summary:   summaryContent,
		WordCount: info.WordCount,
		Timestamp: time.Now().Format(time.RFC3339),
	}

	return summary, nil
}

// cleanSummaryContent 清理摘要内容，去除思考标签和无关内容
func (t *SummaryCRUDTool) cleanSummaryContent(content string) string {
	// 使用正则表达式去除 <think> 标签及其内容
	thinkPattern := regexp.MustCompile(`(?s)<think>.*?</think>`)
	cleaned := thinkPattern.ReplaceAllString(content, "")

	// 去除单独的 <think> 或 </think> 标签
	cleaned = regexp.MustCompile(`</?think>`).ReplaceAllString(cleaned, "")

	// 去除其他可能的XML标签
	xmlPattern := regexp.MustCompile(`<[^>]*>`)
	cleaned = xmlPattern.ReplaceAllString(cleaned, "")

	// 清理多余的空白字符
	cleaned = regexp.MustCompile(`\s+`).ReplaceAllString(cleaned, " ")

	return strings.TrimSpace(cleaned)
}

// successResponse 构建成功响应
func (t *SummaryCRUDTool) successResponse(message string, data interface{}) string {
	response := map[string]interface{}{
		"success": true,
		"message": message,
		"data":    data,
	}

	jsonBytes, _ := sonic.MarshalIndent(response, "", "  ")
	return string(jsonBytes)
}
