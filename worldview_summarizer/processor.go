package worldview_summarizer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

// Processor 世界观总结处理器
type Processor struct {
	llmManager *providers.Manager
	logger     *logger.ZapLogger
	model      string
}

// NewProcessor 创建世界观总结处理器
func NewProcessor(llmManager *providers.Manager, logger *logger.ZapLogger) *Processor {
	return &Processor{
		llmManager: llmManager,
		logger:     logger,
		model:      "qwen3:4b", // 使用 Ollama 本地模型
	}
}

// TaskType 实现 TaskProcessor 接口
func (p *Processor) TaskType() string {
	return "worldview_summarizer"
}

// ProcessTask 实现 TaskProcessor 接口
func (p *Processor) ProcessTask(ctx context.Context, task Task) error {
	worldviewTask, ok := task.(*WorldviewSummarizerTask)
	if !ok {
		return fmt.Errorf("无效的世界观总结任务类型")
	}

	p.logger.Info(fmt.Sprintf("开始处理世界观总结任务: %s", worldviewTask.GetID()))

	return p.processWorldviewSummarizer(ctx, worldviewTask)
}

// processWorldviewSummarizer 处理世界观总结任务
func (p *Processor) processWorldviewSummarizer(ctx context.Context, task *WorldviewSummarizerTask) error {
	// 检查小说路径是否存在
	if _, err := os.Stat(task.NovelPath); os.IsNotExist(err) {
		return fmt.Errorf("小说目录不存在: %s", task.NovelPath)
	}

	// 世界观文件路径
	worldviewPath := filepath.Join(task.NovelPath, "worldview.md")

	// 创建章节管理器获取最新内容
	chapterManager := managers.NewChapterManager(task.NovelPath)

	// 获取最新章节内容
	latestChapter, err := chapterManager.GetLatestChapterContent()
	if err != nil {
		p.logger.Warn(fmt.Sprintf("获取最新章节失败: %v", err))
		latestChapter = ""
	}

	// 读取现有世界观内容
	currentWorldview := ""
	if content, err := os.ReadFile(worldviewPath); err == nil {
		currentWorldview = string(content)
	}

	// 如果提供了具体的更新内容，直接处理
	if task.UpdateContent != "" {
		updatedWorldview := p.mergeWorldviewUpdate(currentWorldview, task.UpdateContent)
		if err := p.saveWorldview(worldviewPath, updatedWorldview); err != nil {
			return fmt.Errorf("保存世界观文件失败: %w", err)
		}
		p.logger.Info("世界观信息已直接更新")
		return nil
	}

	// 使用AI分析最新章节对世界观的影响
	updateResult, err := p.analyzeWorldviewChanges(ctx, currentWorldview, latestChapter)
	if err != nil {
		return fmt.Errorf("分析世界观变化失败: %w", err)
	}

	// 根据分析结果决定是否更新
	if updateResult.NeedsUpdate {
		if err := p.saveWorldview(worldviewPath, updateResult.UpdatedWorldview); err != nil {
			return fmt.Errorf("保存世界观信息失败: %w", err)
		}
		p.logger.Info(fmt.Sprintf("世界观信息已更新: %s", updateResult.Reason))
	} else {
		p.logger.Info(fmt.Sprintf("世界观信息无需更新: %s", updateResult.Reason))
	}

	p.logger.Info(fmt.Sprintf("世界观总结任务完成: %s", task.GetID()))
	return nil
}

// WorldviewAnalysisResult AI世界观分析结果
type WorldviewAnalysisResult struct {
	NeedsUpdate      bool   `json:"needs_update"`      // 是否需要更新
	UpdatedWorldview string `json:"updated_worldview"` // 更新后的世界观信息
	Reason           string `json:"reason"`            // 更新/不更新的原因
}

// analyzeWorldviewChanges 分析章节内容对世界观的影响
func (p *Processor) analyzeWorldviewChanges(ctx context.Context, currentWorldview, latestChapter string) (*WorldviewAnalysisResult, error) {
	// 获取 Ollama 模型
	model, err := p.llmManager.GetOllamaModel(ctx, providers.WithModel(p.model))
	if err != nil {
		return nil, fmt.Errorf("获取 Ollama 模型失败: %w", err)
	}

	// 检测当前最新的世界设定
	latestWorldInfo := p.extractLatestWorldInfo(currentWorldview)

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
	return p.parseWorldviewAnalysisResponse(response.Content, currentWorldview)
}

// extractLatestWorldInfo 提取最新世界信息
func (p *Processor) extractLatestWorldInfo(worldviewContent string) string {
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
func (p *Processor) parseWorldviewAnalysisResponse(response, currentWorldview string) (*WorldviewAnalysisResult, error) {
	// 清理响应内容
	cleaned := p.cleanAIOutput(response)

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
func (p *Processor) mergeWorldviewUpdate(currentWorldview, updateContent string) string {
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
			return p.addUpdateToLatestWorld(currentWorldview, matches[0][1])
		}
	}

	// 如果没有特殊标记，直接追加
	return currentWorldview + "\n\n" + updateContent
}

// addUpdateToLatestWorld 在最新世界设定后添加更新内容
func (p *Processor) addUpdateToLatestWorld(currentWorldview, updateContent string) string {
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

// saveWorldview 保存世界观文件
func (p *Processor) saveWorldview(worldviewPath, content string) error {
	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(worldviewPath), 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(worldviewPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("写入世界观文件失败: %w", err)
	}

	return nil
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
