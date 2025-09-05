package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/Kizunad/modular-workflow-v2/components/common"
	"github.com/Kizunad/modular-workflow-v2/providers"
	"github.com/Kizunad/modular-workflow-v2/queue"
)

// SummeryAppConfig 摘要应用配置
type SummeryAppConfig struct {
	*AppConfig
	ShowSteps   bool
	EnableRetry bool
}

// DefaultSummeryAppConfig 默认摘要应用配置
func DefaultSummeryAppConfig() *SummeryAppConfig {
	return &SummeryAppConfig{
		AppConfig:   DefaultAppConfig(),
		ShowSteps:   true,
		EnableRetry: true,
	}
}

// SummeryApp 摘要应用
type SummeryApp struct {
	*App
	summeryConfig *SummeryAppConfig
}

// NewSummeryApp 创建摘要应用
func NewSummeryApp() *SummeryApp {
	config := DefaultSummeryAppConfig()
	config.Name = "小说摘要工具"
	config.Description = "AI驱动的小说章节摘要生成系统"

	return &SummeryApp{
		App:           NewApp(config.AppConfig),
		summeryConfig: config,
	}
}

// Run 运行摘要应用
func (sa *SummeryApp) Run(args []string) error {
	ctx := context.Background()

	// 解析参数和标志
	userInput, flags, err := sa.ParseArgsWithFlags(args)

	// 检查帮助标志
	if _, hasHelp := flags["-h"]; hasHelp {
		sa.showUsage()
		return nil
	}
	if _, hasHelp := flags["--help"]; hasHelp {
		sa.showUsage()
		return nil
	}

	// 检查摘要类型标志
	summaryType := "content" // 默认为内容摘要
	if _, hasID := flags["-i"]; hasID {
		summaryType = "id"
	} else if _, hasID := flags["--id"]; hasID {
		summaryType = "id"
	} else if _, hasLatest := flags["-l"]; hasLatest {
		summaryType = "latest"
	} else if _, hasLatest := flags["--latest"]; hasLatest {
		summaryType = "latest"
	} else if _, hasWorldview := flags["-w"]; hasWorldview {
		summaryType = "worldview"
	} else if _, hasWorldview := flags["--worldview"]; hasWorldview {
		summaryType = "worldview"
	} else if _, hasCharacter := flags["-r"]; hasCharacter {
		summaryType = "character"
	} else if _, hasCharacter := flags["--character"]; hasCharacter {
		summaryType = "character"
	} else if _, hasChapter := flags["--chapter"]; hasChapter {
		summaryType = "chapter"
	} else if _, hasAll := flags["--all"]; hasAll {
		summaryType = "all"
	}

	// 对于特定摘要类型或-p参数，忽略"参数不足"错误
	if err != nil {
		if _, hasP := flags["-p"]; !hasP {
			if _, hasPrompt := flags["--prompt"]; !hasPrompt {
				// 这些摘要类型可以不需要用户输入参数
				if summaryType != "latest" && summaryType != "worldview" && summaryType != "character" && summaryType != "all" {
					sa.showUsage()
					return nil
				}
			}
		}
	}

	return sa.handleSummeryWithFlags(ctx, sa.App, userInput, flags, summaryType)
}

