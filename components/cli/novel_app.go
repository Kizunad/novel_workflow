package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Kizunad/modular-workflow-v2/components/agents"
	"github.com/Kizunad/modular-workflow-v2/components/common"
	"github.com/Kizunad/modular-workflow-v2/components/content"
	"github.com/Kizunad/modular-workflow-v2/components/content/managers"
	"github.com/Kizunad/modular-workflow-v2/components/workflows"
	"github.com/Kizunad/modular-workflow-v2/config"
	"github.com/Kizunad/modular-workflow-v2/logger"
	"github.com/Kizunad/modular-workflow-v2/providers"
)

// NovelAppConfig 小说应用配置
type NovelAppConfig struct {
	*AppConfig
	ShowSteps   bool
	EnableRetry bool
}

// DefaultNovelAppConfig 默认小说应用配置
func DefaultNovelAppConfig() *NovelAppConfig {
	return &NovelAppConfig{
		AppConfig:   DefaultAppConfig(),
		ShowSteps:   true,
		EnableRetry: true,
	}
}

// NovelApp 小说续写应用
type NovelApp struct {
	*App
	novelConfig *NovelAppConfig
}

// NewNovelApp 创建小说续写应用
func NewNovelApp() *NovelApp {
	config := DefaultNovelAppConfig()
	config.Name = "小说续写工具"
	config.Description = "AI驱动的小说续写系统"

	return &NovelApp{
		App:         NewApp(config.AppConfig),
		novelConfig: config,
	}
}

// Run 运行小说续写应用
func (na *NovelApp) Run(args []string) error {
	ctx := context.Background()

	// 自定义解析参数和标志
	userInput, flags, err := na.ParseArgsWithFlags(args)

	// 检查帮助标志 - 优先使用自定义帮助
	if _, hasHelp := flags["-h"]; hasHelp {
		na.showUsage()
		return nil
	}
	if _, hasHelp := flags["--help"]; hasHelp {
		na.showUsage()
		return nil
	}

	// 如果有-p参数，忽略"参数不足"错误
	if err != nil {
		if _, hasP := flags["-p"]; !hasP {
			if _, hasPrompt := flags["--prompt"]; !hasPrompt {
				na.showUsage()
				return nil
			}
		}
		// 如果有-p或--prompt参数，继续执行
	}

	return na.handleNovelWritingWithFlags(ctx, na.App, userInput, flags)
}

// handleNovelWritingWithFlags 处理小说续写逻辑（支持标志参数）
func (na *NovelApp) handleNovelWritingWithFlags(ctx context.Context, app *App, userPrompt string, flags map[string]string) error {
	// 检查配置文件标志
	if configPath, ok := flags["--config"]; ok {
		na.App.config.ConfigPath = configPath
		na.GetCLI().ShowInfo("🔧", fmt.Sprintf("使用指定配置文件: %s", configPath))
	} else if configPath, ok := flags["-c"]; ok {
		na.App.config.ConfigPath = configPath
		na.GetCLI().ShowInfo("🔧", fmt.Sprintf("使用指定配置文件: %s", configPath))
	}

	// 初始化App
	if err := na.App.Initialize(ctx); err != nil {
		na.GetCLI().ShowGracefulError("初始化失败", err.Error(), "请检查配置文件是否正确")
		return err
	}

	// 处理 -p/--prompt 参数
	var finalPrompt string
	if promptFile, hasP := flags["-p"]; hasP {
		promptContent, err := na.loadPromptFile(promptFile)
		if err != nil {
			return err
		}
		finalPrompt = promptContent

		// 显示加载的文件信息
		cli := app.GetCLI()
		cli.ShowInfo("📄", fmt.Sprintf("已加载提示词文件: %s", promptFile))
	} else if promptFile, hasPrompt := flags["--prompt"]; hasPrompt {
		promptContent, err := na.loadPromptFile(promptFile)
		if err != nil {
			return err
		}
		finalPrompt = promptContent

		// 显示加载的文件信息
		cli := app.GetCLI()
		cli.ShowInfo("📄", fmt.Sprintf("已加载提示词文件: %s", promptFile))
	} else {
		finalPrompt = userPrompt
	}

	// 如果既没有命令行提示词也没有文件提示词，显示使用说明
	if finalPrompt == "" {
		na.showUsage()
		return fmt.Errorf("未提供有效的提示词")
	}

	return na.handleNovelWriting(ctx, app, finalPrompt)
}

