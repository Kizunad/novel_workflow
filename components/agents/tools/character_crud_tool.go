package tools

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/Kizunad/modular-workflow-v2/components/content/managers"
	"github.com/Kizunad/modular-workflow-v2/providers"
)

// CharacterCRUDTool 角色增删改查工具，基于character_update/processor.go重构
type CharacterCRUDTool struct {
	novelDir   string
	llmManager *providers.Manager
	model      string
}

// NewCharacterCRUDTool 创建角色CRUD工具
func NewCharacterCRUDTool(novelDir string, llmManager *providers.Manager) *CharacterCRUDTool {
	return &CharacterCRUDTool{
		novelDir:   novelDir,
		llmManager: llmManager,
		model:      "qwen3:4b", // 使用 Ollama 本地模型
	}
}

// Info 实现BaseTool接口，使用简化的工具信息定义
func (t *CharacterCRUDTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "character_crud",
		Desc: "角色管理工具，用于读取、更新、分析角色信息。支持AI智能分析章节对角色的影响并自动更新角色状态",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action": {
				Type:     schema.String,
				Desc:     "操作类型: read/update/analyze_changes",
				Required: true,
			},
			"character_name": {
				Type:     schema.String,
				Desc:     "角色名称",
				Required: false,
			},
			"update_content": {
				Type:     schema.String,
				Desc:     "直接更新的角色内容",
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
func (t *CharacterCRUDTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// 解析输入参数
	var input map[string]interface{}
	if err := sonic.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", compose.NewInterruptAndRerunErr("JSON参数解析失败，请检查格式并重新调用。原始参数: " + argumentsInJSON + "，错误: " + err.Error())
	}

	// 调用内部方法处理
	return t.invoke(ctx, input)
}

