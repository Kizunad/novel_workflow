package managers

import (
	"os"
	"path/filepath"
	"time"

	"github.com/Kizunad/modular-workflow-v2/components/content/token"
)

// FileManager 文件管理器接口
type FileManager interface {
	// 基本操作
	Load() (string, error)
	Save(content string) error
	Exists() bool
	GetPath() string

	// Token相关
	GetTokenCount() int
	EstimateTokens() int

	GetModTime() (time.Time, error)
	IsModified() bool

	// 内容管理
	GetCurrent() string
	Update(content string) error
}

// ContentProvider 内容提供者接口
type ContentProvider interface {
	// 获取内容
	GetContent() (string, error)
	GetContentWithTokenLimit(maxTokens int) (string, int, error)

	// 内容信息
	GetContentType() string
	GetLastModified() time.Time
	IsContentReady() bool
}

// CacheableContent 可缓存内容接口
type CacheableContent interface {
	// 缓存键
	GetCacheKey() string

	// 缓存有效性
	IsCacheValid() bool
	GetCacheExpiration() time.Duration

	// 缓存数据
	GetCacheData() map[string]interface{}
	SetCacheData(data map[string]interface{}) error
}

// TokenAware Token感知接口
type TokenAware interface {
	// Token计算
	CountTokens(text string) int
	EstimateTokens(text string) int

	// Token预算
	SetTokenBudget(budget *token.TokenBudgetManager)
	GetTokenBudget() *token.TokenBudgetManager

	// Token截断
	TruncateToLimit(text string, limit int) (string, int)
}

// FileInfo 文件信息结构
type FileInfo struct {
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	ModTime     time.Time `json:"mod_time"`
	Exists      bool      `json:"exists"`
	IsReadable  bool      `json:"is_readable"`
	IsWriteable bool      `json:"is_writeable"`
}

// GetFileInfo 获取文件信息
func GetFileInfo(path string) *FileInfo {
	info := &FileInfo{
		Path:   path,
		Exists: false,
	}

	if stat, err := os.Stat(path); err == nil {
		info.Exists = true
		info.Size = stat.Size()
		info.ModTime = stat.ModTime()
		info.IsReadable = isReadable(path)
		info.IsWriteable = isWriteable(path)
	}

	return info
}

// isReadable 检查文件是否可读
func isReadable(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	file.Close()
	return true
}

// isWriteable 检查文件是否可写
func isWriteable(path string) bool {
	// 如果文件不存在，检查目录是否可写
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// 检查父目录是否可写
		dir := filepath.Dir(path)
		tempFile := filepath.Join(dir, ".temp_write_test")
		f, err := os.Create(tempFile)
		if err != nil {
			return false
		}
		f.Close()
		os.Remove(tempFile)
		return true
	}

	// 文件存在，尝试以写入模式打开
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return false
	}
	file.Close()
	return true
}