// showUsage 显示novel应用的使用说明
func (na *NovelApp) showUsage() {
	cli := na.GetCLI()
	fmt.Printf("用法: %s [选项] \"指令\"\n", cli.AppName)
	fmt.Println("\n选项:")
	fmt.Println("  -c, --config <path>    指定配置文件路径")
	fmt.Println("  -p, --prompt <file>    指定包含提示词的.md或.txt文件")
	fmt.Println("  -v, --verbose          启用详细输出")
	fmt.Println("  -h, --help             显示帮助信息")

	fmt.Printf("\n示例:\n")
	fmt.Printf("  %s \"主角探索神秘洞穴\"\n", cli.AppName)
	fmt.Printf("  %s --prompt /path/to/story.md\n", cli.AppName)
	fmt.Printf("  %s -p story.md \"继续上一章的情节\"\n", cli.AppName)
	fmt.Printf("  %s --config /path/to/config.yaml -p prompt.txt\n", cli.AppName)
}

// loadPromptFile 加载prompt文件内容
func (na *NovelApp) loadPromptFile(filePath string) (string, error) {
	// 检查文件扩展名
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != ".md" && ext != ".txt" {
		return "", fmt.Errorf("错误：只支持 .md 和 .txt 文件，不支持 %s 文件", ext)
	}

	// 检查文件是否存在
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", fmt.Errorf("错误：文件 %s 不存在", filePath)
	}

	// 读取文件内容
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("错误：读取文件 %s 失败: %w", filePath, err)
	}

	return strings.TrimSpace(string(fileContent)), nil
}

// handleNovelWriting 处理小说续写逻辑
func (na *NovelApp) handleNovelWriting(ctx context.Context, app *App, userPrompt string) error {
	cli := app.GetCLI()
	logger := app.GetLogger()
	config := app.GetConfig()
	queue := app.GetQueue()

	// 显示横幅
	cli.ShowBannerText("小说续写工作流程")
	cli.ShowInfo("📝", fmt.Sprintf("续写指令: %s", userPrompt))
	cli.ShowInfo("🔧", "正在初始化系统组件...")

	// 启动MessageQueue
	if queue != nil {
		if err := queue.Start(ctx); err != nil {
			logger.Warn("MessageQueue启动失败: " + err.Error())
		} else {
			logger.Info("MessageQueue已启动")
		}
	}

	// 获取小说路径
	novelPath, err := config.Novel.GetAbsolutePath()
	if err != nil {
		return fmt.Errorf("获取小说路径失败: %w", err)
	}

	// 创建组件
	llmManager := providers.NewManager(config, *logger)
	contentGenerator := content.NewGenerator(novelPath)           // 完整内容生成器（供planner使用）
	limitedGenerator := content.NewLimitedGenerator(novelPath, 2) // 限制内容生成器（供writer使用，只读最新2章）
	worldviewManager := managers.NewWorldviewManager(novelPath)
	characterManager := managers.NewCharacterManager(novelPath)

	// 创建规划器（使用 gemini-2.5-flash）
	plannerComponent := agents.NewPlanner(&agents.PlannerConfig{
		LLMManager:       llmManager,
		ContentGenerator: contentGenerator,
		Logger:           logger,
		ShowProgress:     na.novelConfig.ShowSteps,
		PlannerModel:     "gemini-2.5-flash", // 固定使用 gemini
	})

	// 创建Writer Graph
	writerGraphBuilder := agents.NewWriterGraphBuilder(&agents.WriterGraphConfig{
		LLMManager:   llmManager,
		Logger:       logger,
		ShowProgress: na.novelConfig.ShowSteps,
	})

	writerGraph, err := writerGraphBuilder.CreateWriterGraphWithRetry(ctx)
	if err != nil {
		return fmt.Errorf("创建 Writer Graph 失败: %w", err)
	}

	// 编译Writer Graph
	compiledWriter, err := writerGraph.Compile(ctx)
	if err != nil {
		return fmt.Errorf("编译 Writer Graph 失败: %w", err)
	}

	cli.ShowSuccess("系统初始化完成")
	cli.ShowInfo("📚", "开始执行续写工作流程...")
	cli.ShowSeparator()

	// 创建小说工作流
	novelWorkflow := workflows.NewNovelWorkflow(&workflows.NovelWorkflowConfig{
		Logger:           logger,
		Generator:        contentGenerator,
		LimitedGenerator: limitedGenerator,
		Planner:          plannerComponent,
		WorldviewManager: worldviewManager,
		CharacterManager: characterManager,
		LLMManager:       llmManager,
		CompiledWriter:   compiledWriter,
		ShowProgress:     na.novelConfig.ShowSteps,
	})

	// 创建并编译工作流
	workflow, err := novelWorkflow.CreateWorkflow()
	if err != nil {
		return fmt.Errorf("创建工作流失败: %w", err)
	}

	// 执行工作流
	result, err := workflow.Invoke(ctx, userPrompt)
	if err != nil {
		return fmt.Errorf("执行工作流失败: %w", err)
	}

	cli.ShowFooterText("续写工作流程完成！")
	cli.ShowSeparator()

	// 保存章节
	if err := na.saveChapter(ctx, result, cli, logger); err != nil {
		return fmt.Errorf("保存章节失败: %w", err)
	}

	// 提交摘要任务到队列
	if queue != nil && result != "" {
		if err := app.EnqueueSummarizeTask(novelPath, result); err != nil {
			logger.Warn("提交摘要任务失败: " + err.Error())
		} else {
			cli.ShowInfo("📋", "已提交摘要生成任务到后台处理")
		}
	}

	// 检测角色更新需求并提交角色更新任务
	if queue != nil && result != "" {
		if err := app.EnqueueCharacterUpdateTask(novelPath, "主角", ""); err != nil {
			logger.Warn("提交角色更新任务失败: " + err.Error())
		} else {
			cli.ShowInfo("👤", fmt.Sprintf("已提交角色 %s 更新任务到后台处理", " 主角 "))
		}
	}

	// 检测世界观更新需求并提交世界观总结任务
	if queue != nil && result != "" {
		if err := app.EnqueueWorldviewSummarizerTask(novelPath, ""); err != nil {
			logger.Warn("提交世界观总结任务失败: " + err.Error())
		} else {
			cli.ShowInfo("🌍", "已提交世界观总结任务到后台处理")
		}
	}

	// 显示完整结果
	cli.ShowResult("完整续写结果", result)

	// 等待队列处理完成并关闭
	if queue != nil {
		cli.ShowInfo("⏳", "等待后台任务完成...")
		queue.WaitUntilComplete()

		if err := queue.Shutdown(5 * time.Second); err != nil {
			logger.Warn("关闭MessageQueue失败: " + err.Error())
		} else {
			logger.Info("MessageQueue已正常关闭")
		}
	}

	return nil
}

