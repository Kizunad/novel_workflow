package queue

import (
	"time"

	"github.com/Kizunad/modular-workflow-v2/config"
)

// Config 队列内部配置
type Config struct {
	// 从外部配置读取
	Enabled    bool
	Workers    int
	BufferSize int
	
	// 内部默认值
	RetryInterval   time.Duration
	MaxRetries      int
	ShutdownTimeout time.Duration
}

// NewConfig 从外部配置创建队列配置
func NewConfig(external *config.MessageQueueConfig) *Config {
	cfg := &Config{
		// 从外部配置
		Enabled:    external.Enabled,
		Workers:    external.Workers,
		BufferSize: external.BufferSize,
		
		// 内部默认值
		RetryInterval:   30 * time.Second,
		MaxRetries:      3,
		ShutdownTimeout: 30 * time.Second,
	}
	
	// 基本验证和调整
	if cfg.Workers <= 0 {
		cfg.Workers = 2
	}
	if cfg.BufferSize <= 0 {
		cfg.BufferSize = 100
	}
	
	return cfg
}