package cli

import (
	"context"
	"fmt"

	"github.com/Kizunad/modular-workflow-v2/components/workflows"
	"github.com/Kizunad/modular-workflow-v2/providers"
)

// WriteAppConfig 写作应用配置
type WriteAppConfig struct {
	*AppConfig
	ShowSteps   bool
	EnableRetry bool
}

// DefaultWriteAppConfig 默认写作应用配置
func DefaultWriteAppConfig() *WriteAppConfig {
	return &WriteAppConfig{
		AppConfig:   DefaultAppConfig(),
		ShowSteps:   true,
		EnableRetry: true,
	}
}

// WriteApp 写作应用
type WriteApp struct {
	*App
	writeConfig *WriteAppConfig
}

// NewWriteApp 创建写作应用
func NewWriteApp() *WriteApp {
	config := DefaultWriteAppConfig()
	config.Name = "小说创作工具"
	config.Description = "AI驱动的小说章节创作系统"

	return &WriteApp{
		App:         NewApp(config.AppConfig),
		writeConfig: config,
	}
}

// Run 运行写作应用
func (wa *WriteApp) Run(args []string) error {
	ctx := context.Background()

	// 解析参数和标志
	userInput, flags, err := wa.ParseArgsWithFlags(args)

	// 检查帮助标志
	if _, hasHelp := flags["-h"]; hasHelp {
		wa.showUsage()
		return nil
	}
	if _, hasHelp := flags["--help"]; hasHelp {
		wa.showUsage()
		return nil
	}

	// 如果有-p参数，忽略"参数不足"错误
	if err != nil {
		if _, hasP := flags["-p"]; !hasP {
			if _, hasPrompt := flags["--prompt"]; !hasPrompt {
				wa.showUsage()
				return nil
			}
		}
	}

	return wa.handleWritingWithFlags(ctx, wa.App, userInput, flags)
}

// handleWritingWithFlags 处理写作逻辑（支持标志参数）
func (wa *WriteApp) handleWritingWithFlags(ctx context.Context, app *App, userPrompt string, flags map[string]string) error {
	// 检查配置文件标志
	if configPath, ok := flags["--config"]; ok {
		wa.App.config.ConfigPath = configPath
		wa.GetCLI().ShowInfo("🔧", fmt.Sprintf("使用指定配置文件: %s", configPath))
	} else if configPath, ok := flags["-c"]; ok {
		wa.App.config.ConfigPath = configPath
		wa.GetCLI().ShowInfo("🔧", fmt.Sprintf("使用指定配置文件: %s", configPath))
	}

	// 初始化App
	if err := wa.App.Initialize(ctx); err != nil {
		wa.GetCLI().ShowGracefulError("初始化失败", err.Error(), "请检查配置文件是否正确")
		return err
	}

	// 处理 -p/--prompt 参数
	var finalPrompt string
	if promptFile, hasP := flags["-p"]; hasP {
		promptContent, err := wa.loadPromptFile(promptFile)
		if err != nil {
			return err
		}
		finalPrompt = promptContent

		// 显示加载的文件信息
		cli := app.GetCLI()
		cli.ShowInfo("📄", fmt.Sprintf("已加载提示词文件: %s", promptFile))
	} else if promptFile, hasPrompt := flags["--prompt"]; hasPrompt {
		promptContent, err := wa.loadPromptFile(promptFile)
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
		wa.showUsage()
		return fmt.Errorf("未提供有效的提示词")
	}

	return wa.handleWriting(ctx, app, finalPrompt)
}

// showUsage 显示write应用的使用说明
func (wa *WriteApp) showUsage() {
	cli := wa.GetCLI()
	fmt.Printf("用法: %s [选项] \"创作需求\"\n", cli.AppName)
	fmt.Println("\n选项:")
	fmt.Println("  -c, --config <path>    指定配置文件路径")
	fmt.Println("  -p, --prompt <file>    指定包含创作需求的.md或.txt文件")
	fmt.Println("  -v, --verbose          启用详细输出")
	fmt.Println("  -h, --help             显示帮助信息")

	fmt.Printf("\n示例:\n")
	fmt.Printf("  %s \"根据现有规划创作下一章\"\n", cli.AppName)
	fmt.Printf("  %s --prompt /path/to/writing-requirements.md\n", cli.AppName)
	fmt.Printf("  %s -p requirements.md \"基于规划创作精彩内容\"\n", cli.AppName)
	fmt.Printf("  %s --config /path/to/config.yaml -p prompt.txt\n", cli.AppName)
}

// loadPromptFile 加载prompt文件内容（使用App的LoadPromptFile方法）
func (wa *WriteApp) loadPromptFile(filePath string) (string, error) {
	return wa.App.LoadPromptFile(filePath)
}

// handleWriting 处理写作逻辑
func (wa *WriteApp) handleWriting(ctx context.Context, app *App, userPrompt string) error {
	cli := app.GetCLI()
	logger := app.GetLogger()
	config := app.GetConfig()

	// 显示横幅
	cli.ShowBannerText("小说创作工作流程")
	cli.ShowInfo("✍️", fmt.Sprintf("创作需求: %s", userPrompt))
	cli.ShowInfo("🔧", "正在初始化系统组件...")

	// 获取小说路径
	novelPath, err := config.Novel.GetAbsolutePath()
	if err != nil {
		return fmt.Errorf("获取小说路径失败: %w", err)
	}

	// 创建组件
	llmManager := providers.NewManager(config, *logger)

	cli.ShowSuccess("系统初始化完成")
	cli.ShowInfo("✍️", "开始执行创作工作流程...")
	cli.ShowSeparator()

	// 创建写作工作流
	writeWorkflow := workflows.NewWriteWorkflow(&workflows.WriteWorkflowConfig{
		Logger:       logger,
		NovelDir:     novelPath,
		LLMManager:   llmManager,
		ShowProgress: wa.writeConfig.ShowSteps,
		WriterModel:  "deepseek-chat",
	})

	// 创建并编译工作流
	result, err := writeWorkflow.ExecuteWithMonitoring(userPrompt)

	if err != nil {
		cli.ShowError(err)
		return nil
	}
	cli.ShowFooterText("创作工作流程完成！")
	cli.ShowSeparator()

	// 显示创作结果
	cli.ShowResult("创作结果", result)

	// 显示创作状态信息
	cli.ShowInfo("💡", "创作内容已保存到章节文件中")
	cli.ShowInfo("🎯", "可以继续使用 write 命令创作更多章节")

	return nil
}

// SetShowSteps 设置是否显示步骤
func (wa *WriteApp) SetShowSteps(show bool) {
	wa.writeConfig.ShowSteps = show
}

// SetEnableRetry 设置是否启用重试
func (wa *WriteApp) SetEnableRetry(enable bool) {
	wa.writeConfig.EnableRetry = enable
}
