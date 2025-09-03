package generators

import (
	"time"

	"github.com/Kizunad/modular-workflow-v2/components/content/token"
)

// ContentGenerator 内容生成器接口
type ContentGenerator interface {
	// 基本生成功能
	Generate() (string, error)
	GenerateWithTokenLimit(maxTokens int) (string, int, error)

	// 内容信息
	GetContentType() string
	GetLastGenerated() time.Time
	IsContentReady() bool

	// Token感知
	GetTokenCount() int
	EstimateTokens() int
	SetTokenBudget(budget *token.TokenBudgetManager)
}

// ContextAwareGenerator 上下文感知生成器接口
type ContextAwareGenerator interface {
	ContentGenerator

	// 上下文管理（使用interface{}避免循环导入）
	GenerateContext() (interface{}, error)
	GenerateContextWithBudget(budget map[string]int) (interface{}, error)

	// 组件获取
	GetWorldview() string
	GetCharacters() string
	GetChapters() string
	GetPlans() string
	GetIndex() string
}

// AggregatableGenerator 可聚合生成器接口 ?这个是做什么的
type AggregatableGenerator interface {
	ContentGenerator

	// 聚合功能
	AggregateContent(sources []ContentSource) (string, error)
	AggregateWithWeights(sources []WeightedContentSource) (string, error)

	// 优先级管理
	SetContentPriority(contentType string, priority int)
	GetContentPriority(contentType string) int
}


// ContentSource 内容源
type ContentSource struct {
	Type     string                 `json:"type"`
	Content  string                 `json:"content"`
	Metadata map[string]interface{} `json:"metadata"`
}

// WeightedContentSource 加权内容源
type WeightedContentSource struct {
	ContentSource
	Weight   float64 `json:"weight"`
	Priority int     `json:"priority"`
}

// GeneratorConfig 生成器配置
type GeneratorConfig struct {
	// 基础配置
	NovelDir string `json:"novel_dir"`

	// Token配置
	MaxTokens        int                       `json:"max_tokens"`
	TokenBudget      *token.TokenBudgetManager `json:"-"`
	TokenPercentages *token.TokenPercentages   `json:"token_percentages"`

	// 内容权重
	ContentWeights    map[string]float64 `json:"content_weights"`
	ContentPriorities map[string]int     `json:"content_priorities"`
}

// DefaultGeneratorConfig 默认生成器配置
func DefaultGeneratorConfig(novelDir string) *GeneratorConfig {
	return &GeneratorConfig{
		NovelDir:         novelDir,
		MaxTokens:        8000,
		TokenPercentages: token.DefaultTokenPercentages(),
		ContentWeights: map[string]float64{
			"worldview":  1.0,
			"characters": 1.0,
			"chapters":   3.0, // 章节内容权重最高
			"plan":       1.5,
			"index":      0.5,
		},
		ContentPriorities: map[string]int{
			"worldview":  2,
			"characters": 2,
			"chapters":   1, // 最高优先级
			"plan":       3,
			"index":      4,
		},
	}
}

// GeneratorMetrics 生成器指标
type GeneratorMetrics struct {
	// 生成统计
	TotalGenerations int           `json:"total_generations"`
	LastGeneration   time.Time     `json:"last_generation"`
	AverageGenTime   time.Duration `json:"average_gen_time"`

	// Token统计
	TotalTokensGenerated int     `json:"total_tokens_generated"` // TODO: 名词使用错误，并非Generated，而是导入/获取/获得
	AverageTokensPerGen  int     `json:"average_tokens_per_gen"`
	TokenEfficiency      float64 `json:"token_efficiency"`


	// 错误统计
	GenerationErrors int            `json:"generation_errors"`
	ErrorTypes       map[string]int `json:"error_types"`
	LastError        string         `json:"last_error"`
	LastErrorTime    time.Time      `json:"last_error_time"`
}

// NewGeneratorMetrics 创建新的生成器指标
func NewGeneratorMetrics() *GeneratorMetrics {
	return &GeneratorMetrics{
		ErrorTypes: make(map[string]int),
	}
}

// RecordGeneration 记录生成事件
func (gm *GeneratorMetrics) RecordGeneration(duration time.Duration, tokenCount int) {
	gm.TotalGenerations++
	gm.LastGeneration = time.Now()
	gm.TotalTokensGenerated += tokenCount

	// 计算平均值
	if gm.TotalGenerations > 0 {
		totalDuration := time.Duration(gm.TotalGenerations)*gm.AverageGenTime + duration
		gm.AverageGenTime = totalDuration / time.Duration(gm.TotalGenerations)
		gm.AverageTokensPerGen = gm.TotalTokensGenerated / gm.TotalGenerations
	}
}


// RecordError 记录错误
func (gm *GeneratorMetrics) RecordError(errorType, errorMsg string) {
	gm.GenerationErrors++
	gm.ErrorTypes[errorType]++
	gm.LastError = errorMsg
	gm.LastErrorTime = time.Now()
}


// CalculateTokenEfficiency 计算Token效率
func (gm *GeneratorMetrics) CalculateTokenEfficiency(targetTokens int) {
	if targetTokens > 0 && gm.TotalGenerations > 0 {
		actualTokens := float64(gm.TotalTokensGenerated) / float64(gm.TotalGenerations)
		gm.TokenEfficiency = actualTokens / float64(targetTokens)
	}
}
