package managers

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Kizunad/modular-workflow-v2/components/content/token"
	content "github.com/Kizunad/modular-workflow-v2/components/content/utils"
)

// PlanEntry 单个章节的规划记录
type PlanEntry struct {
	Title string `json:"title"`
	Plan  string `json:"plan"`
}

// PlannerState planner.json 的整体结构
type PlannerState struct {
	Chapters      int         `json:"chapters"`
	Plans         []PlanEntry `json:"plans"`
	LatestContent string      `json:"latest_content"`
	UpdatedAt     string      `json:"updated_at"`
}

// PlannerContentManager Token感知的规划内容管理器
type PlannerContentManager struct {
	*BaseFileManager
	novelDir string
	state    *PlannerState
}

// NewPlannerContentManager 创建规划内容管理器
func NewPlannerContentManager(novelDir string) *PlannerContentManager {
	plannerPath := filepath.Join(novelDir, "planner.json")
	
	manager := &PlannerContentManager{
		BaseFileManager: NewBaseFileManager(plannerPath),
		novelDir:        novelDir,
	}
	
	// 尝试加载现有状态
	manager.load()
	
	return manager
}

// NewPlannerContentManagerWithTokenBudget 创建带Token预算的规划内容管理器
func NewPlannerContentManagerWithTokenBudget(novelDir string, tokenBudget *token.TokenBudgetManager) *PlannerContentManager {
	manager := NewPlannerContentManager(novelDir)
	manager.SetTokenBudget(tokenBudget)
	return manager
}

// load 加载规划状态（内部方法）
func (pcm *PlannerContentManager) load() {
	if content, err := pcm.BaseFileManager.Load(); err == nil && content != "" {
		var state PlannerState
		if err := json.Unmarshal([]byte(content), &state); err == nil {
			pcm.state = &state
		}
	}
	
	// 如果加载失败或文件不存在，初始化默认状态
	if pcm.state == nil {
		pcm.state = &PlannerState{
			Chapters:      0,
			Plans:         []PlanEntry{},
			LatestContent: "",
			UpdatedAt:     time.Now().Format(time.RFC3339),
		}
	}
}

// Load 获取当前规划状态
func (pcm *PlannerContentManager) Load() (*PlannerState, error) {
	// 检查文件是否被修改，如果是则重新加载
	if pcm.IsModified() {
		pcm.load()
	}
	
	return pcm.state, nil
}

// Save 保存规划状态
func (pcm *PlannerContentManager) Save(ps *PlannerState) error {
	if ps == nil {
		return content.NewInvalidConfigError("planner state cannot be nil", nil)
	}
	
	// 更新时间戳
	ps.UpdatedAt = time.Now().Format(time.RFC3339)
	
	// 序列化为JSON
	data, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return content.NewFileWriteError(pcm.GetPath(), err)
	}
	
	// 保存到文件
	if err := pcm.BaseFileManager.Save(string(data)); err != nil {
		return err
	}
	
	// 更新内存状态
	pcm.state = ps
	
	return nil
}

// CountChapters 计算章节数量
func (pcm *PlannerContentManager) CountChapters() (int, error) {
	// 使用新的ChapterManager（假设已重构）
	chapterManager := NewChapterManager(pcm.novelDir)
	return chapterManager.GetChapterCount(), nil
}

// UpsertPlan 插入或更新规划
func (pcm *PlannerContentManager) UpsertPlan(title, plan string) error {
	if title == "" {
		return content.NewInvalidConfigError("plan title cannot be empty", nil)
	}
	
	state, err := pcm.Load()
	if err != nil {
		return err
	}
	
	// 查找是否已存在该标题的规划
	found := false
	for i, entry := range state.Plans {
		if entry.Title == title {
			state.Plans[i].Plan = plan
			found = true
			break
		}
	}
	
	// 如果不存在，添加新的规划
	if !found {
		state.Plans = append(state.Plans, PlanEntry{
			Title: title,
			Plan:  plan,
		})
	}
	
	return pcm.Save(state)
}

// GetPlan 获取指定标题的规划
func (pcm *PlannerContentManager) GetPlan(title string) (string, bool) {
	state, err := pcm.Load()
	if err != nil {
		return "", false
	}
	
	for _, entry := range state.Plans {
		if entry.Title == title {
			return entry.Plan, true
		}
	}
	
	return "", false
}

// GetAllPlans 获取所有规划
func (pcm *PlannerContentManager) GetAllPlans() []PlanEntry {
	state, err := pcm.Load()
	if err != nil {
		return []PlanEntry{}
	}
	
	// 按标题排序
	plans := make([]PlanEntry, len(state.Plans))
	copy(plans, state.Plans)
	
	sort.Slice(plans, func(i, j int) bool {
		return plans[i].Title < plans[j].Title
	})
	
	return plans
}