// handleSummeryWithFlags 处理摘要逻辑（支持标志参数）
func (sa *SummeryApp) handleSummeryWithFlags(ctx context.Context, app *App, userPrompt string, flags map[string]string, summaryType string) error {
	// 检查配置文件标志
	if configPath, ok := flags["--config"]; ok {
		sa.App.config.ConfigPath = configPath
		sa.GetCLI().ShowInfo("🔧", fmt.Sprintf("使用指定配置文件: %s", configPath))
	} else if configPath, ok := flags["-c"]; ok {
		sa.App.config.ConfigPath = configPath
		sa.GetCLI().ShowInfo("🔧", fmt.Sprintf("使用指定配置文件: %s", configPath))
	}

	// 初始化App
	if err := sa.App.Initialize(ctx); err != nil {
		sa.GetCLI().ShowGracefulError("初始化失败", err.Error(), "请检查配置文件是否正确")
		return err
	}

	// 检查novel-dir标志，在初始化后设置
	if novelDir, ok := flags["--novel-dir"]; ok {
		if sa.App.cfg != nil {
			sa.App.cfg.Novel.Path = novelDir
			sa.GetCLI().ShowInfo("📂", fmt.Sprintf("使用指定小说目录: %s", novelDir))
		}
	}

	// 处理不同的摘要类型
	switch summaryType {
	case "content":
		return sa.handleContentSummary(ctx, app, userPrompt, flags)
	case "id":
		return sa.handleIDSummary(ctx, app, userPrompt, flags)
	case "latest":
		return sa.handleLatestSummary(ctx, app, flags)
	case "worldview":
		return sa.handleWorldviewSummary(ctx, app, userPrompt, flags)
	case "character":
		return sa.handleCharacterSummary(ctx, app, userPrompt, flags)
	case "chapter":
		return sa.handleChapterSummary(ctx, app, userPrompt, flags)
	case "all":
		return sa.handleAllAgents(ctx, app, flags)
	default:
		return fmt.Errorf("不支持的摘要类型: %s", summaryType)
	}
}

// handleContentSummary 处理章节内容摘要
func (sa *SummeryApp) handleContentSummary(ctx context.Context, app *App, userPrompt string, flags map[string]string) error {
	cli := app.GetCLI()

	// 处理 -p/--prompt 参数
	var finalContent string
	if promptFile, hasP := flags["-p"]; hasP {
		promptContent, err := sa.loadPromptFile(promptFile)
		if err != nil {
			return err
		}
		finalContent = promptContent
		cli.ShowInfo("📄", fmt.Sprintf("已加载章节内容文件: %s", promptFile))
	} else if promptFile, hasPrompt := flags["--prompt"]; hasPrompt {
		promptContent, err := sa.loadPromptFile(promptFile)
		if err != nil {
			return err
		}
		finalContent = promptContent
		cli.ShowInfo("📄", fmt.Sprintf("已加载章节内容文件: %s", promptFile))
	} else {
		finalContent = userPrompt
	}

	// 如果没有内容，显示使用说明
	if finalContent == "" {
		sa.showUsage()
		return fmt.Errorf("未提供章节内容")
	}

	return sa.handleSummary(ctx, app, "章节内容摘要", func(mq *queue.MessageQueue) error {
		taskID := fmt.Sprintf("summary-content-%d", time.Now().Unix())
		task := queue.CreateSummarizeTask(taskID, finalContent)
		return mq.Enqueue(task)
	})
}

// handleIDSummary 处理通过章节ID摘要
func (sa *SummeryApp) handleIDSummary(ctx context.Context, app *App, userPrompt string, flags map[string]string) error {
	// 获取章节ID
	var chapterID string
	if idValue, hasID := flags["-i"]; hasID {
		chapterID = idValue
	} else if idValue, hasID := flags["--id"]; hasID {
		chapterID = idValue
	} else if userPrompt != "" {
		chapterID = userPrompt
	}

	if chapterID == "" {
		sa.showUsage()
		return fmt.Errorf("未提供章节ID")
	}

	return sa.handleSummary(ctx, app, fmt.Sprintf("章节ID摘要 (ID: %s)", chapterID), func(mq *queue.MessageQueue) error {
		taskID := fmt.Sprintf("summary-id-%s-%d", chapterID, time.Now().Unix())
		task := queue.CreateSummarizeByIDTask(taskID, chapterID)
		return mq.Enqueue(task)
	})
}

// handleLatestSummary 处理最新章节摘要
func (sa *SummeryApp) handleLatestSummary(ctx context.Context, app *App, flags map[string]string) error {
	return sa.handleSummary(ctx, app, "最新章节摘要", func(mq *queue.MessageQueue) error {
		taskID := fmt.Sprintf("summary-latest-%d", time.Now().Unix())
		task := queue.CreateLatestChapterSummarizeTask(taskID)
		return mq.Enqueue(task)
	})
}

