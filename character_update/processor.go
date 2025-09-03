package character_update

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"

	"github.com/Kizunad/modular-workflow-v2/components/content/managers"
	"github.com/Kizunad/modular-workflow-v2/logger"
	"github.com/Kizunad/modular-workflow-v2/providers"
)

// Task 任务接口
type Task interface {
	GetID() string
	GetType() string
}

// Processor 角色更新处理器
type Processor struct {
	llmManager *providers.Manager
	logger     *logger.ZapLogger
	model      string
}

// NewProcessor 创建角色更新处理器
func NewProcessor(llmManager *providers.Manager, logger *logger.ZapLogger) *Processor {
	return &Processor{
		llmManager: llmManager,
		logger:     logger,
		model:      "qwen3:4b", // 使用 Ollama 本地模型
	}
}

// TaskType 实现 TaskProcessor 接口
func (p *Processor) TaskType() string {
	return "character_update"
}

// ProcessTask 实现 TaskProcessor 接口
func (p *Processor) ProcessTask(ctx context.Context, task Task) error {
	updateTask, ok := task.(*CharacterUpdateTask)
	if !ok {
		return fmt.Errorf("无效的角色更新任务类型")
	}

	p.logger.Info(fmt.Sprintf("开始处理角色更新任务: %s", updateTask.GetID()))

	return p.processCharacterUpdate(ctx, updateTask)
}

// processCharacterUpdate 处理角色更新任务
func (p *Processor) processCharacterUpdate(ctx context.Context, task *CharacterUpdateTask) error {
	// 检查小说路径是否存在
	if _, err := os.Stat(task.NovelPath); os.IsNotExist(err) {
		return fmt.Errorf("小说目录不存在: %s", task.NovelPath)
	}

	// 创建管理器
	characterManager := managers.NewCharacterManager(task.NovelPath)
	chapterManager := managers.NewChapterManager(task.NovelPath)

	// 直接获取当前角色信息（不使用Token限制）
	currentCharacter := characterManager.GetCurrent()
	if currentCharacter == "" {
		return fmt.Errorf("未找到角色信息文件")
	}

	// 直接获取最新章节内容（不使用Token限制）
	latestChapter, err := chapterManager.GetLatestChapterContent()
	if err != nil {
		// 如果没有章节内容，记录但不退出
		p.logger.Warn(fmt.Sprintf("获取最新章节失败，将仅基于角色信息更新: %v", err))
		latestChapter = ""
	}

	// 如果提供了具体的更新内容，直接使用
	if task.UpdateContent != "" {
		if err := characterManager.Update(task.UpdateContent); err != nil {
			return fmt.Errorf("更新角色信息失败: %w", err)
		}
		p.logger.Info(fmt.Sprintf("角色信息已直接更新: %s", task.CharacterName))
		return nil
	}

	// 使用AI分析最新章节对角色的影响
	updateResult, err := p.analyzeCharacterChanges(ctx, currentCharacter, latestChapter, task.CharacterName)
	if err != nil {
		return fmt.Errorf("分析角色变化失败: %w", err)
	}

	// 根据分析结果决定是否更新
	if updateResult.NeedsUpdate {
		if err := characterManager.Update(updateResult.UpdatedCharacter); err != nil {
			return fmt.Errorf("保存角色信息失败: %w", err)
		}
		p.logger.Info(fmt.Sprintf("角色信息已更新: %s - %s", task.CharacterName, updateResult.Reason))
	} else {
		p.logger.Info(fmt.Sprintf("角色信息无需更新: %s - %s", task.CharacterName, updateResult.Reason))
	}

	p.logger.Info(fmt.Sprintf("角色更新任务完成: %s -> %s", task.GetID(), task.CharacterName))
	return nil
}

// AnalysisResult AI分析结果
type AnalysisResult struct {
	NeedsUpdate      bool   `json:"needs_update"`      // 是否需要更新
	UpdatedCharacter string `json:"updated_character"` // 更新后的角色信息
	Reason           string `json:"reason"`            // 更新/不更新的原因
}

// analyzeCharacterChanges 分析章节内容对角色的影响
func (p *Processor) analyzeCharacterChanges(ctx context.Context, currentCharacter, latestChapter, characterName string) (*AnalysisResult, error) {
	// 获取 Ollama 模型
	model, err := p.llmManager.GetOllamaModel(ctx, providers.WithModel(p.model))
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
	return p.parseAnalysisResponse(response.Content, currentCharacter)
}

// parseAnalysisResponse 解析AI分析响应
func (p *Processor) parseAnalysisResponse(response, currentCharacter string) (*AnalysisResult, error) {
	// 清理响应内容
	cleaned := p.cleanAIOutput(response)

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
func (p *Processor) cleanAIOutput(content string) string {
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
