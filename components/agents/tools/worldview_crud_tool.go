package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/Kizunad/modular-workflow-v2/providers"
)

// WorldviewCRUDTool 世界观增删改查工具，基于worldview_summarizer/processor.go重构
type WorldviewCRUDTool struct {
	novelDir   string
	llmManager *providers.Manager
	model      string
}

// NewWorldviewCRUDTool 创建世界观CRUD工具
func NewWorldviewCRUDTool(novelDir string, llmManager *providers.Manager) *WorldviewCRUDTool {
	return &WorldviewCRUDTool{
		novelDir:   novelDir,
		llmManager: llmManager,
		model:      "qwen3:4b", // 使用 Ollama 本地模型
	}
}

// Info 实现BaseTool接口
func (t *WorldviewCRUDTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "worldview_crud",
		Desc: "世界观管理工具，用于读取、更新、分析世界观设定。支持AI智能分析章节对世界观的影响并自动更新世界设定",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action": {
				Type:     schema.String,
				Desc:     "操作类型: read/update/analyze_changes/merge_update",
				Required: true,
			},
			"update_content": {
				Type:     schema.String,
				Desc:     "直接更新的世界观内容",
				Required: false,
			},
			"latest_chapter": {
				Type:     schema.String,
				Desc:     "最新章节内容，用于AI分析",
				Required: false,
			},
		}),
	}, nil
}

// InvokableRun 实现InvokableTool接口
func (t *WorldviewCRUDTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// 解析输入参数
	var input map[string]interface{}
	if err := sonic.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", compose.NewInterruptAndRerunErr("JSON参数解析失败，请检查格式并重新调用。原始参数: " + argumentsInJSON + "，错误: " + err.Error())
	}

	// 调用内部方法处理
	return t.invoke(ctx, input)
}

