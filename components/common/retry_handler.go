package common

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
)

// ============================================================================
// 核心接口定义
// ============================================================================

// RetryableOperation 可重试的操作接口 - 统一的重试抽象
type RetryableOperation[T any] interface {
	Execute(ctx context.Context) (T, error)
	ShouldRetry(err error) bool
	OnRetry(attempt int, err error, delay time.Duration)
}

// ============================================================================
// 配置定义
// ============================================================================

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries      int           // 最大重试次数，默认3
	InitialDelay    time.Duration // 初始延迟时间
	MaxDelay        time.Duration // 最大延迟时间
	BackoffExponent float64       // 指数退避倍数
	JitterFactor    float64       // 抖动因子
}

// DefaultRetryConfig 默认重试配置
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:      3,
		InitialDelay:    1 * time.Second,
		MaxDelay:        60 * time.Second,
		BackoffExponent: 2.0,
		JitterFactor:    0.1,
	}
}

// HTTPRetryConfig HTTP错误专用重试配置 - 为Gemini 2.5 Flash优化
func HTTPRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:      5, // HTTP错误重试次数更多
		InitialDelay:    2 * time.Second,
		MaxDelay:        120 * time.Second,
		BackoffExponent: 2.0,
		JitterFactor:    0.2, // 更多抖动避免惊群
	}
}

// ============================================================================
// 核心执行函数
// ============================================================================

// ExecuteWithRetry 执行带重试的操作 - 核心函数
func ExecuteWithRetry[T any](ctx context.Context, operation RetryableOperation[T], config *RetryConfig) (T, error) {
	var result T
	var lastErr error
	
	if config == nil {
		config = DefaultRetryConfig()
	}
	
	startTime := time.Now()
	maxAttempts := config.MaxRetries + 1
	
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// 第一次不延迟，从第二次开始延迟
		if attempt > 1 {
			delay := calculateDelay(attempt-1, config)
			operation.OnRetry(attempt, lastErr, delay)
			
			if err := waitWithContext(ctx, delay); err != nil {
				return result, fmt.Errorf("等待重试被中断: %w", err)
			}
		}
		
		// 执行操作
		result, err := operation.Execute(ctx)
		if err == nil {
			return result, nil // 成功返回
		}
		
		lastErr = err
		
		// 检查是否应该重试
		if !operation.ShouldRetry(err) {
			break // 不可重试错误，直接退出
		}
		
		if attempt >= maxAttempts {
			break // 达到最大重试次数
		}
	}
	
	return result, fmt.Errorf("重试失败 (重试%d次, 耗时%v): %w", 
		config.MaxRetries, time.Since(startTime), lastErr)
}

// ============================================================================
// HTTP操作实现
// ============================================================================

// HTTPOperation HTTP操作的通用实现
type HTTPOperation[T any] struct {
	ExecuteFunc func(ctx context.Context) (T, error)
	OnRetryFunc func(attempt int, err error, delay time.Duration)
}

// Execute 执行HTTP操作
func (op *HTTPOperation[T]) Execute(ctx context.Context) (T, error) {
	return op.ExecuteFunc(ctx)
}

// ShouldRetry 判断HTTP错误是否应该重试
func (op *HTTPOperation[T]) ShouldRetry(err error) bool {
	return isRetryableHTTPError(err)
}

// OnRetry 重试回调
func (op *HTTPOperation[T]) OnRetry(attempt int, err error, delay time.Duration) {
	if op.OnRetryFunc != nil {
		op.OnRetryFunc(attempt, err, delay)
	}
}

// NewHTTPOperation 创建HTTP操作
func NewHTTPOperation[T any](
	executeFunc func(ctx context.Context) (T, error),
	onRetryFunc func(int, error, time.Duration),
) *HTTPOperation[T] {
	return &HTTPOperation[T]{
		ExecuteFunc: executeFunc,
		OnRetryFunc: onRetryFunc,
	}
}

// ============================================================================
// 便捷函数
// ============================================================================

