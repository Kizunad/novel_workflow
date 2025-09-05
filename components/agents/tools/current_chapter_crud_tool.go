package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/Kizunad/modular-workflow-v2/components/content/managers"
)

// CurrentChapterCRUDTool 当前章节增删改查工具
type CurrentChapterCRUDTool struct {
	novelDir string
}

// NewCurrentChapterCRUDTool 创建当前章节CRUD工具
func NewCurrentChapterCRUDTool(novelDir string) *CurrentChapterCRUDTool {
	return &CurrentChapterCRUDTool{
		novelDir: novelDir,
	}
}

// Info 工具信息描述
func (t *CurrentChapterCRUDTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "current_chapter_crud",
		Desc: "当前章节管理工具：增删改查当前正在编写的章节。支持创建、读取、更新章节内容，以及获取章节状态。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action": {
				Type:     schema.String,
				Desc:     "操作类型：create, read, update, get_latest, list, count",
				Required: true,
			},
			"chapter_id": {
				Type:     schema.String,
				Desc:     "章节ID（如：001、002）- 仅用于读取操作，创建时由系统自动递增生成",
				Required: false,
			},
			"title": {
				Type:     schema.String,
				Desc:     "章节标题",
				Required: false,
			},
			"content": {
				Type:     schema.String,
				Desc:     "章节内容",
				Required: false,
			},
			"limit": {
				Type:     schema.Integer,
				Desc:     "限制返回数量（用于列表操作）",
				Required: false,
			},
		}),
	}, nil
}

// InvokableRun 执行工具操作
func (t *CurrentChapterCRUDTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// 解析输入参数
	var input map[string]interface{}
	if err := sonic.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", compose.NewInterruptAndRerunErr("JSON参数解析失败，请检查格式并重新调用。原始参数: " + argumentsInJSON + "，错误: " + err.Error())
	}

	return t.invoke(ctx, input)
}

// 章节信息结构体
type ChapterInfo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	WordCount int    `json:"word_count"`
	Path      string `json:"path"`
}

// 响应结构体定义
type ChapterResponse struct {
	Success  bool          `json:"success"`
	Message  string        `json:"message"`
	Data     *ChapterInfo  `json:"data,omitempty"`
	Chapters []ChapterInfo `json:"chapters,omitempty"`
	Count    int           `json:"count,omitempty"`
}

