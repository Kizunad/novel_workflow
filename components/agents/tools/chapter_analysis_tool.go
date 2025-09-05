package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/Kizunad/modular-workflow-v2/components/content/managers"
)

// ChapterAnalysisTool 通用章节分析工具，提供章节内容的通用分析功能
type ChapterAnalysisTool struct {
	novelDir string
}

// NewChapterAnalysisTool 创建章节分析工具
func NewChapterAnalysisTool(novelDir string) *ChapterAnalysisTool {
	return &ChapterAnalysisTool{
		novelDir: novelDir,
	}
}

// Info 实现BaseTool接口
func (t *ChapterAnalysisTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "chapter_analysis",
		Desc: "通用章节分析工具，提供章节内容提取、清理、结构分析等功能",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action": {
				Type:     schema.String,
				Desc:     "操作类型: extract_info/clean_content/analyze_structure/get_latest",
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
		}),
	}, nil
}

// InvokableRun 实现InvokableTool接口
func (t *ChapterAnalysisTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// 解析输入参数
	var input map[string]interface{}
	if err := sonic.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", compose.NewInterruptAndRerunErr("JSON参数解析失败，请检查格式并重新调用。原始参数: " + argumentsInJSON + "，错误: " + err.Error())
	}

	// 调用内部方法处理
	return t.invoke(ctx, input)
}

