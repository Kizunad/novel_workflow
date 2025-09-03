package summarizer

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"

	"github.com/Kizunad/modular-workflow-v2/logger"
	"github.com/Kizunad/modular-workflow-v2/providers"
)

// Task 任务接口
type Task interface {
	GetID() string
	GetType() string
}

// Processor 摘要处理器
type Processor struct {
	llmManager *providers.Manager
	logger     *logger.ZapLogger
	model      string
}

// NewProcessor 创建摘要处理器
func NewProcessor(llmManager *providers.Manager, logger *logger.ZapLogger) *Processor {
	return &Processor{
		llmManager: llmManager,
		logger:     logger,
		model:      "qwen3:4b", // 使用 Ollama 本地模型
	}
}

// TaskType 实现 TaskProcessor 接口
func (p *Processor) TaskType() string {
	return "summarize"
}

// ProcessTask 实现 TaskProcessor 接口
func (p *Processor) ProcessTask(ctx context.Context, task Task) error {
	summarizeTask, ok := task.(*SummarizeTask)
	if !ok {
		return fmt.Errorf("无效的摘要任务类型")
	}

	p.logger.Info(fmt.Sprintf("开始处理摘要任务: %s", summarizeTask.GetID()))

	return p.processSummarize(ctx, summarizeTask)
}

// processSummarize 处理摘要任务
func (p *Processor) processSummarize(ctx context.Context, task *SummarizeTask) error {
	// 首先检测小说路径是否存在
	if _, err := os.Stat(task.NovelPath); os.IsNotExist(err) {
		return fmt.Errorf("小说目录不存在: %s", task.NovelPath)
	}

	// 获取 Ollama 模型
	model, err := p.llmManager.GetOllamaModel(ctx, providers.WithModel(p.model))
	if err != nil {
		return fmt.Errorf("获取 Ollama 模型失败: %w", err)
	}

	// 提取章节信息
	chapterInfo, err := p.extractChapterInfo(task.Content)
	if err != nil {
		return fmt.Errorf("提取章节信息失败: %w", err)
	}

	// 生成摘要
	summary, err := p.generateSummary(ctx, model, task.Content, chapterInfo)
	if err != nil {
		return fmt.Errorf("生成摘要失败: %w", err)
	}

	// 更新索引
	indexManager := NewIndexManager(task.NovelPath)
	if err := indexManager.UpdateSummary(*summary); err != nil {
		return fmt.Errorf("更新索引失败: %w", err)
	}

	p.logger.Info(fmt.Sprintf("摘要任务完成: %s -> 章节 %s", task.GetID(), summary.ChapterID))
	return nil
}

// extractChapterInfo 从内容中提取章节信息
func (p *Processor) extractChapterInfo(content string) (*ChapterInfo, error) {
	// 简单的章节信息提取逻辑
	lines := strings.Split(content, "\n")

	info := &ChapterInfo{
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

// ChapterInfo 章节信息
type ChapterInfo struct {
	ChapterID string
	Title     string
	WordCount int
}

// generateSummary 生成章节摘要
func (p *Processor) generateSummary(ctx context.Context, model model.ToolCallingChatModel, content string, info *ChapterInfo) (*ChapterSummary, error) {
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
	summaryContent := p.cleanSummaryContent(response.Content)
	if summaryContent == "" {
		summaryContent = "内容更新"
	}

	// 构建最终摘要
	summary := &ChapterSummary{
		ChapterID: info.ChapterID,
		Title:     info.Title,
		Summary:   summaryContent,
		WordCount: info.WordCount,
		Timestamp: time.Now(),
	}

	return summary, nil
}

// cleanSummaryContent 清理摘要内容，去除思考标签和无关内容
func (p *Processor) cleanSummaryContent(content string) string {
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
