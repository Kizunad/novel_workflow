package common

import (
	"fmt"
	"os"
	"strings"
)

// CLIHelper CLI辅助工具
type CLIHelper struct {
	AppName     string
	AppDesc     string
	ShowBanner  bool
	ShowFooter  bool
}

// NewCLIHelper 创建CLI辅助工具
func NewCLIHelper(appName, appDesc string) *CLIHelper {
	return &CLIHelper{
		AppName:    appName,
		AppDesc:    appDesc,
		ShowBanner: true,
		ShowFooter: true,
	}
}

// ParseArgs 解析命令行参数
func (c *CLIHelper) ParseArgs(args []string, minArgs int) (string, error) {
	if len(args) < minArgs+1 {
		return "", fmt.Errorf("参数不足")
	}
	return args[1], nil
}

// ShowUsage 显示使用说明
func (c *CLIHelper) ShowUsage(example string) {
	fmt.Printf("用法: %s \"指令\"\n", c.AppName)
	if example != "" {
		fmt.Printf("示例: %s \"%s\"\n", c.AppName, example)
	}
}

// ShowUsageWithFlags 显示使用说明（包含标志）
func (c *CLIHelper) ShowUsageWithFlags(example string) {
	fmt.Printf("用法: %s [选项] \"指令\"\n", c.AppName)
	fmt.Println("\n选项:")
	fmt.Println("  -c, --config <path>    指定配置文件路径")
	fmt.Println("  -v, --verbose          启用详细输出")
	fmt.Println("  -h, --help             显示帮助信息")
	
	if example != "" {
		fmt.Printf("\n示例:\n")
		fmt.Printf("  %s \"%s\"\n", c.AppName, example)
		fmt.Printf("  %s --config /path/to/config.yaml \"%s\"\n", c.AppName, example)
		fmt.Printf("  %s -c config.yaml --verbose \"%s\"\n", c.AppName, example)
	}
}

// ShowBannerText 显示横幅
func (c *CLIHelper) ShowBannerText(title string) {
	if !c.ShowBanner {
		return
	}
	
	fmt.Printf("=== %s ===\n", title)
	if c.AppDesc != "" {
		fmt.Printf("%s\n", c.AppDesc)
	}
}

// ShowStep 显示步骤信息
func (c *CLIHelper) ShowStep(stepNum int, stepDesc string) {
	fmt.Printf("\n📋 第%d步：%s...\n", stepNum, stepDesc)
}

// ShowSuccess 显示成功信息
func (c *CLIHelper) ShowSuccess(message string) {
	fmt.Printf("✅ %s\n", message)
}

// ShowError 显示错误信息
func (c *CLIHelper) ShowError(err error) {
	fmt.Printf("❌ 错误: %v\n", err)
}

// ShowGracefulError 显示友好的错误信息
func (c *CLIHelper) ShowGracefulError(title, message, suggestion string) {
	fmt.Printf("❌ %s\n", title)
	if message != "" {
		fmt.Printf("   %s\n", message)
	}
	if suggestion != "" {
		fmt.Printf("💡 建议: %s\n", suggestion)
	}
}

// ShowProgress 显示进度信息
func (c *CLIHelper) ShowProgress(current, total int, desc string) {
	if total > 0 {
		progress := float64(current) / float64(total) * 100
		fmt.Printf("📊 进度: %.1f%% (%d/%d) %s\n", progress, current, total, desc)
	} else {
		fmt.Printf("📊 %s...\n", desc)
	}
}

// ShowSeparator 显示分隔线
func (c *CLIHelper) ShowSeparator() {
	fmt.Println("─────────────────────────────────────────────────────")
}

// ShowInfo 显示信息
func (c *CLIHelper) ShowInfo(icon, message string) {
	fmt.Printf("%s %s\n", icon, message)
}

// ShowResult 显示结果信息
func (c *CLIHelper) ShowResult(title, content string) {
	fmt.Printf("\n📖 %s:\n", title)
	c.ShowSeparator()
	fmt.Println(content)
	c.ShowSeparator()
}

// ShowFooterText 显示页脚
func (c *CLIHelper) ShowFooterText(message string) {
	if !c.ShowFooter {
		return
	}
	
	fmt.Printf("\n🎉 %s\n", message)
}

// ShowFileInfo 显示文件信息
func (c *CLIHelper) ShowFileInfo(path, id, title string, count int) {
	fmt.Printf("📁 文件路径: %s\n", path)
	if id != "" {
		fmt.Printf("📋 ID: %s\n", id)
	}
	if title != "" {
		fmt.Printf("📖 标题: %s\n", title)
	}
	if count > 0 {
		fmt.Printf("📄 数量: %d 个\n", count)
	}
}

// ShowPreview 显示内容预览
func (c *CLIHelper) ShowPreview(content string, maxLen int) {
	if len(content) > maxLen {
		preview := content[:maxLen] + "..."
		fmt.Printf("📝 内容预览: %s\n", preview)
	} else {
		fmt.Printf("📝 内容预览: %s\n", content)
	}
}

// FormatLength 格式化长度显示
func (c *CLIHelper) FormatLength(length int) string {
	return fmt.Sprintf("%d 字符", length)
}

// TruncateString 截断字符串
func (c *CLIHelper) TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// IsVerbose 检查是否启用详细模式
func (c *CLIHelper) IsVerbose() bool {
	for _, arg := range os.Args {
		if arg == "-v" || arg == "--verbose" {
			return true
		}
	}
	return false
}

// HasFlag 检查是否存在标志
func (c *CLIHelper) HasFlag(flag string) bool {
	for _, arg := range os.Args {
		if arg == flag || strings.HasPrefix(arg, flag+"=") {
			return true
		}
	}
	return false
}

// GetFlagValue 获取标志的值
// 支持 --flag=value 和 --flag value 两种格式
func (c *CLIHelper) GetFlagValue(flag string) (string, bool) {
	args := os.Args
	for i, arg := range args {
		// 检查 --flag=value 格式
		if strings.HasPrefix(arg, flag+"=") {
			return strings.TrimPrefix(arg, flag+"="), true
		}
		// 检查 --flag value 格式
		if arg == flag && i+1 < len(args) {
			return args[i+1], true
		}
	}
	return "", false
}

// ParseArgsWithFlags 解析命令行参数，支持标志提取
func (c *CLIHelper) ParseArgsWithFlags(args []string, minArgs int) (userInput string, flags map[string]string, err error) {
	flags = make(map[string]string)
	var nonFlagArgs []string
	
	i := 1 // 跳过程序名
	for i < len(args) {
		arg := args[i]
		
		// 处理 --flag=value 格式
		if strings.Contains(arg, "=") && strings.HasPrefix(arg, "-") {
			parts := strings.SplitN(arg, "=", 2)
			if len(parts) == 2 {
				flags[parts[0]] = parts[1]
				i++
				continue
			}
		}
		
		// 处理 --flag value 格式
		if strings.HasPrefix(arg, "-") && i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			flags[arg] = args[i+1]
			i += 2
			continue
		}
		
		// 处理单独的标志（无值）
		if strings.HasPrefix(arg, "-") {
			flags[arg] = ""
			i++
			continue
		}
		
		// 非标志参数
		nonFlagArgs = append(nonFlagArgs, arg)
		i++
	}
	
	if len(nonFlagArgs) < minArgs {
		return "", flags, fmt.Errorf("参数不足")
	}
	
	if len(nonFlagArgs) > 0 {
		userInput = nonFlagArgs[0]
	}
	
	return userInput, flags, nil
}