// saveChapter 保存章节
func (na *NovelApp) saveChapter(ctx context.Context, chapterContent string, cli *common.CLIHelper, logger *logger.ZapLogger) error {
	cli.ShowStep(4, "保存章节到文件")

	// 从全局配置获取小说路径
	globalConfig := config.GetGlobal()
	novelPath, err := globalConfig.Novel.GetAbsolutePath()
	if err != nil {
		return fmt.Errorf("获取小说路径失败: %w", err)
	}

	// 创建章节管理器
	chapterManager := content.NewChapterManager(novelPath)

	// 写入章节
	chapterPath, chapterInfo, err := chapterManager.WriteChapterWithInfo(chapterContent)
	if err != nil {
		return fmt.Errorf("保存章节失败: %w", err)
	}

	// 显示保存结果
	cli.ShowSuccess("章节保存成功")
	cli.ShowFileInfo(chapterPath, chapterInfo.ChapterID, chapterInfo.Title, len(chapterInfo.Content))

	if logger != nil {
		logger.Info(fmt.Sprintf("章节保存成功: %s", chapterPath))
	}

	return nil
}

// SetNovelDir 已废弃：现在通过config.yaml配置小说目录
// 为了兼容性保留此方法，但不再使用
func (na *NovelApp) SetNovelDir(dir string) {
	// 此方法已废弃，小说目录现在通过配置文件指定
}

// SetShowSteps 设置是否显示步骤
func (na *NovelApp) SetShowSteps(show bool) {
	na.novelConfig.ShowSteps = show
}

// SetEnableRetry 设置是否启用重试
func (na *NovelApp) SetEnableRetry(enable bool) {
	na.novelConfig.EnableRetry = enable
}
