package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kizunad/modular-workflow-v2/components/common"
	"github.com/Kizunad/modular-workflow-v2/config"
	"github.com/Kizunad/modular-workflow-v2/logger"
	"github.com/Kizunad/modular-workflow-v2/providers"
	"github.com/Kizunad/modular-workflow-v2/queue"
)

// AppConfig 应用配置
type AppConfig struct {
	Name        string
	Description string
	ConfigPath  string
	ShowBanner  bool
	ShowFooter  bool
}

// DefaultAppConfig 默认应用配置
func DefaultAppConfig() *AppConfig {
	return &AppConfig{
		Name:        "Modular Workflow App",
		Description: "基于模块化设计的工作流应用",
		ConfigPath:  "../../config.yaml",
		ShowBanner:  true,
		ShowFooter:  true,
	}
}

// App 通用CLI应用框架
type App struct {
	config *AppConfig
	cli    *common.CLIHelper
	logger *logger.ZapLogger
	cfg    *config.Config
	queue  *queue.MessageQueue
}

// NewApp 创建新的应用实例
func NewApp(config *AppConfig) *App {
	if config == nil {
		config = DefaultAppConfig()
	}
	
	return &App{
		config: config,
		cli:    common.NewCLIHelper(config.Name, config.Description),
		logger: logger.New(),
	}
}

// Initialize 初始化应用
func (a *App) Initialize(ctx context.Context) error {
	cfg, err := a.loadConfig()
	if err != nil {
		return err
	}
	a.cfg = cfg
	
	// 初始化全局配置
	if err := config.InitGlobal(a.config.ConfigPath); err != nil {
		return fmt.Errorf("初始化全局配置失败: %w", err)
	}
	
	// 初始化LLM管理器
	llmManager := providers.NewManager(cfg, *a.logger)
	
	// 获取小说目录路径
	novelDir, err := cfg.Novel.GetAbsolutePath()
	if err != nil {
		return fmt.Errorf("获取小说目录路径失败: %w", err)
	}
	
	// 初始化消息队列
	a.queue, err = queue.InitQueue(&cfg.MessageQueue, novelDir, llmManager, a.logger)
	if err != nil {
		return fmt.Errorf("初始化消息队列失败: %w", err)
	}
	
	a.showInitBanner()
	return nil
}

// loadConfig 加载配置文件，包含回退逻辑
func (a *App) loadConfig() (*config.Config, error) {
	cfg, err := config.NewLoader().Load(a.config.ConfigPath)
	if err != nil {
		return a.handleConfigLoadError(err)
	}
	return cfg, nil
}

// handleConfigLoadError 处理配置加载错误
func (a *App) handleConfigLoadError(err error) (*config.Config, error) {
	a.cli.ShowInfo("⚠️", fmt.Sprintf("配置文件 '%s' 加载失败: %v", a.config.ConfigPath, err))
	
	defaultPath := "../../config.yaml"
	if a.config.ConfigPath == defaultPath {
		a.cli.ShowInfo("💡", "请确保配置文件存在，或使用 --config 指定正确的配置文件路径")
		return nil, fmt.Errorf("配置文件加载失败")
	}
	
	return a.tryFallbackConfig(defaultPath)
}

// tryFallbackConfig 尝试使用默认配置文件
func (a *App) tryFallbackConfig(defaultPath string) (*config.Config, error) {
	a.cli.ShowInfo("🔄", fmt.Sprintf("尝试使用默认配置文件: %s", defaultPath))
	
	cfg, err := config.NewLoader().Load(defaultPath)
	if err != nil {
		a.cli.ShowInfo("❌", fmt.Sprintf("默认配置文件也无法加载: %v", err))
		a.cli.ShowInfo("💡", "请确保配置文件存在，或使用 --config 指定正确的配置文件路径")
		return nil, fmt.Errorf("配置文件加载失败")
	}
	
	a.cli.ShowInfo("✅", "已使用默认配置文件")
	return cfg, nil
}

// showInitBanner 显示初始化横幅
func (a *App) showInitBanner() {
	if a.config.ShowBanner {
		a.cli.ShowBannerText("系统初始化")
		a.cli.ShowInfo("🔧", "正在初始化系统组件...")
	}
}

// ParseArgs 解析命令行参数
func (a *App) ParseArgs(args []string) (string, error) {
	return a.cli.ParseArgs(args, 1)
}

// ParseArgsWithFlags 解析命令行参数，支持配置文件标志
func (a *App) ParseArgsWithFlags(args []string) (string, map[string]string, error) {
	return a.cli.ParseArgsWithFlags(args, 1)
}

// ShowUsage 显示使用说明
func (a *App) ShowUsage(example string) {
	a.cli.ShowUsage(example)
}