// invoke 内部调用方法
func (t *CharacterCRUDTool) invoke(ctx context.Context, input map[string]any) (string, error) {
	action, ok := input["action"].(string)
	if !ok {
		return "", compose.NewInterruptAndRerunErr("缺少必要参数 action（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	// 创建角色管理器
	characterManager := managers.NewCharacterManager(t.novelDir)

	switch action {
	case "read":
		return t.handleRead(characterManager)
	case "update":
		return t.handleUpdate(characterManager, input)
	case "analyze_changes":
		return t.handleAnalyzeChanges(ctx, characterManager, input)
	default:
		return "", compose.NewInterruptAndRerunErr("不支持的操作类型: " + action + "，支持的操作: read/update/analyze_changes，当前参数: " + fmt.Sprintf("%v", input))
	}
}

// handleRead 处理读取角色信息
func (t *CharacterCRUDTool) handleRead(characterManager *managers.CharacterManager) (string, error) {
	// 检查文件是否存在
	if !characterManager.Exists() {
		return "", fmt.Errorf("角色信息文件不存在")
	}

	// 获取当前角色信息
	currentCharacter := characterManager.GetCurrent()
	if currentCharacter == "" {
		// 文件存在但为空，初始化默认内容
		if err := characterManager.ResetToDefault(); err != nil {
			return "", fmt.Errorf("初始化角色信息失败: %w", err)
		}
		currentCharacter = characterManager.GetCurrent()
	}

	return t.successResponse("角色信息读取成功", map[string]string{
		"character_info": currentCharacter,
	}), nil
}

// handleUpdate 处理更新角色信息
func (t *CharacterCRUDTool) handleUpdate(characterManager *managers.CharacterManager, input map[string]any) (string, error) {
	updateContent, contentOk := input["update_content"].(string)
	if !contentOk || updateContent == "" {
		return "", compose.NewInterruptAndRerunErr("更新角色信息需要提供有效的 update_content 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	// 更新角色信息
	if err := characterManager.Update(updateContent); err != nil {
		return "", fmt.Errorf("更新角色信息失败: %w", err)
	}

	return t.successResponse("角色信息更新成功", map[string]string{
		"updated_content": updateContent,
	}), nil
}

// handleAnalyzeChanges 处理AI分析角色变化
func (t *CharacterCRUDTool) handleAnalyzeChanges(ctx context.Context, characterManager *managers.CharacterManager, input map[string]any) (string, error) {
	characterName, nameOk := input["character_name"].(string)
	if !nameOk || characterName == "" {
		return "", compose.NewInterruptAndRerunErr("分析角色变化需要提供有效的 character_name 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	latestChapter, chapterOk := input["latest_chapter"].(string)
	if !chapterOk || latestChapter == "" {
		return "", compose.NewInterruptAndRerunErr("分析角色变化需要提供有效的 latest_chapter 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	// 检查文件是否存在
	if !characterManager.Exists() {
		return "", fmt.Errorf("角色信息文件不存在")
	}

	// 获取当前角色信息
	currentCharacter := characterManager.GetCurrent()
	if currentCharacter == "" {
		// 文件存在但为空，初始化默认内容
		if err := characterManager.ResetToDefault(); err != nil {
			return "", fmt.Errorf("初始化角色信息失败: %w", err)
		}
		currentCharacter = characterManager.GetCurrent()
	}

	// 使用AI分析角色变化
	analysisResult, err := t.analyzeCharacterChanges(ctx, currentCharacter, latestChapter, characterName)
	if err != nil {
		return "", fmt.Errorf("分析角色变化失败: %w", err)
	}

	// 如果需要更新，执行更新
	if analysisResult.NeedsUpdate {
		if err := characterManager.Update(analysisResult.UpdatedCharacter); err != nil {
			return "", fmt.Errorf("保存角色信息失败: %w", err)
		}
	}

	return t.successResponse("角色变化分析完成", analysisResult), nil
}

// AnalysisResult AI分析结果
type AnalysisResult struct {
	NeedsUpdate      bool   `json:"needs_update"`      // 是否需要更新
	UpdatedCharacter string `json:"updated_character"` // 更新后的角色信息
	Reason           string `json:"reason"`            // 更新/不更新的原因
}

// analyzeCharacterChanges 分析章节内容对角色的影响
func (t *CharacterCRUDTool) analyzeCharacterChanges(ctx context.Context, currentCharacter, latestChapter, characterName string) (*AnalysisResult, error) {
	// 获取 Ollama 模型
	model, err := t.llmManager.GetOllamaModel(ctx, providers.WithModel(t.model))
	if err != nil {
		return nil, fmt.Errorf("获取 Ollama 模型失败: %w", err)
	}

	// 创建角色分析模板
	template := prompt.FromMessages(
		schema.Jinja2,
		schema.SystemMessage(`你是一个专业的小说角色状态分析师。你的任务是：

1. **仔细对比** 最新章节内容 与 当前角色信息
2. **精确判断** 章节中是否存在影响角色状态的关键事件
3. **保守原则** 只有在确实有实质性变化时才建议更新

关键事件包括：
- 角色获得新能力、技能或力量
- 角色获得/失去重要物品或装备
- 角色位置发生重要变化
- 角色身体/精神状态发生显著改变
- 角色关系或身份发生重要变化

**严格要求：**
- 只根据章节中明确描述的事件更新
- 不推测未明确说明的变化
- 保持原有格式结构
- 如无实质性变化，明确说明"无需更新"

**输出格式：**
如果需要更新，请返回：
UPDATE_NEEDED
[更新后的完整角色信息]
REASON: [具体更新原因]

如果无需更新，请返回：
NO_UPDATE_NEEDED
REASON: [不更新的原因]

`),
		schema.UserMessage(`当前角色信息：
{{current_character}}

最新章节内容：
{{latest_chapter}}

目标角色：{{character_name}}

请分析是否需要更新角色信息：`),
	)

	// 构建输入
	input := map[string]any{
		"current_character": currentCharacter,
		"latest_chapter":    latestChapter,
		"character_name":    characterName,
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
	return t.parseAnalysisResponse(response.Content, currentCharacter)
}

// parseAnalysisResponse 解析AI分析响应
func (t *CharacterCRUDTool) parseAnalysisResponse(response, currentCharacter string) (*AnalysisResult, error) {
	// 清理响应内容
	cleaned := t.cleanAIOutput(response)

	// 检查是否需要更新
	if strings.Contains(cleaned, "UPDATE_NEEDED") {
		// 提取更新后的角色信息
		lines := strings.Split(cleaned, "\n")
		var updatedCharacter []string
		var reason string
		var inCharacterSection bool

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "UPDATE_NEEDED" {
				inCharacterSection = true
				continue
			}
			if strings.HasPrefix(line, "REASON:") {
				reason = strings.TrimSpace(strings.TrimPrefix(line, "REASON:"))
				inCharacterSection = false
				continue
			}
			if inCharacterSection && line != "" {
				updatedCharacter = append(updatedCharacter, line)
			}
		}

		if len(updatedCharacter) == 0 {
			return &AnalysisResult{
				NeedsUpdate:      false,
				UpdatedCharacter: currentCharacter,
				Reason:           "AI响应格式错误，保持原有信息",
			}, nil
		}

		return &AnalysisResult{
			NeedsUpdate:      true,
			UpdatedCharacter: strings.Join(updatedCharacter, "\n"),
			Reason:           reason,
		}, nil
	}

	// 不需要更新
	reason := "未检测到角色状态变化"
	if strings.Contains(cleaned, "REASON:") {
		reasonParts := strings.Split(cleaned, "REASON:")
		if len(reasonParts) > 1 {
			reason = strings.TrimSpace(reasonParts[1])
		}
	}

	return &AnalysisResult{
		NeedsUpdate:      false,
		UpdatedCharacter: currentCharacter,
		Reason:           reason,
	}, nil
}

// cleanAIOutput 清理AI输出内容，移除思考标签和无用信息
func (t *CharacterCRUDTool) cleanAIOutput(content string) string {
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
func (t *CharacterCRUDTool) successResponse(message string, data interface{}) string {
	response := map[string]interface{}{
		"success": true,
		"message": message,
		"data":    data,
	}

	jsonBytes, _ := sonic.MarshalIndent(response, "", "  ")
	return string(jsonBytes)
}