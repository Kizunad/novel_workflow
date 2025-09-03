package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestLoadConfig(t *testing.T) {
	// 创建临时配置文件
	content := `llm:
  ollama:
    base_url: "http://localhost:11434"
    models: ["llama3.2"]
  openai:
    base_url: "http://localhost:13000/v1/"
    api_key: "test-key"
    models: ["gpt-4o-mini"]
  timeout: "45s"

vector:
  type: "chromadb"
  endpoint: "http://localhost:8000"
  timeout: "30s"

app:
  name: "test_app"
  version: "1.0.0"
  port: 8081
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	assert.NoError(t, err)
	tmpFile.Close()

	// 测试配置加载
	loader := NewLoader()
	cfg, err := loader.Load(tmpFile.Name())
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// 验证加载的配置值
	assert.Equal(t, "http://localhost:11434", cfg.LLM.Ollama.BaseURL)
	assert.Equal(t, []string{"llama3.2"}, cfg.LLM.Ollama.Models)
	assert.Equal(t, "http://localhost:13000/v1/", cfg.LLM.OpenAI.BaseURL)
	assert.Equal(t, "test-key", cfg.LLM.OpenAI.APIKey)
	assert.Equal(t, []string{"gpt-4o-mini"}, cfg.LLM.OpenAI.Models)
	assert.Equal(t, 45*time.Second, cfg.LLM.Timeout)

	assert.Equal(t, "chromadb", cfg.Vector.Type)
	assert.Equal(t, "http://localhost:8000", cfg.Vector.Endpoint)
	assert.Equal(t, "30s", cfg.Vector.Timeout)

	assert.Equal(t, "test_app", cfg.App.Name)
	assert.Equal(t, "1.0.0", cfg.App.Version)
	assert.Equal(t, 8081, cfg.App.Port)
}

func TestConfigWithDefaults(t *testing.T) {
	// 创建只包含部分字段的临时配置文件
	content := `vector:
  type: "faiss"
  endpoint: "http://localhost:9000"

app:
  name: "default_test"
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	assert.NoError(t, err)
	tmpFile.Close()

	loader := NewLoader()
	cfg, err := loader.Load(tmpFile.Name())
	assert.NoError(t, err)
	assert.NotNil(t, cfg)

	// 验证默认值
	assert.Equal(t, 30*time.Second, cfg.LLM.Timeout)
	assert.Equal(t, "http://localhost:11434", cfg.LLM.Ollama.BaseURL)
	assert.Equal(t, "http://localhost:13000/v1/", cfg.LLM.OpenAI.BaseURL)
	assert.Equal(t, 8080, cfg.App.Port)
}

func TestGlobalInitializer(t *testing.T) {
	content := `app:
  name: "global_test"
  port: 8082
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	assert.NoError(t, err)
	tmpFile.Close()

	// 测试全局初始化
	err = InitGlobal(tmpFile.Name())
	assert.NoError(t, err)

	cfg := GetGlobal()
	assert.Equal(t, "global_test", cfg.App.Name)
	assert.Equal(t, 8082, cfg.App.Port)
}

func TestLoadNonExistentFile(t *testing.T) {
	loader := NewLoader()
	cfg, err := loader.Load("non-existent.yaml")
	assert.Error(t, err)
	assert.Nil(t, cfg)
}

func TestWatchConfig(t *testing.T) {
	content := `app:
  name: "watch_test"
  port: 8083
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	assert.NoError(t, err)
	tmpFile.Close()

	logger := zap.NewNop() // 测试中不真正输出日志
	loader := NewLoader()

	// 测试Watch不会panic
	cfg := loader.MustLoad(tmpFile.Name())
	assert.NotNil(t, cfg)

	// Watch应该在后台运行，测试中只验证不会panic
	loader.Watch(logger)
}

func TestGetters(t *testing.T) {
	content := `app:
  name: "getter_test"
`

	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(content)
	assert.NoError(t, err)
	tmpFile.Close()

	loader := NewLoader()
	cfg, err := loader.Load(tmpFile.Name())
	assert.NoError(t, err)

	// 验证Get方法
	assert.Equal(t, cfg, loader.Get())
}
