package config

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

// Config 主配置结构
type Config struct {
	LLM          LLMConfig          `yaml:"llm" mapstructure:"llm"`
	Vector       VectorConfig       `yaml:"vector" mapstructure:"vector"`
	App          AppConfig          `yaml:"app" mapstructure:"app"`
	Novel        NovelConfig        `yaml:"novel" mapstructure:"novel"`
	MessageQueue MessageQueueConfig `yaml:"message_queue" mapstructure:"message_queue"`
}

// LLMConfig LLM配置
type LLMConfig struct {
	Ollama  OllamaConfig  `yaml:"ollama" mapstructure:"ollama"`
	OpenAI  OpenAIConfig  `yaml:"openai" mapstructure:"openai"`
	Timeout time.Duration `yaml:"timeout" mapstructure:"timeout"`
}

// OllamaConfig Ollama配置
type OllamaConfig struct {
	BaseURL string   `yaml:"base_url" mapstructure:"base_url"`
	Models  []string `yaml:"models" mapstructure:"models"`
}

// OpenAIConfig OpenAI配置
type OpenAIConfig struct {
	BaseURL string   `yaml:"base_url" mapstructure:"base_url"`
	APIKey  string   `yaml:"api_key" mapstructure:"api_key"`
	Models  []string `yaml:"models" mapstructure:"models"`
}

// VectorConfig 向量数据库配置
type VectorConfig struct {
	Type     string `yaml:"type" mapstructure:"type"`
	Endpoint string `yaml:"endpoint" mapstructure:"endpoint"`
	Timeout  string `yaml:"timeout" mapstructure:"timeout"`
}

// AppConfig 应用配置
type AppConfig struct {
	Name    string `yaml:"name" mapstructure:"name"`
	Version string `yaml:"version" mapstructure:"version"`
	Port    int    `yaml:"port" mapstructure:"port"`
}

// NovelConfig 小说配置
type NovelConfig struct {
	Path         string        `yaml:"path" mapstructure:"path"`
	Content      ContentConfig `yaml:"content" mapstructure:"content"`
}

// ContentConfig 内容管理配置
type ContentConfig struct {
	// Token配置
	MaxTokens        int                  `yaml:"max_tokens" mapstructure:"max_tokens"`
	TokenPercentages TokenPercentageConfig `yaml:"token_percentages" mapstructure:"token_percentages"`
	
	// 缓存配置
	EnableCache      bool   `yaml:"enable_cache" mapstructure:"enable_cache"`
	CacheTTLSeconds  int    `yaml:"cache_ttl_seconds" mapstructure:"cache_ttl_seconds"`
	
	// 内容权重配置
	ContentWeights   map[string]float64 `yaml:"content_weights" mapstructure:"content_weights"`
	ContentPriorities map[string]int    `yaml:"content_priorities" mapstructure:"content_priorities"`
	
	// 高级选项
	PreferRecent     bool    `yaml:"prefer_recent" mapstructure:"prefer_recent"`
	AllowPartial     bool    `yaml:"allow_partial" mapstructure:"allow_partial"`
	StrictBudget     bool    `yaml:"strict_budget" mapstructure:"strict_budget"`
	QualityThreshold float64 `yaml:"quality_threshold" mapstructure:"quality_threshold"`
}

// TokenPercentageConfig Token百分比配置
type TokenPercentageConfig struct {
	Plan      float64 `yaml:"plan" mapstructure:"plan"`
	Character float64 `yaml:"character" mapstructure:"character"`
	Worldview float64 `yaml:"worldview" mapstructure:"worldview"`
	Chapters  float64 `yaml:"chapters" mapstructure:"chapters"`
	Index     float64 `yaml:"index" mapstructure:"index"`
}

// MessageQueueConfig 消息队列配置
type MessageQueueConfig struct {
	Enabled    bool `yaml:"enabled" mapstructure:"enabled"`
	Workers    int  `yaml:"workers" mapstructure:"workers"`
	BufferSize int  `yaml:"buffer_size" mapstructure:"buffer_size"`
}

// GetAbsolutePath 获取小说目录的绝对路径
func (n *NovelConfig) GetAbsolutePath() (string, error) {
	if n.Path == "" {
		return "", fmt.Errorf("小说路径未配置")
	}
	
	// 如果是绝对路径，直接返回
	if filepath.IsAbs(n.Path) {
		return n.Path, nil
	}
	
	// 相对路径转换为绝对路径
	absPath, err := filepath.Abs(n.Path)
	if err != nil {
		return "", fmt.Errorf("转换小说路径为绝对路径失败: %w", err)
	}
	
	return absPath, nil
}