// ShowUsageWithFlags 显示使用说明（包含标志）
func (a *App) ShowUsageWithFlags(example string) {
	a.cli.ShowUsageWithFlags(example)
}

// ShowError 显示错误信息
func (a *App) ShowError(err error) {
	a.cli.ShowError(err)
	if a.logger != nil {
		a.logger.Error("应用执行失败: " + err.Error())
	}
}

// ShowSuccess 显示成功信息
func (a *App) ShowSuccess(message string) {
	if a.config.ShowFooter {
		a.cli.ShowFooterText(message)
	}
}

// GetConfig 获取配置
func (a *App) GetConfig() *config.Config {
	return a.cfg
}

// GetLogger 获取日志记录器
func (a *App) GetLogger() *logger.ZapLogger {
	return a.logger
}

// GetCLI 获取CLI辅助工具
func (a *App) GetCLI() *common.CLIHelper {
	return a.cli
}

// GetQueue 获取消息队列
func (a *App) GetQueue() *queue.MessageQueue {
	return a.queue
}

// EnqueueSummarizeTask 提交摘要任务到队列
func (a *App) EnqueueSummarizeTask(novelPath, content string) error {
	if a.queue == nil {
		// 队列未启用，直接跳过
		return nil
	}
	
	task := queue.CreateSummarizeTask(novelPath, content)
	return a.queue.Enqueue(task)
}

// EnqueueCharacterUpdateTask 提交角色更新任务到队列
func (a *App) EnqueueCharacterUpdateTask(novelPath, characterName, updateContent string) error {
	if a.queue == nil {
		// 队列未启用，直接跳过
		return nil
	}
	
	task := queue.CreateCharacterUpdateTask(novelPath, characterName, updateContent)
	return a.queue.Enqueue(task)
}

// EnqueueWorldviewSummarizerTask 提交世界观总结任务到队列
func (a *App) EnqueueWorldviewSummarizerTask(novelPath, updateContent string) error {
	if a.queue == nil {
		// 队列未启用，直接跳过
		return nil
	}
	
	task := queue.CreateWorldviewSummarizerTask(novelPath, updateContent)
	return a.queue.Enqueue(task)
}

// Run 运行应用的通用框架
func (a *App) Run(args []string, handler func(ctx context.Context, app *App, userInput string) error) error {
	return a.RunWithFlags(args, func(ctx context.Context, app *App, userInput string, flags map[string]string) error {
		return handler(ctx, app, userInput)
	})
}

// RunWithFlags 运行应用的通用框架，支持标志解析
func (a *App) RunWithFlags(args []string, handler func(ctx context.Context, app *App, userInput string, flags map[string]string) error) error {
	ctx := context.Background()
	
	// 解析参数和标志
	userInput, flags, err := a.ParseArgsWithFlags(args)
	
	// 检查帮助标志
	if _, hasHelp := flags["-h"]; hasHelp {
		a.ShowUsageWithFlags("主角运用编程思维分析异世界魔法规律")
		return nil
	}
	if _, hasHelp := flags["--help"]; hasHelp {
		a.ShowUsageWithFlags("主角运用编程思维分析异世界魔法规律")
		return nil
	}
	
	if err != nil {
		a.ShowUsageWithFlags("主角运用编程思维分析异世界魔法规律")
		return nil // 不返回错误，避免重复显示
	}
	
	// 检查配置文件标志
	if configPath, ok := flags["--config"]; ok {
		a.config.ConfigPath = configPath
		if a.config.ShowBanner {
			a.cli.ShowInfo("🔧", fmt.Sprintf("使用指定配置文件: %s", configPath))
		}
	} else if configPath, ok := flags["-c"]; ok {
		a.config.ConfigPath = configPath
		if a.config.ShowBanner {
			a.cli.ShowInfo("🔧", fmt.Sprintf("使用指定配置文件: %s", configPath))
		}
	}
	
	// 初始化应用
	if err := a.Initialize(ctx); err != nil {
		a.cli.ShowGracefulError("初始化失败", err.Error(), "请检查配置文件是否正确")
		return nil // 友好退出
	}
	
	// 执行处理逻辑
	if err := handler(ctx, a, userInput, flags); err != nil {
		a.cli.ShowGracefulError("执行失败", err.Error(), "请检查输入参数或配置")
		return nil // 友好退出
	}
	
	return nil
}

// IsVerboseMode 检查是否为详细模式
func (a *App) IsVerboseMode() bool {
	return a.cli.IsVerbose()
}

// HasFlag 检查是否存在指定标志
func (a *App) HasFlag(flag string) bool {
	return a.cli.HasFlag(flag)
}

// Exit 退出应用
func (a *App) Exit(code int) {
	os.Exit(code)
}

// LoadPromptFile 加载prompt文件内容
func (a *App) LoadPromptFile(filePath string) (string, error) {
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