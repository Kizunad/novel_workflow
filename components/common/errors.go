package common

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Kizunad/modular-workflow-v2/logger"
)

// 退出码定义
const (
	ExitCodeSuccess       = 0
	ExitCodeGeneral       = 1
	ExitCodeConfig        = 2
	ExitCodeNetwork       = 3
	ExitCodeFileSystem    = 4
	ExitCodeInitialize    = 5
)

// CleanupFunc 清理函数类型
type CleanupFunc func() error

// Resource 表示需要清理的资源
type Resource struct {
	Name     string
	Cleanup  CleanupFunc
	Priority int // 优先级，数字越小优先级越高
}

// CleanupManager 资源清理管理器
type CleanupManager struct {
	resources []Resource
	logger    *logger.ZapLogger
	mutex     sync.RWMutex
	cleaned   bool
}

// NewCleanupManager 创建清理管理器
func NewCleanupManager(logger *logger.ZapLogger) *CleanupManager {
	return &CleanupManager{
		resources: make([]Resource, 0),
		logger:    logger,
		cleaned:   false,
	}
}

// RegisterResource 注册需要清理的资源
func (cm *CleanupManager) RegisterResource(name string, cleanup CleanupFunc, priority int) {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	
	cm.resources = append(cm.resources, Resource{
		Name:     name,
		Cleanup:  cleanup,
		Priority: priority,
	})
	
	if cm.logger != nil {
		cm.logger.Debug(fmt.Sprintf("注册清理资源: %s (优先级: %d)", name, priority))
	}
}

// RegisterHTTPServer 注册HTTP服务器清理
func (cm *CleanupManager) RegisterHTTPServer(server *http.Server) {
	cm.RegisterResource("HTTP服务器", func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		if cm.logger != nil {
			cm.logger.Info("正在优雅关闭HTTP服务器...")
		}
		
		if err := server.Shutdown(ctx); err != nil {
			return fmt.Errorf("HTTP服务器关闭失败: %w", err)
		}
		
		if cm.logger != nil {
			cm.logger.Info("HTTP服务器已优雅关闭")
		}
		return nil
	}, 1) // 高优先级
}

// RegisterLogger 注册Logger清理
func (cm *CleanupManager) RegisterLogger(logger *logger.ZapLogger) {
	cm.RegisterResource("Logger", func() error {
		if err := logger.Close(); err != nil {
			return fmt.Errorf("Logger同步失败: %w", err)
		}
		return nil
	}, 99) // 低优先级，最后清理
}

// RegisterGenericCloser 注册通用Closer接口
func (cm *CleanupManager) RegisterGenericCloser(name string, closer interface{ Close() error }, priority int) {
	cm.RegisterResource(name, func() error {
		return closer.Close()
	}, priority)
}

// Cleanup 执行所有清理操作
func (cm *CleanupManager) Cleanup() {
	cm.mutex.Lock()
	defer cm.mutex.Unlock()
	
	if cm.cleaned {
		return
	}
	cm.cleaned = true
	
	if cm.logger != nil {
		cm.logger.Info("开始执行资源清理...")
	}
	
	// 按优先级排序（数字越小优先级越高）
	resources := make([]Resource, len(cm.resources))
	copy(resources, cm.resources)
	
	// 简单冒泡排序按优先级排序
	for i := 0; i < len(resources); i++ {
		for j := i + 1; j < len(resources); j++ {
			if resources[i].Priority > resources[j].Priority {
				resources[i], resources[j] = resources[j], resources[i]
			}
		}
	}
	
	// 执行清理
	for _, resource := range resources {
		if err := resource.Cleanup(); err != nil {
			if cm.logger != nil {
				cm.logger.Error(fmt.Sprintf("清理资源失败 [%s]: %v", resource.Name, err))
			}
		} else if cm.logger != nil {
			cm.logger.Debug(fmt.Sprintf("成功清理资源: %s", resource.Name))
		}
	}
	
	if cm.logger != nil {
		cm.logger.Info("资源清理完成")
	}
}

// SetupGracefulShutdown 设置优雅关闭信号处理
func (cm *CleanupManager) SetupGracefulShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)
	
	go func() {
		sig := <-c
		if cm.logger != nil {
			cm.logger.Info(fmt.Sprintf("接收到退出信号: %s", sig.String()))
		}
		cm.Cleanup()
		os.Exit(ExitCodeSuccess)
	}()
}

// FatalWithCleanup 记录错误并优雅退出
func FatalWithCleanup(logger *logger.ZapLogger, err error, cleanupManager *CleanupManager) {
	if logger != nil {
		logger.Error(fmt.Sprintf("程序致命错误: %v", err))
	}
	
	if cleanupManager != nil {
		cleanupManager.Cleanup()
	}
	
	os.Exit(ExitCodeGeneral)
}

// FatalWithMessage 自定义错误消息并退出
func FatalWithMessage(logger *logger.ZapLogger, message string, cleanupManager *CleanupManager) {
	if logger != nil {
		logger.Error(fmt.Sprintf("程序终止: %s", message))
	}
	
	if cleanupManager != nil {
		cleanupManager.Cleanup()
	}
	
	os.Exit(ExitCodeGeneral)
}

// ExitWithCode 指定退出码退出
func ExitWithCode(code int, logger *logger.ZapLogger, message string, cleanupManager *CleanupManager) {
	if logger != nil {
		if code == ExitCodeSuccess {
			logger.Info(message)
		} else {
			logger.Error(fmt.Sprintf("程序异常退出 (code: %d): %s", code, message))
		}
	}
	
	if cleanupManager != nil {
		cleanupManager.Cleanup()
	}
	
	os.Exit(code)
}

// Must 包装错误，如果有错误则调用FatalWithCleanup
func Must(err error, logger *logger.ZapLogger, cleanupManager *CleanupManager) {
	if err != nil {
		FatalWithCleanup(logger, err, cleanupManager)
	}
}

// MustWithMessage 包装错误，如果有错误则使用自定义消息
func MustWithMessage(err error, message string, logger *logger.ZapLogger, cleanupManager *CleanupManager) {
	if err != nil {
		FatalWithMessage(logger, fmt.Sprintf("%s: %v", message, err), cleanupManager)
	}
}