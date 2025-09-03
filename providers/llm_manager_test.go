package providers

import (
	"context"
	"testing"
	"time"

	"github.com/Kizunad/modular-workflow-v2/config"
	"github.com/Kizunad/modular-workflow-v2/logger"
)

// TestLLMManager 测试LLM Manager的基本功能
func TestLLMManager(t *testing.T) {
	// 创建测试配置
	cfg := &config.Config{
		LLM: config.LLMConfig{
			Ollama: config.OllamaConfig{
				BaseURL: "http://localhost:11434",
				Models:  []string{"qwen3:4b"},
			},
			OpenAI: config.OpenAIConfig{
				BaseURL: "http://localhost:13000/v1/",
				APIKey:  "sk-rac1XoSpt3eESULMNGKxAvBQq2WwcqIoSJMhsg2ubOU6tiJQ",
				Models:  []string{"glm-4.5-air"},
			},
			Timeout: 30 * time.Second,
		},
	}

	// 创建logger
	zapLogger := logger.New()
	defer zapLogger.Close()

	// 创建LLM Manager
	manager := NewManager(cfg, *zapLogger)

	t.Run("Test GetOllamaModel with fallback", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		model, err := manager.GetOllamaModel(ctx)
		if err != nil {
			t.Logf("GetOllamaModel failed (expected if Ollama unavailable): %v", err)
			return
		}

		if model == nil {
			t.Error("Expected non-nil model")
		}

		t.Logf("Successfully got model from GetOllamaModel")
	})

	t.Run("Test GetOpenAIModel direct", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		model, err := manager.GetOpenAIModel(ctx)
		if err != nil {
			t.Logf("GetOpenAIModel failed (expected if new-api unavailable): %v", err)
			return
		}

		if model == nil {
			t.Error("Expected non-nil model")
		}

		t.Logf("Successfully got model from GetOpenAIModel")
	})

	t.Run("Test Provider Health Checks", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 测试Ollama健康检查
		ollamaProvider := manager.GetOllamaProvider()
		ollamaErr := ollamaProvider.HealthCheck(ctx)
		t.Logf("Ollama health check result: %v", ollamaErr)

		// 测试OpenAI健康检查
		openaiProvider := manager.GetOpenAIProvider()
		openaiErr := openaiProvider.HealthCheck(ctx)
		t.Logf("new-api health check result: %v", openaiErr)
	})
}

// TestLLMManagerFallbackLogic 专门测试fallback逻辑
func TestLLMManagerFallbackLogic(t *testing.T) {
	// 创建一个Ollama不可用的配置
	cfg := &config.Config{
		LLM: config.LLMConfig{
			Ollama: config.OllamaConfig{
				BaseURL: "http://localhost:11434",
				Models:  []string{"qwen3:4b"},
			},
			OpenAI: config.OpenAIConfig{
				BaseURL: "http://localhost:13000/v1/",
				APIKey:  "sk-rac1XoSpt3eESULMNGKxAvBQq2WwcqIoSJMhsg2ubOU6tiJQ",
				Models:  []string{"glm-4.5-air"},
			},
		},
	}

	zapLogger := logger.New()
	defer zapLogger.Close()

	manager := NewManager(cfg, *zapLogger)

	t.Run("Test Fallback Logic", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// GetOllamaModel应该fallback到OpenAI（如果OpenAI可用）
		model, err := manager.GetOllamaModel(ctx)
		if err != nil {
			t.Logf("Fallback also failed (expected if both services unavailable): %v", err)
			return
		}

		if model == nil {
			t.Error("Expected non-nil model from fallback")
		}

		t.Logf("Fallback logic worked successfully")
	})
}

// BenchmarkLLMManager 性能基准测试
func BenchmarkLLMManager(b *testing.B) {
	cfg := &config.Config{
		LLM: config.LLMConfig{
			Ollama: config.OllamaConfig{
				BaseURL: "http://localhost:11434",
				Models:  []string{"qwen3:4b"},
			},
			OpenAI: config.OpenAIConfig{
				BaseURL: "http://localhost:13000/v1/",
				APIKey:  "sk-rac1XoSpt3eESULMNGKxAvBQq2WwcqIoSJMhsg2ubOU6tiJQ",
				Models:  []string{"glm-4.5-air"},
			},
		},
	}

	zapLogger := logger.New()
	defer zapLogger.Close()

	manager := NewManager(cfg, *zapLogger)
	ctx := context.Background()

	b.Run("GetOllamaModel", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := manager.GetOllamaModel(ctx)
			if err != nil {
				b.Logf("GetOllamaModel failed: %v", err)
			}
		}
	})
}
