package content

import (
	"fmt"
)

// ContentError 内容相关错误类型
type ContentError struct {
	Type    ErrorType
	Message string
	Cause   error
}

// ErrorType 错误类型枚举
type ErrorType int

const (
	ErrorTypeFileNotFound ErrorType = iota
	ErrorTypeFileRead
	ErrorTypeFileWrite
	ErrorTypeTokenExceeded
	ErrorTypeInvalidConfig
	ErrorTypeInvalidPath
	ErrorTypeCacheFailure
)

// Error 实现error接口
func (e *ContentError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Type.String(), e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Type.String(), e.Message)
}

// Unwrap 支持errors.Unwrap
func (e *ContentError) Unwrap() error {
	return e.Cause
}

// String ErrorType的字符串表示
func (et ErrorType) String() string {
	switch et {
	case ErrorTypeFileNotFound:
		return "FILE_NOT_FOUND"
	case ErrorTypeFileRead:
		return "FILE_READ"
	case ErrorTypeFileWrite:
		return "FILE_WRITE"
	case ErrorTypeTokenExceeded:
		return "TOKEN_EXCEEDED"
	case ErrorTypeInvalidConfig:
		return "INVALID_CONFIG"
	case ErrorTypeInvalidPath:
		return "INVALID_PATH"
	case ErrorTypeCacheFailure:
		return "CACHE_FAILURE"
	default:
		return "UNKNOWN"
	}
}

// 便捷的错误创建函数

// NewFileNotFoundError 文件未找到错误
func NewFileNotFoundError(path string, cause error) *ContentError {
	return &ContentError{
		Type:    ErrorTypeFileNotFound,
		Message: fmt.Sprintf("file not found: %s", path),
		Cause:   cause,
	}
}

// NewFileReadError 文件读取错误
func NewFileReadError(path string, cause error) *ContentError {
	return &ContentError{
		Type:    ErrorTypeFileRead,
		Message: fmt.Sprintf("failed to read file: %s", path),
		Cause:   cause,
	}
}

// NewFileWriteError 文件写入错误
func NewFileWriteError(path string, cause error) *ContentError {
	return &ContentError{
		Type:    ErrorTypeFileWrite,
		Message: fmt.Sprintf("failed to write file: %s", path),
		Cause:   cause,
	}
}

// NewTokenExceededError Token超限错误
func NewTokenExceededError(current, limit int) *ContentError {
	return &ContentError{
		Type:    ErrorTypeTokenExceeded,
		Message: fmt.Sprintf("token count %d exceeds limit %d", current, limit),
	}
}

// NewInvalidConfigError 无效配置错误
func NewInvalidConfigError(message string, cause error) *ContentError {
	return &ContentError{
		Type:    ErrorTypeInvalidConfig,
		Message: message,
		Cause:   cause,
	}
}

// NewInvalidPathError 无效路径错误
func NewInvalidPathError(path string, cause error) *ContentError {
	return &ContentError{
		Type:    ErrorTypeInvalidPath,
		Message: fmt.Sprintf("invalid path: %s", path),
		Cause:   cause,
	}
}

// NewCacheFailureError 缓存失败错误
func NewCacheFailureError(operation string, cause error) *ContentError {
	return &ContentError{
		Type:    ErrorTypeCacheFailure,
		Message: fmt.Sprintf("cache operation failed: %s", operation),
		Cause:   cause,
	}
}