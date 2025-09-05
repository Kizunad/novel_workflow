package cli

import (
	"context"
	"fmt"

	"github.com/Kizunad/modular-workflow-v2/components/workflows"
	"github.com/Kizunad/modular-workflow-v2/providers"
)

// PlanAppConfig 计划应用配置
type PlanAppConfig struct {
	*AppConfig
	ShowSteps   bool
	EnableRetry bool
}

// DefaultPlanAppConfig 默认计划应用配置
func DefaultPlanAppConfig() *PlanAppConfig {
	return &PlanAppConfig{
		AppConfig:   DefaultAppConfig(),
		ShowSteps:   true,
		EnableRetry: true,
	}
}

// PlanApp 计划制定应用
type PlanApp struct {
	*App
	planConfig *PlanAppConfig
}

// NewPlanApp 创建计划制定应用
func NewPlanApp() *PlanApp {
	config := DefaultPlanAppConfig()
	config.Name = "小说规划工具"
	config.Description = "AI驱动的小说章节规划系统"

	return &PlanApp{
		App:        NewApp(config.AppConfig),
		planConfig: config,
	}
}

// Run 运行计划制定应用
func (pa *PlanApp) Run(args []string) error {
	ctx := context.Background()

	// 解析参数和标志
	userInput, flags, err := pa.ParseArgsWithFlags(args)

	// 检查帮助标志
	if _, hasHelp := flags["-h"]; hasHelp {
		pa.showUsage()
		return nil
	}
	if _, hasHelp := flags["--help"]; hasHelp {
		pa.showUsage()
		return nil
	}

	// 如果有-p参数，忽略"参数不足"错误
	if err != nil {
		if _, hasP := flags["-p"]; !hasP {
			if _, hasPrompt := flags["--prompt"]; !hasPrompt {
				pa.showUsage()
				return nil
			}
		}
	}

	return pa.handlePlanningWithFlags(ctx, pa.App, userInput, flags)
}

// handlePlanningWithFlags 处理计划制定逻辑（支持标志参数）
func (pa *PlanApp) handlePlanningWithFlags(ctx context.Context, app *App, userPrompt string, flags map[string]string) error {
	// 检查配置文件标志
	if configPath, ok := flags["--config"]; ok {
		pa.App.config.ConfigPath = configPath
		pa.GetCLI().ShowInfo("🔧", fmt.Sprintf("使用指定配置文件: %s", configPath))
	} else if configPath, ok := flags["-c"]; ok {
		pa.App.config.ConfigPath = configPath
		pa.GetCLI().ShowInfo("🔧", fmt.Sprintf("使用指定配置文件: %s", configPath))
	}

	// 初始化App
	if err := pa.App.Initialize(ctx); err != nil {
		pa.GetCLI().ShowGracefulError("初始化失败", err.Error(), "请检查配置文件是否正确")
		return err
	}

	// 处理 -p/--prompt 参数
	var finalPrompt string
	if promptFile, hasP := flags["-p"]; hasP {
		promptContent, err := pa.loadPromptFile(promptFile)
		if err != nil {
			return err
		}
		finalPrompt = promptContent

		// 显示加载的文件信息
		cli := app.GetCLI()
		cli.ShowInfo("📄", fmt.Sprintf("已加载提示词文件: %s", promptFile))
	} else if promptFile, hasPrompt := flags["--prompt"]; hasPrompt {
		promptContent, err := pa.loadPromptFile(promptFile)
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
		pa.showUsage()
		return fmt.Errorf("未提供有效的提示词")
	}

	return pa.handlePlanning(ctx, app, finalPrompt)
}

// showUsage 显示plan应用的使用说明
func (pa *PlanApp) showUsage() {
	cli := pa.GetCLI()
	fmt.Printf("用法: %s [选项] \"规划需求\"\n", cli.AppName)
	fmt.Println("\n选项:")
	fmt.Println("  -c, --config <path>    指定配置文件路径")
	fmt.Println("  -p, --prompt <file>    指定包含规划需求的.md或.txt文件")
	fmt.Println("  -v, --verbose          启用详细输出")
	fmt.Println("  -h, --help             显示帮助信息")

	fmt.Printf("\n示例:\n")
	fmt.Printf("  %s \"制定第三章的详细规划\"\n", cli.AppName)
	fmt.Printf("  %s --prompt /path/to/planning-requirements.md\n", cli.AppName)
	fmt.Printf("  %s -p requirements.md \"基于现有章节制定后续规划\"\n", cli.AppName)
	fmt.Printf("  %s --config /path/to/config.yaml -p prompt.txt\n", cli.AppName)
}

// loadPromptFile 加载prompt文件内容（使用App的LoadPromptFile方法）
func (pa *PlanApp) loadPromptFile(filePath string) (string, error) {
	return pa.App.LoadPromptFile(filePath)
}

// handlePlanning 处理计划制定逻辑
func (pa *PlanApp) handlePlanning(ctx context.Context, app *App, userPrompt string) error {
	cli := app.GetCLI()
	logger := app.GetLogger()
	config := app.GetConfig()

	// 显示横幅
	cli.ShowBannerText("小说规划工作流程")
	cli.ShowInfo("📋", fmt.Sprintf("规划需求: %s", userPrompt))
	cli.ShowInfo("🔧", "正在初始化系统组件...")

	// 获取小说路径
	novelPath, err := config.Novel.GetAbsolutePath()
	if err != nil {
		return fmt.Errorf("获取小说路径失败: %w", err)
	}

	// 创建组件
	llmManager := providers.NewManager(config, *logger)

	cli.ShowSuccess("系统初始化完成")
	cli.ShowInfo("📚", "开始执行规划工作流程...")
	cli.ShowSeparator()

	// 创建规划工作流
	planWorkflow := workflows.NewPlanWorkflow(&workflows.PlanWorkflowConfig{
		Logger:       logger,
		NovelDir:     novelPath,
		LLMManager:   llmManager,
		ShowProgress: pa.planConfig.ShowSteps,
		PlannerModel: "deepseek-chat",
	})

	result, _ := planWorkflow.ExecuteWithMonitoring(userPrompt)

	cli.ShowFooterText("规划工作流程完成！")
	cli.ShowSeparator()

	// 显示规划结果
	cli.ShowResult("规划结果", result)

	// 显示计划状态信息
	cli.ShowInfo("💡", "规划已保存到 plan.json 文件中")
	cli.ShowInfo("🎯", "可以使用 write 命令根据规划进行创作")

	return nil
}

// SetShowSteps 设置是否显示步骤
func (pa *PlanApp) SetShowSteps(show bool) {
	pa.planConfig.ShowSteps = show
}

// SetEnableRetry 设置是否启用重试
func (pa *PlanApp) SetEnableRetry(enable bool) {
	pa.planConfig.EnableRetry = enable
}