// invoke 内部调用方法
func (t *ChapterAnalysisTool) invoke(ctx context.Context, input map[string]any) (string, error) {
	action, ok := input["action"].(string)
	if !ok {
		return "", compose.NewInterruptAndRerunErr("缺少必要参数 action（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	switch action {
	case "extract_info":
		return t.handleExtractInfo(input)
	case "clean_content":
		return t.handleCleanContent(input)
	case "analyze_structure":
		return t.handleAnalyzeStructure(input)
	case "get_latest":
		return t.handleGetLatest()
	default:
		return "", compose.NewInterruptAndRerunErr("不支持的操作类型: " + action + "，支持的操作: extract_info/clean_content/analyze_structure/get_latest，当前参数: " + fmt.Sprintf("%v", input))
	}
}

// handleExtractInfo 处理提取章节基本信息
func (t *ChapterAnalysisTool) handleExtractInfo(input map[string]any) (string, error) {
	chapterContent, contentOk := input["chapter_content"].(string)
	if !contentOk || chapterContent == "" {
		return "", compose.NewInterruptAndRerunErr("提取章节信息需要提供有效的 chapter_content 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	// 提取章节信息
	info := t.extractChapterInfo(chapterContent)

	return t.successResponse("章节信息提取成功", info), nil
}

// handleCleanContent 处理清理章节内容
func (t *ChapterAnalysisTool) handleCleanContent(input map[string]any) (string, error) {
	chapterContent, contentOk := input["chapter_content"].(string)
	if !contentOk || chapterContent == "" {
		return "", compose.NewInterruptAndRerunErr("清理章节内容需要提供有效的 chapter_content 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	// 清理内容
	cleanedContent := t.cleanChapterContent(chapterContent)

	result := map[string]interface{}{
		"original_length": len(chapterContent),
		"cleaned_length":  len(cleanedContent),
		"cleaned_content": cleanedContent,
	}

	return t.successResponse("章节内容清理完成", result), nil
}

// handleAnalyzeStructure 处理分析章节结构
func (t *ChapterAnalysisTool) handleAnalyzeStructure(input map[string]any) (string, error) {
	chapterContent, contentOk := input["chapter_content"].(string)
	if !contentOk || chapterContent == "" {
		return "", compose.NewInterruptAndRerunErr("分析章节结构需要提供有效的 chapter_content 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	// 分析章节结构
	structure := t.analyzeChapterStructure(chapterContent)

	return t.successResponse("章节结构分析完成", structure), nil
}

// handleGetLatest 处理获取最新章节
func (t *ChapterAnalysisTool) handleGetLatest() (string, error) {
	// 创建章节管理器
	chapterManager := managers.NewChapterManager(t.novelDir)

	// 获取最新章节内容
	latestChapter, err := chapterManager.GetLatestChapterContent()
	if err != nil {
		return "", fmt.Errorf("获取最新章节失败: %w", err)
	}

	// 获取最新章节路径
	latestPath := chapterManager.GetLatestChapterPath()

	// 提取章节信息
	info := t.extractChapterInfo(latestChapter)

	result := map[string]interface{}{
		"chapter_content": latestChapter,
		"chapter_path":    latestPath,
		"chapter_info":    info,
	}

	return t.successResponse("最新章节获取成功", result), nil
}

// ChapterBasicInfo 章节基本信息
type ChapterBasicInfo struct {
	Title       string `json:"title"`
	WordCount   int    `json:"word_count"`
	LineCount   int    `json:"line_count"`
	ParagraphCount int `json:"paragraph_count"`
	HasDialogue bool   `json:"has_dialogue"`
	Language    string `json:"language"`
}

// extractChapterInfo 提取章节基本信息
func (t *ChapterAnalysisTool) extractChapterInfo(content string) *ChapterBasicInfo {
	lines := strings.Split(content, "\n")
	
	info := &ChapterBasicInfo{
		WordCount:      len(strings.Fields(content)),
		LineCount:      len(lines),
		ParagraphCount: t.countParagraphs(content),
		HasDialogue:    t.hasDialogue(content),
		Language:       "中文", // 简单假设
	}

	// 尝试从第一行提取标题
	if len(lines) > 0 {
		firstLine := strings.TrimSpace(lines[0])
		
		// 检查多种标题格式
		if strings.HasPrefix(firstLine, "===") && strings.HasSuffix(firstLine, "===") {
			info.Title = strings.TrimSpace(strings.Trim(firstLine, "="))
		} else if strings.HasPrefix(firstLine, "第") && strings.Contains(firstLine, "章") {
			info.Title = firstLine
		} else if strings.Contains(firstLine, "Chapter") || strings.Contains(firstLine, "第") {
			info.Title = firstLine
		} else {
			info.Title = "未识别标题"
		}
	}

	return info
}

// cleanChapterContent 清理章节内容
func (t *ChapterAnalysisTool) cleanChapterContent(content string) string {
	// 移除 HTML 标签
	htmlRegex := regexp.MustCompile(`<[^>]*>`)
	cleaned := htmlRegex.ReplaceAllString(content, "")

	// 移除思考标签 <think>...</think>
	thinkRegex := regexp.MustCompile(`(?s)<think>.*?</think>`)
	cleaned = thinkRegex.ReplaceAllString(cleaned, "")

	// 移除多余空白行
	cleaned = regexp.MustCompile(`\n\s*\n\s*\n`).ReplaceAllString(cleaned, "\n\n")

	// 移除行首尾空白
	lines := strings.Split(cleaned, "\n")
	var cleanedLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleanedLines = append(cleanedLines, trimmed)
		}
	}

	return strings.Join(cleanedLines, "\n")
}

// ChapterStructure 章节结构信息
type ChapterStructure struct {
	HasTitle      bool     `json:"has_title"`
	DialogueCount int      `json:"dialogue_count"`
	ActionCount   int      `json:"action_count"`
	DescCount     int      `json:"description_count"`
	Characters    []string `json:"characters"`
	Locations     []string `json:"locations"`
	TimeMarkers   []string `json:"time_markers"`
}

// analyzeChapterStructure 分析章节结构
func (t *ChapterAnalysisTool) analyzeChapterStructure(content string) *ChapterStructure {
	structure := &ChapterStructure{
		HasTitle:      t.hasTitle(content),
		DialogueCount: t.countDialogue(content),
		ActionCount:   t.countActionPhrases(content),
		DescCount:     t.countDescriptions(content),
		Characters:    t.extractCharacterNames(content),
		Locations:     t.extractLocations(content),
		TimeMarkers:   t.extractTimeMarkers(content),
	}

	return structure
}

// 辅助方法

// countParagraphs 计算段落数
func (t *ChapterAnalysisTool) countParagraphs(content string) int {
	paragraphs := strings.Split(content, "\n\n")
	count := 0
	for _, p := range paragraphs {
		if strings.TrimSpace(p) != "" {
			count++
		}
	}
	return count
}

// hasDialogue 检查是否包含对话
func (t *ChapterAnalysisTool) hasDialogue(content string) bool {
	// 简单检查引号
	return strings.Contains(content, "\"") || strings.Contains(content, "'") ||
		strings.Contains(content, "\"") || strings.Contains(content, "\"")
}

// hasTitle 检查是否有标题
func (t *ChapterAnalysisTool) hasTitle(content string) bool {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return false
	}
	
	firstLine := strings.TrimSpace(lines[0])
	return strings.Contains(firstLine, "第") && strings.Contains(firstLine, "章") ||
		   strings.Contains(firstLine, "Chapter") ||
		   (strings.HasPrefix(firstLine, "===") && strings.HasSuffix(firstLine, "==="))
}

// countDialogue 计算对话数量
func (t *ChapterAnalysisTool) countDialogue(content string) int {
	dialogueRegex := regexp.MustCompile(`["""]([^"""]*?)["""]`)
	return len(dialogueRegex.FindAllString(content, -1))
}

// countActionPhrases 计算动作描述数量
func (t *ChapterAnalysisTool) countActionPhrases(content string) int {
	actionWords := []string{"走", "跑", "坐", "站", "看", "听", "说", "想", "拿", "放"}
	count := 0
	for _, word := range actionWords {
		count += strings.Count(content, word)
	}
	return count
}

// countDescriptions 计算描述性内容数量
func (t *ChapterAnalysisTool) countDescriptions(content string) int {
	descWords := []string{"的", "得", "地", "很", "非常", "十分", "极其"}
	count := 0
	for _, word := range descWords {
		count += strings.Count(content, word)
	}
	return count / 10 // 简单估算
}

// extractCharacterNames 提取角色名称
func (t *ChapterAnalysisTool) extractCharacterNames(content string) []string {
	// 简单的中文人名匹配
	nameRegex := regexp.MustCompile(`[云李王张刘陈杨黄赵吴周徐孙马朱胡郭何高林罗郑梁谢宋唐许韩冯邓曹彭曾萧蔡潘田董袁于余叶蒋杜苏魏程吕丁任沈姚卢姜崔钟谭陆汪范金石廖贾夏韦傅方白邹孟熊秦邱江尹薛闫段雷龙黎史陶贺顾毛郝龚邵万钱严覃武戴莫孔向汤]\w{1,2}`)
	matches := nameRegex.FindAllString(content, -1)
	
	// 去重
	nameMap := make(map[string]bool)
	var names []string
	for _, name := range matches {
		if !nameMap[name] {
			nameMap[name] = true
			names = append(names, name)
		}
	}
	
	return names
}

// extractLocations 提取地点信息
func (t *ChapterAnalysisTool) extractLocations(content string) []string {
	locationWords := []string{"房间", "客厅", "卧室", "厨房", "院子", "花园", "街道", "公园", "学校", "医院"}
	var locations []string
	
	for _, loc := range locationWords {
		if strings.Contains(content, loc) {
			locations = append(locations, loc)
		}
	}
	
	return locations
}

// extractTimeMarkers 提取时间标记
func (t *ChapterAnalysisTool) extractTimeMarkers(content string) []string {
	timeWords := []string{"早晨", "上午", "中午", "下午", "晚上", "深夜", "今天", "昨天", "明天"}
	var timeMarkers []string
	
	for _, time := range timeWords {
		if strings.Contains(content, time) {
			timeMarkers = append(timeMarkers, time)
		}
	}
	
	return timeMarkers
}

// successResponse 构建成功响应
func (t *ChapterAnalysisTool) successResponse(message string, data interface{}) string {
	response := map[string]interface{}{
		"success": true,
		"message": message,
		"data":    data,
	}

	jsonBytes, _ := sonic.MarshalIndent(response, "", "  ")
	return string(jsonBytes)
}