// SetDefaults 设置内容配置默认值
func (c *ContentConfig) SetDefaults() {
	if c.MaxTokens <= 0 {
		c.MaxTokens = 8000
	}
	
	// 设置默认Token百分比
	if c.TokenPercentages.Plan == 0 && c.TokenPercentages.Character == 0 &&
		c.TokenPercentages.Worldview == 0 && c.TokenPercentages.Chapters == 0 &&
		c.TokenPercentages.Index == 0 {
		
		c.TokenPercentages = TokenPercentageConfig{
			Plan:      0.15,
			Character: 0.10,
			Worldview: 0.10,
			Chapters:  0.60,
			Index:     0.05,
		}
	}
	
	// 设置默认缓存配置
	if c.CacheTTLSeconds <= 0 {
		c.CacheTTLSeconds = 180 // 3分钟
		c.EnableCache = true
	}
	
	// 设置默认内容权重
	if c.ContentWeights == nil {
		c.ContentWeights = map[string]float64{
			"worldview":  1.0,
			"characters": 1.0,
			"chapters":   3.0,
			"plan":       1.5,
			"index":      0.5,
		}
	}
	
	// 设置默认内容优先级
	if c.ContentPriorities == nil {
		c.ContentPriorities = map[string]int{
			"worldview":  2,
			"characters": 2,
			"chapters":   1, // 最高优先级
			"plan":       3,
			"index":      4,
		}
	}
	
	// 设置默认高级选项
	c.PreferRecent = true
	c.AllowPartial = false
	c.StrictBudget = true
	if c.QualityThreshold == 0 {
		c.QualityThreshold = 0.8
	}
}

// Validate 验证内容配置
func (c *ContentConfig) Validate() error {
	if c.MaxTokens <= 0 {
		return fmt.Errorf("max_tokens必须大于0")
	}
	
	// 验证Token百分比
	total := c.TokenPercentages.Plan + c.TokenPercentages.Character +
		c.TokenPercentages.Worldview + c.TokenPercentages.Chapters +
		c.TokenPercentages.Index
		
	if total < 0.99 || total > 1.01 { // 允许±1%的误差
		return fmt.Errorf("token百分比总和应该等于1.0，当前为%.3f", total)
	}
	
	// 检查各个百分比是否为非负数
	percentages := map[string]float64{
		"plan":      c.TokenPercentages.Plan,
		"character": c.TokenPercentages.Character,
		"worldview": c.TokenPercentages.Worldview,
		"chapters":  c.TokenPercentages.Chapters,
		"index":     c.TokenPercentages.Index,
	}
	
	for name, value := range percentages {
		if value < 0 {
			return fmt.Errorf("token百分比'%s'不能为负数: %.3f", name, value)
		}
	}
	
	// 验证质量阈值
	if c.QualityThreshold < 0 || c.QualityThreshold > 1 {
		return fmt.Errorf("quality_threshold必须在0-1之间，当前为%.3f", c.QualityThreshold)
	}
	
	return nil
}

// Loader 配置加载器
type Loader struct {
	config *Config
}

// NewLoader 创建配置加载器
func NewLoader() *Loader {
	return &Loader{}
}

// Load 加载配置文件
func (l *Loader) Load(configPath string) (*Config, error) {
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")
	viper.AutomaticEnv()

	// 默认值
	viper.SetDefault("llm.timeout", "30s")
	viper.SetDefault("llm.ollama.base_url", "http://localhost:11434")
	viper.SetDefault("llm.openai.base_url", "http://localhost:13000/v1/")
	viper.SetDefault("app.port", 8080)
	viper.SetDefault("novel.path", "../novels/novel_example_title")
	viper.SetDefault("message_queue.enabled", false)
	viper.SetDefault("message_queue.workers", 2)
	viper.SetDefault("message_queue.buffer_size", 100)

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("解码配置失败: %w", err)
	}

	l.config = &cfg
	return &cfg, nil
}

// MustLoad 加载配置，失败则panic
func (l *Loader) MustLoad(configPath string) *Config {
	cfg, err := l.Load(configPath)
	if err != nil {
		panic(fmt.Sprintf("配置加载失败: %v", err))
	}
	return cfg
}

// Get 获取已加载的配置
func (l *Loader) Get() *Config {
	return l.config
}

// Reload 重新加载配置
func (l *Loader) Reload(configPath string) error {
	newCfg, err := l.Load(configPath)
	if err != nil {
		return err
	}
	l.config = newCfg
	return nil
}

// Watch 监听配置变更
func (l *Loader) Watch(logger *zap.Logger) {
	viper.OnConfigChange(func(e fsnotify.Event) {
		logger.Info("配置文件已变更", zap.String("file", e.Name))
	})
	viper.WatchConfig()
}

// Global 全局配置访问
var globalLoader *Loader

// InitGlobal 初始化全局配置
func InitGlobal(configPath string) error {
	globalLoader = NewLoader()
	_, err := globalLoader.Load(configPath)
	return err
}

// GetGlobal 获取全局配置
func GetGlobal() *Config {
	if globalLoader == nil {
		panic("配置未初始化，请先调用InitGlobal")
	}
	return globalLoader.Get()
}