// handleWorldviewSummary 处理世界观总结摘要
func (sa *SummeryApp) handleWorldviewSummary(ctx context.Context, app *App, userPrompt string, flags map[string]string) error {
	cli := app.GetCLI()

	// 处理 -p/--prompt 参数或用户输入
	var updateContent string
	if promptFile, hasP := flags["-p"]; hasP {
		promptContent, err := sa.loadPromptFile(promptFile)
		if err != nil {
			return err
		}
		updateContent = promptContent
		cli.ShowInfo("📄", fmt.Sprintf("已加载世界观更新内容文件: %s", promptFile))
	} else if promptFile, hasPrompt := flags["--prompt"]; hasPrompt {
		promptContent, err := sa.loadPromptFile(promptFile)
		if err != nil {
			return err
		}
		updateContent = promptContent
		cli.ShowInfo("📄", fmt.Sprintf("已加载世界观更新内容文件: %s", promptFile))
	} else if worldviewContent, hasWorldview := flags["--worldview"]; hasWorldview {
		// 从 --worldview 标志获取内容
		updateContent = worldviewContent
	} else if worldviewContent, hasW := flags["-w"]; hasW {
		// 从 -w 标志获取内容
		updateContent = worldviewContent
	} else {
		// 最后才使用 userPrompt
		updateContent = userPrompt
	}

	// 如果没有内容，使用AI分析模式
	if updateContent == "" {
		cli.ShowInfo("🤖", "使用AI分析模式，将分析最新章节中的世界观信息")
		return sa.handleSummary(ctx, app, "世界观AI分析", func(mq *queue.MessageQueue) error {
			taskID := fmt.Sprintf("worldview-analysis-%d", time.Now().Unix())
			task := queue.CreateWorldviewAnalysisTask(taskID)
			return mq.Enqueue(task)
		})
	}

	return sa.handleSummary(ctx, app, "世界观总结", func(mq *queue.MessageQueue) error {
		taskID := fmt.Sprintf("worldview-update-%d", time.Now().Unix())
		task := queue.CreateWorldviewSummarizerTask(taskID, updateContent)
		return mq.Enqueue(task)
	})
}

// handleCharacterSummary 处理角色更新摘要
func (sa *SummeryApp) handleCharacterSummary(ctx context.Context, app *App, userPrompt string, flags map[string]string) error {
	cli := app.GetCLI()

	// 获取角色名称
	var characterName string
	if charValue, hasChar := flags["-r"]; hasChar {
		characterName = charValue
	} else if charValue, hasChar := flags["--character"]; hasChar {
		characterName = charValue
	} else if userPrompt != "" {
		// 如果用户输入不为空，将其作为角色名称
		characterName = userPrompt
	}

	if characterName == "" {
		sa.showUsage()
		return fmt.Errorf("未提供角色名称")
	}

	// 处理 -p/--prompt 参数作为更新内容
	var updateContent string
	if promptFile, hasP := flags["-p"]; hasP {
		promptContent, err := sa.loadPromptFile(promptFile)
		if err != nil {
			return err
		}
		updateContent = promptContent
		cli.ShowInfo("📄", fmt.Sprintf("已加载角色更新内容文件: %s", promptFile))
	} else if promptFile, hasPrompt := flags["--prompt"]; hasPrompt {
		promptContent, err := sa.loadPromptFile(promptFile)
		if err != nil {
			return err
		}
		updateContent = promptContent
		cli.ShowInfo("📄", fmt.Sprintf("已加载角色更新内容文件: %s", promptFile))
	}

	return sa.handleSummary(ctx, app, fmt.Sprintf("角色更新 (%s)", characterName), func(mq *queue.MessageQueue) error {
		taskID := fmt.Sprintf("character-update-%s-%d", characterName, time.Now().Unix())
		task := queue.CreateCharacterUpdateTask(taskID, characterName, updateContent)
		return mq.Enqueue(task)
	})
}