// WithHTTPRetry 为HTTP操作添加重试能力 - 最常用的函数
func WithHTTPRetry[T any](
	ctx context.Context,
	executeFunc func(ctx context.Context) (T, error),
	onRetryFunc func(int, error, time.Duration),
) (T, error) {
	operation := NewHTTPOperation(executeFunc, onRetryFunc)
	return ExecuteWithRetry(ctx, operation, HTTPRetryConfig())
}

// WithRetry 通用重试函数
func WithRetry[T any](
	ctx context.Context,
	executeFunc func(ctx context.Context) (T, error),
	config *RetryConfig,
) (T, error) {
	operation := &HTTPOperation[T]{ExecuteFunc: executeFunc}
	return ExecuteWithRetry(ctx, operation, config)
}

// ============================================================================
// 错误检测函数
// ============================================================================

// isRetryableHTTPError 判断是否为可重试的HTTP错误
func isRetryableHTTPError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := strings.ToLower(err.Error())
	
	// 限速错误 - Gemini 2.5 Flash重点关注
	rateLimitPatterns := []string{
		"429", "too many requests", "rate limit", "rate-limit", 
		"ratelimit", "quota exceeded", "request limit", 
		"throttle", "throttling",
	}
	
	// 临时性错误
	temporaryPatterns := []string{
		"timeout", "connection refused", "connection reset",
		"network is unreachable", "temporary failure",
		"502", "503", "504", // Bad Gateway, Service Unavailable, Gateway Timeout
		"520", "521", "522", "523", "524", // Cloudflare错误
	}
	
	allPatterns := append(rateLimitPatterns, temporaryPatterns...)
	
	for _, pattern := range allPatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}
	
	return false
}

// ============================================================================
// 辅助函数
// ============================================================================

// calculateDelay 计算延迟时间 - 指数退避 + 随机抖动
func calculateDelay(attempt int, config *RetryConfig) time.Duration {
	if attempt <= 0 {
		return 0
	}
	
	// 指数退避: delay = initial_delay * (backoff_exponent ^ (attempt - 1))
	delay := float64(config.InitialDelay) * math.Pow(config.BackoffExponent, float64(attempt-1))
	
	// 限制最大延迟
	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}
	
	// 添加随机抖动避免惊群效应
	if config.JitterFactor > 0 {
		jitter := delay * config.JitterFactor * (rand.Float64()*2 - 1) // [-factor, +factor]
		delay += jitter
	}
	
	// 确保延迟为正数
	if delay < 0 {
		delay = float64(config.InitialDelay)
	}
	
	return time.Duration(delay)
}

// waitWithContext 带上下文的等待
func waitWithContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(delay):
		return nil
	}
}

// ============================================================================
// 兼容性函数 - 用于现有workflow集成
// ============================================================================

// CreateMessageRetryOperation 创建消息重试操作 - 用于现有workflow
func CreateMessageRetryOperation(
	executeFunc func(ctx context.Context) (*schema.Message, error),
	minContentLen int,
	onRetryFunc func(int, error, time.Duration),
) RetryableOperation[*schema.Message] {
	return &MessageRetryOperation{
		ExecuteFunc:   executeFunc,
		MinContentLen: minContentLen,
		OnRetryFunc:   onRetryFunc,
	}
}

// MessageRetryOperation 消息重试操作实现
type MessageRetryOperation struct {
	ExecuteFunc   func(ctx context.Context) (*schema.Message, error)
	MinContentLen int
	OnRetryFunc   func(int, error, time.Duration)
}

func (op *MessageRetryOperation) Execute(ctx context.Context) (*schema.Message, error) {
	return op.ExecuteFunc(ctx)
}

func (op *MessageRetryOperation) ShouldRetry(err error) bool {
	// 先检查HTTP错误
	if isRetryableHTTPError(err) {
		return true
	}
	
	// 检查内容长度问题（通过错误消息判断）
	errStr := strings.ToLower(err.Error())
	contentIssues := []string{"empty", "too short", "insufficient", "内容为空", "过短"}
	
	for _, issue := range contentIssues {
		if strings.Contains(errStr, issue) {
			return true
		}
	}
	
	return false
}

func (op *MessageRetryOperation) OnRetry(attempt int, err error, delay time.Duration) {
	if op.OnRetryFunc != nil {
		op.OnRetryFunc(attempt, err, delay)
	}
}