// GetPlansWithTokenLimit 获取限制Token数量的规划内容
func (pcm *PlannerContentManager) GetPlansWithTokenLimit(maxTokens int) (string, int) {
	plans := pcm.GetAllPlans()
	if len(plans) == 0 {
		return "", 0
	}
	
	// 构建规划文本
	var planBuilder strings.Builder
	for i, entry := range plans {
		if i > 0 {
			planBuilder.WriteString("\n\n")
		}
		planBuilder.WriteString("章节: ")
		planBuilder.WriteString(entry.Title)
		planBuilder.WriteString("\n规划: ")
		planBuilder.WriteString(entry.Plan)
	}
	
	planText := planBuilder.String()
	
	// 如果有TokenBudget，使用正确的组件名
	if pcm.GetTokenBudget() != nil {
		return pcm.GetTokenBudget().TruncateToTokenLimit(planText, "plan")
	}
	
	// 否则使用基础的截断逻辑
	return pcm.TruncateToLimit(planText, maxTokens)
}

// UpdateLatestContent 更新最新内容
func (pcm *PlannerContentManager) UpdateLatestContent(content string) error {
	state, err := pcm.Load()
	if err != nil {
		return err
	}
	
	// 如果启用了Token预算，截断内容
	if pcm.GetTokenBudget() != nil {
		truncated, _ := pcm.GetTokenBudget().TruncateToTokenLimit(content, "plan")
		state.LatestContent = truncated
	} else {
		state.LatestContent = content
	}
	
	return pcm.Save(state)
}

// GetLatestContent 获取最新内容
func (pcm *PlannerContentManager) GetLatestContent() string {
	state, err := pcm.Load()
	if err != nil {
		return ""
	}
	
	return state.LatestContent
}

// GetLatestContentWithTokenLimit 获取限制Token的最新内容
func (pcm *PlannerContentManager) GetLatestContentWithTokenLimit(maxTokens int) (string, int) {
	content := pcm.GetLatestContent()
	if content == "" {
		return "", 0
	}
	
	return pcm.TruncateToLimit(content, maxTokens)
}

// DeletePlan 删除指定标题的规划
func (pcm *PlannerContentManager) DeletePlan(title string) error {
	state, err := pcm.Load()
	if err != nil {
		return err
	}
	
	// 查找并删除
	for i, entry := range state.Plans {
		if entry.Title == title {
			state.Plans = append(state.Plans[:i], state.Plans[i+1:]...)
			return pcm.Save(state)
		}
	}
	
	// 如果没找到，不报错
	return nil
}

// ClearAllPlans 清空所有规划
func (pcm *PlannerContentManager) ClearAllPlans() error {
	state, err := pcm.Load()
	if err != nil {
		return err
	}
	
	state.Plans = []PlanEntry{}
	state.LatestContent = ""
	
	return pcm.Save(state)
}

// GetPlannerMetadata 获取规划管理器元数据
func (pcm *PlannerContentManager) GetPlannerMetadata() map[string]interface{} {
	info := pcm.GetFileInfo()
	state, _ := pcm.Load()
	
	metadata := map[string]interface{}{
		"path":         info.Path,
		"exists":       info.Exists,
		"mod_time":     info.ModTime,
		"token_count":  pcm.GetTokenCount(),
		"plan_count":   len(state.Plans),
		"has_content":  state.LatestContent != "",
		"updated_at":   state.UpdatedAt,
	}
	
	if pcm.GetTokenBudget() != nil {
		estimated := pcm.EstimateTokens()
		budget := pcm.GetTokenBudget().GetTokenAllocation("plan")
		metadata["token_estimate"] = estimated
		metadata["token_budget"] = budget
		metadata["within_budget"] = estimated <= budget
	}
	
	return metadata
}

// ValidatePlanner 验证规划数据
func (pcm *PlannerContentManager) ValidatePlanner() error {
	state, err := pcm.Load()
	if err != nil {
		return err
	}
	
	// 检查规划数据的一致性
	for i, entry := range state.Plans {
		if entry.Title == "" {
			return content.NewInvalidConfigError(
				"plan entry at index "+strconv.Itoa(i)+" has empty title", nil)
		}
	}
	
	return nil
}

// GetPlansSummary 获取规划摘要
func (pcm *PlannerContentManager) GetPlansSummary() string {
	plans := pcm.GetAllPlans()
	if len(plans) == 0 {
		return "暂无规划内容"
	}
	
	var summaryBuilder strings.Builder
	summaryBuilder.WriteString("已有规划: ")
	
	titles := make([]string, len(plans))
	for i, plan := range plans {
		titles[i] = plan.Title
	}
	
	summaryBuilder.WriteString(strings.Join(titles, ", "))
	
	return summaryBuilder.String()
}

// FormatPlansForContext 格式化规划内容用于上下文
func (pcm *PlannerContentManager) FormatPlansForContext() string {
	plans := pcm.GetAllPlans()
	if len(plans) == 0 {
		return "暂无章节规划"
	}
	
	var contextBuilder strings.Builder
	contextBuilder.WriteString("章节规划:\n\n")
	
	for _, entry := range plans {
		contextBuilder.WriteString("【")
		contextBuilder.WriteString(entry.Title)
		contextBuilder.WriteString("】\n")
		contextBuilder.WriteString(entry.Plan)
		contextBuilder.WriteString("\n\n")
	}
	
	return strings.TrimSpace(contextBuilder.String())
}