// handleChapterSummary 处理章节分析摘要
func (sa *SummeryApp) handleChapterSummary(ctx context.Context, app *App, userPrompt string, flags map[string]string) error {
	cli := app.GetCLI()

	// 处理 -p/--prompt 参数
	var chapterContent string
	if promptFile, hasP := flags["-p"]; hasP {
		promptContent, err := sa.loadPromptFile(promptFile)
		if err != nil {
			return err
		}
		chapterContent = promptContent
		cli.ShowInfo("📄", fmt.Sprintf("已加载章节内容文件: %s", promptFile))
	} else if promptFile, hasPrompt := flags["--prompt"]; hasPrompt {
		promptContent, err := sa.loadPromptFile(promptFile)
		if err != nil {
			return err
		}
		chapterContent = promptContent
		cli.ShowInfo("📄", fmt.Sprintf("已加载章节内容文件: %s", promptFile))
	} else {
		chapterContent = userPrompt
	}

	// 如果没有内容，使用最新章节
	if chapterContent == "" {
		cli.ShowInfo("🔍", "未提供章节内容，将分析最新章节")
		return sa.handleLatestSummary(ctx, app, flags)
	}

	return sa.handleSummary(ctx, app, "章节分析", func(mq *queue.MessageQueue) error {
		taskID := fmt.Sprintf("chapter-analysis-%d", time.Now().Unix())
		task := queue.CreateSummarizeTask(taskID, chapterContent)
		return mq.Enqueue(task)
	})
}

// handleAllAgents 处理全部agents的执行
func (sa *SummeryApp) handleAllAgents(ctx context.Context, app *App, flags map[string]string) error {
	return sa.handleSummary(ctx, app, "全部Agent更新 (摘要+角色+世界观)", func(mq *queue.MessageQueue) error {
		// 按顺序提交三个任务到队列
		baseTime := time.Now().Unix()
		
		// 1. 摘要任务 - 分析最新章节
		summarizeTaskID := fmt.Sprintf("all-summarize-%d", baseTime)
		summarizeTask := queue.CreateLatestChapterSummarizeTask(summarizeTaskID)
		if err := mq.Enqueue(summarizeTask); err != nil {
			return fmt.Errorf("提交摘要任务失败: %w", err)
		}
		
		// 2. 角色更新任务 - AI分析主要角色变化
		characterTaskID := fmt.Sprintf("all-character-%d", baseTime+1)
		characterTask := queue.CreateCharacterUpdateTask(characterTaskID, "主角", "") // 空内容表示AI分析模式
		if err := mq.Enqueue(characterTask); err != nil {
			return fmt.Errorf("提交角色更新任务失败: %w", err)
		}
		
		// 3. 世界观分析任务 - AI分析世界观变化
		worldviewTaskID := fmt.Sprintf("all-worldview-%d", baseTime+2)
		worldviewTask := queue.CreateWorldviewAnalysisTask(worldviewTaskID)
		if err := mq.Enqueue(worldviewTask); err != nil {
			return fmt.Errorf("提交世界观分析任务失败: %w", err)
		}
		
		return nil
	})
}

// handleSummary 处理摘要逻辑通用函数
func (sa *SummeryApp) handleSummary(ctx context.Context, app *App, summaryTitle string, enqueueFunc func(*queue.MessageQueue) error) error {
	cli := app.GetCLI()
	logger := app.GetLogger()
	config := app.GetConfig()

	// 显示横幅
	cli.ShowBannerText("小说摘要工作流程")
	cli.ShowInfo("📋", fmt.Sprintf("摘要类型: %s", summaryTitle))
	cli.ShowInfo("🔧", "正在初始化系统组件...")

	// 获取小说路径
	novelPath, err := config.Novel.GetAbsolutePath()
	if err != nil {
		return fmt.Errorf("获取小说路径失败: %w", err)
	}

	// 创建组件
	llmManager := providers.NewManager(config, *logger)

	// 初始化消息队列
	mq, err := queue.InitQueue(&config.MessageQueue, novelPath, llmManager, logger)
	if err != nil {
		return fmt.Errorf("初始化消息队列失败: %w", err)
	}

	if mq == nil {
		return fmt.Errorf("消息队列被禁用，请在配置文件中启用")
	}

	// 启动消息队列
	if err := mq.Start(ctx); err != nil {
		return fmt.Errorf("启动消息队列失败: %w", err)
	}
	defer mq.Shutdown(30 * time.Second)

	cli.ShowSuccess("系统初始化完成")
	cli.ShowInfo("📊", "开始执行摘要任务...")
	cli.ShowSeparator()

	// 提交任务到队列
	if err := enqueueFunc(mq); err != nil {
		return fmt.Errorf("提交摘要任务失败: %w", err)
	}

	cli.ShowInfo("✅", "摘要任务已提交到队列")
	cli.ShowInfo("⏳", "正在处理摘要任务，请稍候...")

	// 等待任务完成
	sa.waitForCompletion(cli, mq)

	cli.ShowFooterText("摘要工作流程完成！")
	cli.ShowSeparator()

	// 显示结果信息
	cli.ShowInfo("💡", "摘要已生成并保存到索引文件中")
	cli.ShowInfo("📋", "可以查看 index.json 文件获取摘要结果")

	return nil
}