// invoke 工具执行逻辑
func (t *CurrentChapterCRUDTool) invoke(_ context.Context, input map[string]any) (string, error) {
	// 解析操作类型
	action, ok := input["action"].(string)
	if !ok {
		return "", compose.NewInterruptAndRerunErr("缺少操作类型参数 action（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	switch action {
	case "create":
		return t.handleCreate(input)
	case "read":
		return t.handleRead(input)
	case "update":
		return t.handleUpdate(input)
	case "get_latest":
		return t.handleGetLatest()
	case "list":
		return t.handleList(input)
	case "count":
		return t.handleCount()
	default:
		return "", compose.NewInterruptAndRerunErr("未知操作类型: " + action + "，支持的操作: create/read/update/get_latest/list/count，当前参数: " + fmt.Sprintf("%v", input))
	}
}

// handleCreate 处理创建章节
func (t *CurrentChapterCRUDTool) handleCreate(input map[string]any) (string, error) {
	title, titleOk := input["title"].(string)
	if !titleOk || title == "" {
		return "", compose.NewInterruptAndRerunErr("创建章节需要提供有效的 title 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	contentStr, contentOk := input["content"].(string)
	if !contentOk || contentStr == "" {
		return "", compose.NewInterruptAndRerunErr("创建章节需要提供有效的 content 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	// 创建章节管理器
	chapterManager := managers.NewChapterManager(t.novelDir)

	// 获取下一个章节编号（写入前）
	nextChapterNum := chapterManager.GetChapterCount() + 1

	// 实际写入章节文件
	chapterPath, err := chapterManager.WriteChapter(title, contentStr)
	if err != nil {
		return "", err
	}

	// 使用预先确定的章节ID
	chapterID := fmt.Sprintf("%03d", nextChapterNum)

	// 构建响应数据
	info := &ChapterInfo{
		ID:        chapterID,
		Title:     title,
		Content:   contentStr,
		WordCount: len(contentStr),
		Path:      chapterPath,
	}

	return t.successResponse("章节创建成功", info, nil, 0), nil
}

// handleRead 处理读取章节
func (t *CurrentChapterCRUDTool) handleRead(input map[string]any) (string, error) {
	chapterID, chapterIDOk := input["chapter_id"].(string)
	if !chapterIDOk || chapterID == "" {
		return "", compose.NewInterruptAndRerunErr("读取章节需要提供有效的 chapter_id 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	// 验证章节ID是否在有效范围内
	chapterManager := managers.NewChapterManager(t.novelDir)
	existingCount := chapterManager.GetChapterCount()

	// 解析章节ID为数字进行验证
	var chapterNum int
	if _, err := fmt.Sscanf(chapterID, "%d", &chapterNum); err != nil {
		return "", err
	}

	if chapterNum <= 0 || chapterNum > existingCount {
		return "", compose.NewInterruptAndRerunErr(fmt.Sprintf("读取章节ID超出范围，当前有效范围: 001-%03d，提供的chapter_id: %s，当前参数: %v", existingCount, chapterID, input))
	}

	// 使用已创建的章节管理器获取真实内容

	// 获取章节真实内容
	content, err := chapterManager.GetChapterContent(chapterNum)
	if err != nil {
		return "", err
	}

	// 构建真实的章节文件路径
	chapterPath := chapterManager.GetChapterPath(chapterNum)

	// 尝试读取章节的完整信息（包括标题）
	var title string
	if data, err := os.ReadFile(chapterPath); err == nil {
		var chapterData struct {
			ChapterID string `json:"chapter_id"`
			Title     string `json:"title"`
		}
		if sonic.Unmarshal(data, &chapterData) == nil && chapterData.Title != "" {
			title = chapterData.Title
		} else {
			title = fmt.Sprintf("第%s章", chapterID)
		}
	} else {
		title = fmt.Sprintf("第%s章", chapterID)
	}

	// 构建真实的响应数据
	info := &ChapterInfo{
		ID:        chapterID,
		Title:     title,
		Content:   content,
		WordCount: len(content),
		Path:      chapterPath,
	}

	return t.successResponse("章节读取成功", info, nil, 0), nil
}

// handleUpdate 处理更新章节
func (t *CurrentChapterCRUDTool) handleUpdate(input map[string]any) (string, error) {
	chapterID, chapterIDOk := input["chapter_id"].(string)
	if !chapterIDOk || chapterID == "" {
		return "", compose.NewInterruptAndRerunErr("更新章节需要提供有效的 chapter_id 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	// 验证章节ID是否在有效范围内（防止越界修改）
	chapterManager := managers.NewChapterManager(t.novelDir)
	existingCount := chapterManager.GetChapterCount()

	// 解析章节ID为数字进行验证
	var chapterNum int
	if _, err := fmt.Sscanf(chapterID, "%d", &chapterNum); err != nil {
		return "", compose.NewInterruptAndRerunErr("无效的章节ID格式，需要数字格式（如: 001, 002），提供的chapter_id: " + chapterID + "，当前参数: " + fmt.Sprintf("%v", input))
	}

	if chapterNum <= 0 || chapterNum > existingCount {
		return "", compose.NewInterruptAndRerunErr(fmt.Sprintf("更新章节ID超出范围，当前有效范围: 001-%03d，提供的chapter_id: %s，当前参数: %v", existingCount, chapterID, input))
	}

	// 使用ChapterManager获取正确的章节路径
	chapterPath := chapterManager.GetChapterPath(chapterNum)

	// 使用已创建的章节管理器

	// 获取更新参数
	title, titleProvided := input["title"].(string)
	content, contentProvided := input["content"].(string)

	// 如果没有提供新内容，读取现有内容
	if !titleProvided || !contentProvided {
		existingContent, err := chapterManager.GetChapterContent(chapterNum)
		if err != nil {
			return "", err
		}

		// 读取现有标题
		if !titleProvided {
			if data, err := os.ReadFile(chapterPath); err == nil {
				var chapterData struct {
					Title string `json:"title"`
				}
				if sonic.Unmarshal(data, &chapterData) == nil && chapterData.Title != "" {
					title = chapterData.Title
				}
			}
		}

		if !contentProvided {
			content = existingContent
		}
	}

	// 验证必需参数
	if title == "" {
		title = fmt.Sprintf("第%s章", chapterID)
	}
	if content == "" {
		return "", compose.NewInterruptAndRerunErr("更新章节需要提供有效的内容，当前参数: " + fmt.Sprintf("%v", input))
	}

	// 执行真正的更新操作
	err := chapterManager.UpdateChapter(chapterNum, title, content)
	if err != nil {
		return "", err
	}

	// 获取更新后的文件路径
	updatedPath := chapterManager.GetChapterPath(chapterNum)

	// 构建真实的响应数据
	info := &ChapterInfo{
		ID:        chapterID,
		Title:     title,
		Content:   content,
		WordCount: len(content),
		Path:      updatedPath,
	}

	return t.successResponse("章节更新成功", info, nil, 0), nil
}

// handleGetLatest 处理获取最新章节
func (t *CurrentChapterCRUDTool) handleGetLatest() (string, error) {
	// 使用章节兼容管理器
	chapterManager := managers.NewChapterManager(t.novelDir)

	// 获取最新章节路径
	latestPath := chapterManager.GetLatestChapterPath()
	if latestPath == "" {
		return "", compose.NewInterruptAndRerunErr("没有找到任何章节，请先创建章节或检查小说目录是否正确: " + t.novelDir)
	}

	// 读取最新章节内容
	content, err := chapterManager.GetLatestChapterContent()
	if err != nil {
		return "", err
	}

	// 从路径提取章节ID
	baseName := filepath.Base(latestPath)
	chapterID := strings.TrimSuffix(baseName, filepath.Ext(baseName))

	// 读取真实的章节标题
	var title string
	if data, err := os.ReadFile(latestPath); err == nil {
		var chapterData struct {
			Title string `json:"title"`
		}
		if sonic.Unmarshal(data, &chapterData) == nil && chapterData.Title != "" {
			title = chapterData.Title
		} else {
			title = fmt.Sprintf("第%s章", chapterID)
		}
	} else {
		title = fmt.Sprintf("第%s章", chapterID)
	}

	// 构建响应数据
	info := &ChapterInfo{
		ID:        chapterID,
		Title:     title,
		Content:   content,
		WordCount: len(content),
		Path:      latestPath,
	}

	return t.successResponse("获取最新章节成功", info, nil, 0), nil
}

// handleList 处理列出章节
func (t *CurrentChapterCRUDTool) handleList(input map[string]any) (string, error) {
	limit := 10 // 默认限制
	if l, ok := input["limit"].(float64); ok && l > 0 && l <= 1000 { // 添加上限检查
		limit = int(l)
	}

	// 使用章节兼容管理器
	chapterManager := managers.NewChapterManager(t.novelDir)

	// 获取章节数量（简化实现）
	count := chapterManager.GetChapterCount()
	if count == 0 {
		return t.successResponse("当前没有章节", nil, []ChapterInfo{}, 0), nil
	}

	// 限制返回数量
	actualLimit := limit
	if actualLimit > count {
		actualLimit = count
	}

	// 构建真实的章节信息
	var chapterInfos []ChapterInfo
	for i := 1; i <= actualLimit; i++ {
		chapterID := fmt.Sprintf("%03d", i)

		// 获取真实的章节内容
		content, err := chapterManager.GetChapterContent(i)
		if err != nil {
			// 如果读取失败，跳过这个章节
			continue
		}

		// 获取真实的章节路径
		chapterPath := chapterManager.GetChapterPath(i)

		// 读取章节标题
		var title string
		if data, err := os.ReadFile(chapterPath); err == nil {
			var chapterData struct {
				Title string `json:"title"`
			}
			if sonic.Unmarshal(data, &chapterData) == nil && chapterData.Title != "" {
				title = chapterData.Title
			} else {
				title = fmt.Sprintf("第%s章", chapterID)
			}
		} else {
			title = fmt.Sprintf("第%s章", chapterID)
		}

		// 创建内容摘要（前100个字符）
		contentPreview := content
		if len(contentPreview) > 100 {
			contentPreview = contentPreview[:100] + "..."
		}

		info := ChapterInfo{
			ID:        chapterID,
			Title:     title,
			Content:   contentPreview,
			WordCount: len(content),
			Path:      chapterPath,
		}
		chapterInfos = append(chapterInfos, info)
	}

	return t.successResponse(fmt.Sprintf("找到%d个章节", len(chapterInfos)), nil, chapterInfos, count), nil
}

// handleCount 处理获取章节数量
func (t *CurrentChapterCRUDTool) handleCount() (string, error) {
	// 使用章节兼容管理器
	chapterManager := managers.NewChapterManager(t.novelDir)

	count := chapterManager.GetChapterCount()

	return t.successResponse(fmt.Sprintf("共有%d个章节", count), nil, nil, count), nil
}

// successResponse 创建成功响应
func (t *CurrentChapterCRUDTool) successResponse(message string, data *ChapterInfo, chapters []ChapterInfo, count int) string {
	response := ChapterResponse{
		Success:  true,
		Message:  message,
		Data:     data,
		Chapters: chapters,
		Count:    count,
	}

	result, _ := sonic.MarshalIndent(response, "", "  ")
	return string(result)
}