// invoke 内部调用方法
func (t *WorldviewCRUDTool) invoke(ctx context.Context, input map[string]any) (string, error) {
	action, ok := input["action"].(string)
	if !ok {
		return "", compose.NewInterruptAndRerunErr("缺少必要参数 action（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	switch action {
	case "read":
		return t.handleRead()
	case "update":
		return t.handleUpdate(input)
	case "analyze_changes":
		return t.handleAnalyzeChanges(ctx, input)
	case "merge_update":
		return t.handleMergeUpdate(input)
	default:
		return "", compose.NewInterruptAndRerunErr("不支持的操作类型: " + action + "，支持的操作: read/update/analyze_changes/merge_update，当前参数: " + fmt.Sprintf("%v", input))
	}
}

// handleRead 处理读取世界观信息
func (t *WorldviewCRUDTool) handleRead() (string, error) {
	// 世界观文件路径
	worldviewPath := filepath.Join(t.novelDir, "worldview.md")

	// 读取现有世界观内容
	var currentWorldview string
	if content, err := os.ReadFile(worldviewPath); err == nil {
		currentWorldview = string(content)
	} else {
		currentWorldview = ""
	}

	// 提取最新世界信息
	latestWorldInfo := t.extractLatestWorldInfo(currentWorldview)

	return t.successResponse("世界观信息读取成功", map[string]string{
		"worldview_content":   currentWorldview,
		"latest_world_info":   latestWorldInfo,
		"worldview_file_path": worldviewPath,
	}), nil
}

// handleUpdate 处理直接更新世界观信息
func (t *WorldviewCRUDTool) handleUpdate(input map[string]any) (string, error) {
	updateContent, contentOk := input["update_content"].(string)
	if !contentOk || updateContent == "" {
		return "", compose.NewInterruptAndRerunErr("更新世界观信息需要提供有效的 update_content 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	// 世界观文件路径
	worldviewPath := filepath.Join(t.novelDir, "worldview.md")

	// 保存世界观文件
	if err := t.saveWorldview(worldviewPath, updateContent); err != nil {
		return "", fmt.Errorf("保存世界观文件失败: %w", err)
	}

	return t.successResponse("世界观信息更新成功", map[string]string{
		"updated_content": updateContent,
		"file_path":       worldviewPath,
	}), nil
}

// handleMergeUpdate 处理合并更新世界观信息
func (t *WorldviewCRUDTool) handleMergeUpdate(input map[string]any) (string, error) {
	updateContent, contentOk := input["update_content"].(string)
	if !contentOk || updateContent == "" {
		return "", compose.NewInterruptAndRerunErr("合并更新世界观信息需要提供有效的 update_content 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	// 世界观文件路径
	worldviewPath := filepath.Join(t.novelDir, "worldview.md")

	// 读取现有世界观内容
	var currentWorldview string
	if content, err := os.ReadFile(worldviewPath); err == nil {
		currentWorldview = string(content)
	}

	// 合并世界观更新内容
	updatedWorldview := t.mergeWorldviewUpdate(currentWorldview, updateContent)

	// 保存世界观文件
	if err := t.saveWorldview(worldviewPath, updatedWorldview); err != nil {
		return "", fmt.Errorf("保存世界观文件失败: %w", err)
	}

	return t.successResponse("世界观信息合并更新成功", map[string]string{
		"original_content": currentWorldview,
		"update_content":   updateContent,
		"merged_content":   updatedWorldview,
	}), nil
}

// handleAnalyzeChanges 处理AI分析世界观变化
func (t *WorldviewCRUDTool) handleAnalyzeChanges(ctx context.Context, input map[string]any) (string, error) {
	latestChapter, chapterOk := input["latest_chapter"].(string)
	if !chapterOk || latestChapter == "" {
		return "", compose.NewInterruptAndRerunErr("分析世界观变化需要提供有效的 latest_chapter 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	// 世界观文件路径
	worldviewPath := filepath.Join(t.novelDir, "worldview.md")

	// 读取现有世界观内容
	var currentWorldview string
	if content, err := os.ReadFile(worldviewPath); err == nil {
		currentWorldview = string(content)
	}

	// 使用AI分析世界观变化
	analysisResult, err := t.analyzeWorldviewChanges(ctx, currentWorldview, latestChapter)
	if err != nil {
		return "", fmt.Errorf("分析世界观变化失败: %w", err)
	}

	// 如果需要更新，执行更新
	if analysisResult.NeedsUpdate {
		if err := t.saveWorldview(worldviewPath, analysisResult.UpdatedWorldview); err != nil {
			return "", fmt.Errorf("保存世界观信息失败: %w", err)
		}
	}

	return t.successResponse("世界观变化分析完成", analysisResult), nil
}

// WorldviewAnalysisResult AI世界观分析结果
type WorldviewAnalysisResult struct {
	NeedsUpdate      bool   `json:"needs_update"`      // 是否需要更新
	UpdatedWorldview string `json:"updated_worldview"` // 更新后的世界观信息
	Reason           string `json:"reason"`            // 更新/不更新的原因
}

// analyzeWorldviewChanges 分析章节内容对世界观的影响
func (t *WorldviewCRUDTool) analyzeWorldviewChanges(ctx context.Context, currentWorldview, latestChapter string) (*WorldviewAnalysisResult, error) {
	// 获取 Ollama 模型
	model, err := t.llmManager.GetOllamaModel(ctx, providers.WithModel(t.model))
	if err != nil {
		return nil, fmt.Errorf("获取 Ollama 模型失败: %w", err)
	}

	// 检测当前最新的世界设定
	latestWorldInfo := t.extractLatestWorldInfo(currentWorldview)

	// 创建世界观分析模板
	template := prompt.FromMessages(
		schema.Jinja2,
		schema.SystemMessage(`你是一个专业的小说世界观分析师。你的任务是：

1. **仔细分析** 最新章节内容中是否包含新的世界设定信息
2. **精确判断** 是否需要更新世界观设定
3. **保守原则** 只有在发现确实的新世界设定时才建议更新

世界观更新的关键要素包括：
- 新的魔法/力量系统规则
- 新的世界地理/区域信息
- 新的种族/生物设定
- 新的政治/社会结构
- 新的历史/背景信息
- 新的技术/文明程度描述

**更新规则：**
- 如果发现新的世界设定，需要检测当前最新的世界编号
- 如果是对现有世界的补充，使用 [UPDATE]...[UPDATE] 结构在现有设定后追加
- 如果是全新世界，创建新的 "## 世界{编号}：{名称}" 条目
- 保持原有格式结构不变

**当前最新世界信息：**
{{latest_world_info}}

**输出格式：**
如果需要更新，请返回：
UPDATE_NEEDED
[更新后的完整世界观内容]
REASON: [具体更新原因]

如果无需更新，请返回：
NO_UPDATE_NEEDED  
REASON: [不更新的原因]`),
		schema.UserMessage(`当前世界观内容：
{{current_worldview}}

最新章节内容：
{{latest_chapter}}

请分析是否需要更新世界观设定：`),
	)

	// 构建输入
	input := map[string]any{
		"current_worldview": currentWorldview,
		"latest_chapter":    latestChapter,
		"latest_world_info": latestWorldInfo,
	}

	// 格式化提示
	messages, err := template.Format(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("格式化提示失败: %w", err)
	}

	// 调用模型分析
	response, err := model.Generate(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("模型分析失败: %w", err)
	}

	// 解析AI响应
	return t.parseWorldviewAnalysisResponse(response.Content, currentWorldview)
}

// extractLatestWorldInfo 提取最新世界信息
func (t *WorldviewCRUDTool) extractLatestWorldInfo(worldviewContent string) string {
	if worldviewContent == "" {
		return "暂无世界设定信息"
	}

	// 匹配世界标题的正则表达式：## 世界{数字} ：{名称}
	worldRegex := regexp.MustCompile(`(?m)^## 世界(\d+) ：(.+)$`)
	matches := worldRegex.FindAllStringSubmatch(worldviewContent, -1)

	if len(matches) == 0 {
		return "未找到标准格式的世界设定"
	}

	// 找到最大编号的世界
	maxWorldNum := 0
	var latestWorldTitle string
	for _, match := range matches {
		if num, err := strconv.Atoi(match[1]); err == nil && num > maxWorldNum {
			maxWorldNum = num
			latestWorldTitle = match[0] // 完整的标题行
		}
	}

	// 提取该世界的完整内容
	lines := strings.Split(worldviewContent, "\n")
	var worldContent []string
	inTargetWorld := false

	for _, line := range lines {
		if strings.Contains(line, latestWorldTitle) {
			inTargetWorld = true
			worldContent = append(worldContent, line)
			continue
		}

		// 如果遇到下一个世界标题，停止收集
		if inTargetWorld && worldRegex.MatchString(line) {
			break
		}

		if inTargetWorld {
			worldContent = append(worldContent, line)
		}
	}

	if len(worldContent) == 0 {
		return "未找到最新世界的详细设定"
	}

	return fmt.Sprintf("最新世界编号: %d\n内容:\n%s", maxWorldNum, strings.Join(worldContent, "\n"))
}

// parseWorldviewAnalysisResponse 解析AI世界观分析响应
func (t *WorldviewCRUDTool) parseWorldviewAnalysisResponse(response, currentWorldview string) (*WorldviewAnalysisResult, error) {
	// 清理响应内容
	cleaned := t.cleanAIOutput(response)

	// 检查是否需要更新
	if strings.Contains(cleaned, "UPDATE_NEEDED") {
		// 提取更新后的世界观信息
		lines := strings.Split(cleaned, "\n")
		var updatedWorldview []string
		var reason string
		var inWorldviewSection bool

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "UPDATE_NEEDED" {
				inWorldviewSection = true
				continue
			}
			if strings.HasPrefix(line, "REASON:") {
				reason = strings.TrimSpace(strings.TrimPrefix(line, "REASON:"))
				inWorldviewSection = false
				continue
			}
			if inWorldviewSection && line != "" {
				updatedWorldview = append(updatedWorldview, line)
			}
		}

		if len(updatedWorldview) == 0 {
			return &WorldviewAnalysisResult{
				NeedsUpdate:      false,
				UpdatedWorldview: currentWorldview,
				Reason:           "AI响应格式错误，保持原有世界观",
			}, nil
		}

		return &WorldviewAnalysisResult{
			NeedsUpdate:      true,
			UpdatedWorldview: strings.Join(updatedWorldview, "\n"),
			Reason:           reason,
		}, nil
	}

	// 不需要更新
	reason := "未检测到世界观设定变化"
	if strings.Contains(cleaned, "REASON:") {
		reasonParts := strings.Split(cleaned, "REASON:")
		if len(reasonParts) > 1 {
			reason = strings.TrimSpace(reasonParts[1])
		}
	}

	return &WorldviewAnalysisResult{
		NeedsUpdate:      false,
		UpdatedWorldview: currentWorldview,
		Reason:           reason,
	}, nil
}

// mergeWorldviewUpdate 合并世界观更新内容
func (t *WorldviewCRUDTool) mergeWorldviewUpdate(currentWorldview, updateContent string) string {
	if currentWorldview == "" {
		return updateContent
	}

	// 检查是否包含 [UPDATE] 标记
	if strings.Contains(updateContent, "[UPDATE]") {
		// 提取 [UPDATE] 之间的内容
		updateRegex := regexp.MustCompile(`\[UPDATE\](.*?)\[UPDATE\]`)
		matches := updateRegex.FindAllStringSubmatch(updateContent, -1)

		if len(matches) > 0 {
			// 获取最新世界的位置并添加更新内容
			return t.addUpdateToLatestWorld(currentWorldview, matches[0][1])
		}
	}

	// 如果没有特殊标记，直接追加
	return currentWorldview + "\n\n" + updateContent
}

// addUpdateToLatestWorld 在最新世界设定后添加更新内容
func (t *WorldviewCRUDTool) addUpdateToLatestWorld(currentWorldview, updateContent string) string {
	worldRegex := regexp.MustCompile(`(?m)^## 世界(\d+)：(.+)$`)
	matches := worldRegex.FindAllStringSubmatchIndex(currentWorldview, -1)

	if len(matches) == 0 {
		// 如果没有找到世界设定，直接追加
		return currentWorldview + "\n\n" + strings.TrimSpace(updateContent)
	}

	// 找到最后一个世界的位置
	lastMatch := matches[len(matches)-1]

	// 找到该世界设定的结束位置（下一个## 或文件结尾）
	lines := strings.Split(currentWorldview, "\n")
	startLine := strings.Count(currentWorldview[:lastMatch[0]], "\n")

	insertIndex := len(lines) // 默认插入到最后
	for i := startLine + 1; i < len(lines); i++ {
		if strings.HasPrefix(strings.TrimSpace(lines[i]), "## 世界") {
			insertIndex = i
			break
		}
	}

	// 在合适位置插入更新内容
	result := make([]string, 0, len(lines)+10)
	result = append(result, lines[:insertIndex]...)
	result = append(result, "  "+strings.TrimSpace(updateContent)) // 添加适当缩进
	result = append(result, lines[insertIndex:]...)

	return strings.Join(result, "\n")
}

// saveWorldview 追加保存世界观文件（只能追加，不能覆盖）
func (t *WorldviewCRUDTool) saveWorldview(worldviewPath, content string) error {
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(worldviewPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}
	
	// 检查原文件是否存在
	if _, err := os.Stat(worldviewPath); err == nil {
		// 文件已存在，将内容追加到"世界 1"章节末尾
		return t.appendToWorld1Section(worldviewPath, content)
	} else {
		// 文件不存在，创建新文件
		if err := os.WriteFile(worldviewPath, []byte(content), 0644); err != nil {
			return fmt.Errorf("创建世界观文件失败: %w", err)
		}
	}

	return nil
}

// appendToWorld1Section 将内容追加到"世界 1"章节的末尾
func (t *WorldviewCRUDTool) appendToWorld1Section(worldviewPath, content string) error {
	// 读取现有文件内容
	existingContent, err := os.ReadFile(worldviewPath)
	if err != nil {
		return fmt.Errorf("读取现有文件失败: %w", err)
	}
	
	lines := strings.Split(string(existingContent), "\n")
	
	// 查找"世界 1"章节的开始和结束位置
	world1StartIndex := -1
	world1EndIndex := len(lines) // 默认到文件末尾
	
	// 匹配"## 世界 1"的正则表达式
	world1Regex := regexp.MustCompile(`^## 世界\s*1\s*[:：]`)
	
	for i, line := range lines {
		if world1Regex.MatchString(strings.TrimSpace(line)) {
			world1StartIndex = i
		} else if world1StartIndex != -1 && strings.HasPrefix(strings.TrimSpace(line), "## 世界") && !world1Regex.MatchString(strings.TrimSpace(line)) {
			// 找到下一个世界章节，结束位置是当前行的前一行
			world1EndIndex = i
			break
		}
	}
	
	if world1StartIndex == -1 {
		// 没有找到"世界 1"章节，直接追加到文件末尾
		file, err := os.OpenFile(worldviewPath, os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("打开文件进行追加失败: %w", err)
		}
		defer file.Close()
		
		_, err = file.WriteString("\n\n" + content)
		if err != nil {
			return fmt.Errorf("追加世界观内容失败: %w", err)
		}
		return nil
	}
	
	// 构建新的文件内容：在"世界 1"章节末尾插入新内容
	var newLines []string
	newLines = append(newLines, lines[:world1EndIndex]...)
	
	// 添加新内容，使用适当的缩进
	newLines = append(newLines, "")
	newLines = append(newLines, "  "+content) // 使用2个空格缩进，表示属于"世界 1"章节
	
	// 添加剩余的内容（如果有其他世界章节）
	if world1EndIndex < len(lines) {
		newLines = append(newLines, "")
		newLines = append(newLines, lines[world1EndIndex:]...)
	}
	
	// 写入更新后的内容
	newContent := strings.Join(newLines, "\n")
	if err := os.WriteFile(worldviewPath, []byte(newContent), 0644); err != nil {
		return fmt.Errorf("写入更新后的文件失败: %w", err)
	}
	
	return nil
}


// cleanAIOutput 清理AI输出内容，移除思考标签和无用信息
func (t *WorldviewCRUDTool) cleanAIOutput(content string) string {
	// 移除 <think>...</think> 标签和其内容
	thinkRegex := regexp.MustCompile(`(?s)<think>.*?</think>`)
	cleaned := thinkRegex.ReplaceAllString(content, "")

	// 移除其他可能的XML标签
	xmlRegex := regexp.MustCompile(`<[^>]*>`)
	cleaned = xmlRegex.ReplaceAllString(cleaned, "")

	// 移除多余的空行和前后空白
	lines := strings.Split(cleaned, "\n")
	var validLines []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			validLines = append(validLines, line)
		}
	}

	result := strings.Join(validLines, "\n")
	return strings.TrimSpace(result)
}

// successResponse 构建成功响应
func (t *WorldviewCRUDTool) successResponse(message string, data interface{}) string {
	response := map[string]interface{}{
		"success": true,
		"message": message,
		"data":    data,
	}

	jsonBytes, _ := sonic.MarshalIndent(response, "", "  ")
	return string(jsonBytes)
}