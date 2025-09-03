package token

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// TokenCounter Token计数器接口
type TokenCounter interface {
	Count(text string) int
	CountRunes(text string) int
	EstimateTokens(text string) int
}

// SimpleTokenCounter 简单Token计数器实现
type SimpleTokenCounter struct {
	// 中文字符与英文token的近似比率
	chineseToTokenRatio float64
	// 英文单词与token的近似比率
	englishToTokenRatio float64
}

// NewSimpleTokenCounter 创建简单Token计数器
func NewSimpleTokenCounter() *SimpleTokenCounter {
	return &SimpleTokenCounter{
		chineseToTokenRatio: 1.5, // 1个中文字符约等于1.5个token
		englishToTokenRatio: 0.75, // 1个英文单词约等于0.75个token
	}
}

// Count 精确计算Token数（基于经验算法）
func (c *SimpleTokenCounter) Count(text string) int {
	if text == "" {
		return 0
	}

	var chineseChars, englishWords, numbers, punctuation int
	
	// 按空白字符分割文本
	words := strings.Fields(text)
	
	for _, word := range words {
		if c.isChineseWord(word) {
			chineseChars += utf8.RuneCountInString(word)
		} else if c.isNumberWord(word) {
			numbers++
		} else {
			englishWords++
			// 计算标点符号
			for _, r := range word {
				if unicode.IsPunct(r) {
					punctuation++
				}
			}
		}
	}
	
	// Token计算公式（经验值）
	tokens := int(float64(chineseChars)*c.chineseToTokenRatio) +
		int(float64(englishWords)*c.englishToTokenRatio) +
		numbers/2 + // 数字通常token密度较高
		punctuation/3 // 标点符号token密度较低
		
	return tokens
}

// CountRunes 计算文本中的总字符数
func (c *SimpleTokenCounter) CountRunes(text string) int {
	return utf8.RuneCountInString(text)
}

// EstimateTokens 快速估算Token数（更简单的算法）
func (c *SimpleTokenCounter) EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	
	// 快速估算：中英文混合文本平均1.2字符约等于1个token
	runeCount := utf8.RuneCountInString(text)
	return int(float64(runeCount) / 1.2)
}

// isChineseWord 判断是否主要包含中文字符
func (c *SimpleTokenCounter) isChineseWord(word string) bool {
	chineseCount := 0
	totalCount := utf8.RuneCountInString(word)
	
	for _, r := range word {
		if c.isChinese(r) {
			chineseCount++
		}
	}
	
	// 如果中文字符占比超过50%，认为是中文单词
	return float64(chineseCount)/float64(totalCount) > 0.5
}

// isChinese 判断字符是否为中文
func (c *SimpleTokenCounter) isChinese(r rune) bool {
	// Unicode中文字符范围
	return (r >= 0x4e00 && r <= 0x9fff) ||  // CJK统一汉字
		(r >= 0x3400 && r <= 0x4dbf) ||     // CJK扩展A
		(r >= 0x20000 && r <= 0x2a6df) ||   // CJK扩展B
		(r >= 0x2a700 && r <= 0x2b73f) ||   // CJK扩展C
		(r >= 0x2b740 && r <= 0x2b81f) ||   // CJK扩展D
		(r >= 0x2b820 && r <= 0x2ceaf) ||   // CJK扩展E
		(r >= 0x2ceb0 && r <= 0x2ebef)      // CJK扩展F
}

// isNumberWord 判断是否主要包含数字
func (c *SimpleTokenCounter) isNumberWord(word string) bool {
	digitCount := 0
	totalCount := utf8.RuneCountInString(word)
	
	for _, r := range word {
		if unicode.IsDigit(r) {
			digitCount++
		}
	}
	
	// 如果数字字符占比超过70%，认为是数字单词
	return float64(digitCount)/float64(totalCount) > 0.7
}

// TokenBudget Token预算结构
type TokenBudget struct {
	Total       int
	Used        int
	Remaining   int
	Percentages map[string]float64
}

// NewTokenBudget 创建Token预算
func NewTokenBudget(total int, percentages map[string]float64) *TokenBudget {
	return &TokenBudget{
		Total:       total,
		Used:        0,
		Remaining:   total,
		Percentages: percentages,
	}
}

// AllocateTokens 按百分比分配Token（向上取整）
func (tb *TokenBudget) AllocateTokens() map[string]int {
	allocation := make(map[string]int)
	totalAllocated := 0
	
	for component, percentage := range tb.Percentages {
		// 向上取整确保每个组件都有足够的Token
		tokens := int(float64(tb.Total)*percentage + 0.999) // 加0.999实现向上取整
		allocation[component] = tokens
		totalAllocated += tokens
	}
	
	// 如果总分配超过了预算，需要按比例调整
	if totalAllocated > tb.Total {
		ratio := float64(tb.Total) / float64(totalAllocated)
		for component := range allocation {
			// 调整后仍然确保每个组件至少有1个Token
			adjusted := int(float64(allocation[component]) * ratio)
			if adjusted < 1 {
				adjusted = 1
			}
			allocation[component] = adjusted
		}
	}
	
	return allocation
}

// UseTokens 使用Token
func (tb *TokenBudget) UseTokens(count int) bool {
	if tb.Remaining >= count {
		tb.Used += count
		tb.Remaining -= count
		return true
	}
	return false
}

// GetUsageInfo 获取使用信息
func (tb *TokenBudget) GetUsageInfo() (used, remaining, total int) {
	return tb.Used, tb.Remaining, tb.Total
}