// waitForCompletion 等待任务完成
func (sa *SummeryApp) waitForCompletion(cli *common.CLIHelper, mq *queue.MessageQueue) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			status := mq.GetStatus()
			
			if status.PendingTasks == 0 && status.ProcessingTasks == 0 {
				cli.ShowInfo("✅", "所有任务处理完成")
				return
			}
			
			if sa.summeryConfig.ShowSteps {
				cli.ShowInfo("📊", fmt.Sprintf("队列状态 - 待处理: %d, 处理中: %d, 已完成: %d", 
					status.PendingTasks, status.ProcessingTasks, status.CompletedTasks))
			}
		}
	}
}

// showUsage 显示summery应用的使用说明
func (sa *SummeryApp) showUsage() {
	cli := sa.GetCLI()
	fmt.Printf("用法: %s [选项] [内容/章节ID/角色名称]\n", cli.AppName)
	fmt.Println("\n摘要类型:")
	fmt.Println("  默认              章节内容摘要 - 为提供的章节内容生成摘要")
	fmt.Println("  -i, --id          章节ID摘要 - 通过章节ID生成摘要")
	fmt.Println("  -l, --latest      最新章节摘要 - 为最新章节生成摘要")
	fmt.Println("  -w, --worldview   世界观总结 - 分析或更新世界观设定")
	fmt.Println("  -r, --character   角色更新 - 分析或更新角色信息")
	fmt.Println("  --chapter         章节分析 - 深度分析章节结构和内容")
	fmt.Println("  --all             全部更新 - 执行摘要+角色+世界观三个任务")

	fmt.Println("\n选项:")
	fmt.Println("  -c, --config <path>    指定配置文件路径")
	fmt.Println("  -p, --prompt <file>    指定包含内容的.md或.txt文件")
	fmt.Println("  -v, --verbose          启用详细输出")
	fmt.Println("  -h, --help             显示帮助信息")

	fmt.Printf("\n示例:\n")
	fmt.Printf("  %s \"章节内容...\"                           # 为指定内容生成摘要\n", cli.AppName)
	fmt.Printf("  %s --prompt chapter.md                       # 为文件中的内容生成摘要\n", cli.AppName)
	fmt.Printf("  %s --id \"第三章\"                           # 为指定章节ID生成摘要\n", cli.AppName)
	fmt.Printf("  %s --latest                                  # 为最新章节生成摘要\n", cli.AppName)
	fmt.Printf("  %s --worldview                               # AI分析最新章节的世界观变化\n", cli.AppName)
	fmt.Printf("  %s --worldview \"新的魔法系统设定...\"        # 直接更新世界观设定\n", cli.AppName)
	fmt.Printf("  %s --character \"主角\"                      # 分析主角的状态变化\n", cli.AppName)
	fmt.Printf("  %s --character \"主角\" --prompt updates.md  # 用文件内容更新角色信息\n", cli.AppName)
	fmt.Printf("  %s --chapter --prompt chapter.md             # 深度分析章节结构\n", cli.AppName)
	fmt.Printf("  %s --all                                     # 执行全部更新（摘要+角色+世界观）\n", cli.AppName)
	fmt.Printf("  %s --config config.yaml --latest             # 使用指定配置为最新章节生成摘要\n", cli.AppName)
}

// loadPromptFile 加载prompt文件内容（使用App的LoadPromptFile方法）
func (sa *SummeryApp) loadPromptFile(filePath string) (string, error) {
	return sa.App.LoadPromptFile(filePath)
}

// SetShowSteps 设置是否显示步骤
func (sa *SummeryApp) SetShowSteps(show bool) {
	sa.summeryConfig.ShowSteps = show
}

// SetEnableRetry 设置是否启用重试
func (sa *SummeryApp) SetEnableRetry(enable bool) {
	sa.summeryConfig.EnableRetry = enable
}