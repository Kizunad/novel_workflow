package token

import (
	"fmt"
	"math"
	"strings"
	"errors"
)

// TokenBudgetManager Token预算管理器
type TokenBudgetManager struct {
	maxTokens   int
	percentages *TokenPercentages
	counter     TokenCounter
	budget      *TokenBudget
}

// TokenPercentages Token百分比配置
type TokenPercentages struct {
	Plan      float64 `json:"plan" yaml:"plan"`           // 规划内容占比
	Character float64 `json:"character" yaml:"character"` // 角色信息占比
	Worldview float64 `json:"worldview" yaml:"worldview"` // 世界观占比
	Chapters  float64 `json:"chapters" yaml:"chapters"`   // 章节内容占比
	Index     float64 `json:"index" yaml:"index"`         // 索引摘要占比
}

// DefaultTokenPercentages 默认Token百分比配置
func DefaultTokenPercentages() *TokenPercentages {
	return &TokenPercentages{
		Plan:      0.15, // 15% - 规划策略
		Character: 0.10, // 10% - 角色信息
		Worldview: 0.10, // 10% - 世界观设定
		Chapters:  0.60, // 60% - 章节内容（主要部分）
		Index:     0.05, // 5% - 索引摘要
	}
}

// Validate 验证百分比配置
func (tp *TokenPercentages) Validate() error {
	total := tp.Plan + tp.Character + tp.Worldview + tp.Chapters + tp.Index
	
	// 允许±1%的误差
	if math.Abs(total-1.0) > 0.01 {
		return fmt.Errorf("token percentages sum to %.3f, expected 1.0", total)
	}
	
	// 检查每个百分比是否为正数
	if tp.Plan < 0 || tp.Character < 0 || tp.Worldview < 0 || tp.Chapters < 0 || tp.Index < 0 {
		return errors.New("all token percentages must be non-negative")
	}
	
	return nil
}

// ToMap 转换为map格式
func (tp *TokenPercentages) ToMap() map[string]float64 {
	return map[string]float64{
		"plan":      tp.Plan,
		"character": tp.Character,
		"worldview": tp.Worldview,
		"chapters":  tp.Chapters,
		"index":     tp.Index,
	}
}

// NewTokenBudgetManager 创建Token预算管理器
func NewTokenBudgetManager(maxTokens int, percentages *TokenPercentages) (*TokenBudgetManager, error) {
	if maxTokens <= 0 {
		return nil, errors.New("maxTokens must be positive")
	}
	
	if percentages == nil {
		percentages = DefaultTokenPercentages()
	}
	
	if err := percentages.Validate(); err != nil {
		return nil, err
	}
	
	manager := &TokenBudgetManager{
		maxTokens:   maxTokens,
		percentages: percentages,
		counter:     NewSimpleTokenCounter(),
		budget:      NewTokenBudget(maxTokens, percentages.ToMap()),
	}
	
	return manager, nil
}

// GetAllocatedTokens 获取各组件分配的Token数量
func (tbm *TokenBudgetManager) GetAllocatedTokens() map[string]int {
	return tbm.budget.AllocateTokens()
}

// GetTokenAllocation 获取具体组件的Token分配
func (tbm *TokenBudgetManager) GetTokenAllocation(component string) int {
	allocation := tbm.GetAllocatedTokens()
	if tokens, exists := allocation[component]; exists {
		return tokens
	}
	return 0
}

// TruncateToTokenLimit 将文本截断到Token限制内
func (tbm *TokenBudgetManager) TruncateToTokenLimit(text string, component string) (string, int) {
	if text == "" {
		return "", 0
	}
	
	maxTokens := tbm.GetTokenAllocation(component)
	if maxTokens <= 0 {
		return "", 0
	}
	
	// 如果文本Token数在限制内，直接返回
	currentTokens := tbm.counter.Count(text)
	if currentTokens <= maxTokens {
		return text, currentTokens
	}
	
	// 需要截断文本
	return tbm.truncateText(text, maxTokens)
}

// truncateText 智能截断文本
func (tbm *TokenBudgetManager) truncateText(text string, maxTokens int) (string, int) {
	lines := strings.Split(text, "\n")
	var result []string
	totalTokens := 0
	
	for _, line := range lines {
		lineTokens := tbm.counter.Count(line)
		
		// 如果添加这一行会超出限制
		if totalTokens+lineTokens > maxTokens {
			// 尝试部分添加这一行
			if totalTokens < maxTokens {
				remainingTokens := maxTokens - totalTokens
				truncatedLine := tbm.truncateLineByTokens(line, remainingTokens)
				if truncatedLine != "" {
					result = append(result, truncatedLine)
					totalTokens += tbm.counter.Count(truncatedLine)
				}
			}
			break
		}
		
		result = append(result, line)
		totalTokens += lineTokens
	}
	
	return strings.Join(result, "\n"), totalTokens
}

// truncateLineByTokens 按Token数截断单行
func (tbm *TokenBudgetManager) truncateLineByTokens(line string, maxTokens int) string {
	if maxTokens <= 0 {
		return ""
	}
	
	words := strings.Fields(line)
	var result []string
	totalTokens := 0
	
	for _, word := range words {
		wordTokens := tbm.counter.Count(word)
		if totalTokens+wordTokens > maxTokens {
			break
		}
		result = append(result, word)
		totalTokens += wordTokens
	}
	
	return strings.Join(result, " ")
}

// GetUsageStats 获取使用统计
func (tbm *TokenBudgetManager) GetUsageStats() map[string]interface{} {
	used, remaining, total := tbm.budget.GetUsageInfo()
	allocation := tbm.GetAllocatedTokens()
	
	return map[string]interface{}{
		"total":      total,
		"used":       used,
		"remaining":  remaining,
		"allocation": allocation,
		"percentages": tbm.percentages,
	}
}

// UpdatePercentages 更新百分比配置
func (tbm *TokenBudgetManager) UpdatePercentages(newPercentages *TokenPercentages) error {
	if err := newPercentages.Validate(); err != nil {
		return err
	}
	
	tbm.percentages = newPercentages
	tbm.budget = NewTokenBudget(tbm.maxTokens, newPercentages.ToMap())
	
	return nil
}

// CountTokens 计算文本Token数
func (tbm *TokenBudgetManager) CountTokens(text string) int {
	return tbm.counter.Count(text)
}

// EstimateTokens 估算文本Token数（快速算法）
func (tbm *TokenBudgetManager) EstimateTokens(text string) int {
	return tbm.counter.EstimateTokens(text)
}

// ValidateContent 验证内容是否超出Token限制
func (tbm *TokenBudgetManager) ValidateContent(content map[string]string) error {
	allocation := tbm.GetAllocatedTokens()
	
	for component, text := range content {
		if maxTokens, exists := allocation[component]; exists {
			actualTokens := tbm.counter.Count(text)
			if actualTokens > maxTokens {
				return fmt.Errorf("token count %d exceeds limit %d", actualTokens, maxTokens)
			}
		}
	}
	
	return nil
}