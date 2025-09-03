package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// Planning相关的数据结构
type Task struct {
	ID           string    `json:"id"`
	Content      string    `json:"content"`
	Priority     string    `json:"priority"`     // high/medium/low
	Status       string    `json:"status"`       // planned/in_progress/completed
	Dependencies []string  `json:"dependencies"` // 依赖的任务ID
	EstimatedTime int      `json:"estimated_time"` // 预估时间(分钟)
	StartTime    *time.Time `json:"start_time,omitempty"`
	EndTime      *time.Time `json:"end_time,omitempty"`
	Tags         []string  `json:"tags"`
}

type Plan struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Tasks       []Task    `json:"tasks"`
	Goal        string    `json:"goal"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// 全局存储 (实际项目中应该使用数据库)
var (
	plans = make(map[string]*Plan)
	tasks = make(map[string]*Task)
)

// CreatePlanParams 创建计划的参数
type CreatePlanParams struct {
	Title       string `json:"title" jsonschema:"description=计划的标题"`
	Description string `json:"description" jsonschema:"description=计划的详细描述"`
	Goal        string `json:"goal" jsonschema:"description=计划要达成的目标"`
}

// AddTaskParams 添加任务的参数
type AddTaskParams struct {
	PlanID        string   `json:"plan_id" jsonschema:"description=计划ID"`
	Content       string   `json:"content" jsonschema:"description=任务内容"`
	Priority      string   `json:"priority" jsonschema:"description=任务优先级: high/medium/low"`
	EstimatedTime int      `json:"estimated_time" jsonschema:"description=预估完成时间(分钟)"`
	Dependencies  []string `json:"dependencies,omitempty" jsonschema:"description=依赖的任务ID列表"`
	Tags          []string `json:"tags,omitempty" jsonschema:"description=任务标签"`
}

// UpdateTaskParams 更新任务的参数
type UpdateTaskParams struct {
	TaskID string  `json:"task_id" jsonschema:"description=任务ID"`
	Status *string `json:"status,omitempty" jsonschema:"description=更新任务状态: planned/in_progress/completed"`
}

// AnalyzeGoalParams 分析目标的参数
type AnalyzeGoalParams struct {
	Goal        string `json:"goal" jsonschema:"description=需要分析的目标"`
	Context     string `json:"context,omitempty" jsonschema:"description=相关上下文信息"`
	Constraints string `json:"constraints,omitempty" jsonschema:"description=约束条件"`
}

// CreatePlanTool 创建计划工具
type CreatePlanTool struct{}

func (t *CreatePlanTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "create_plan",
		Desc: "创建一个新的计划，用于组织和管理任务",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"title": {
				Desc:     "计划的标题",
				Type:     schema.String,
				Required: true,
			},
			"description": {
				Desc:     "计划的详细描述",
				Type:     schema.String,
				Required: true,
			},
			"goal": {
				Desc:     "计划要达成的目标",
				Type:     schema.String,
				Required: true,
			},
		}),
	}, nil
}

func (t *CreatePlanTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params CreatePlanParams
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}
	
	return CreatePlanFunc(ctx, &params)
}

func NewCreatePlanTool() tool.InvokableTool {
	return &CreatePlanTool{}
}

func CreatePlanFunc(ctx context.Context, params *CreatePlanParams) (string, error) {
	planID := fmt.Sprintf("plan_%d", time.Now().Unix())
	
	plan := &Plan{
		ID:          planID,
		Title:       params.Title,
		Description: params.Description,
		Goal:        params.Goal,
		Tasks:       []Task{},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	plans[planID] = plan
	
	result := fmt.Sprintf("已成功创建计划: %s (ID: %s)\n目标: %s\n描述: %s", 
		plan.Title, plan.ID, plan.Goal, plan.Description)
	
	return result, nil
}

// AddTaskTool 添加任务工具
type AddTaskTool struct{}

func (t *AddTaskTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "add_task",
		Desc: "向指定计划添加任务",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"plan_id": {
				Desc:     "计划ID",
				Type:     schema.String,
				Required: true,
			},
			"content": {
				Desc:     "任务内容",
				Type:     schema.String,
				Required: true,
			},
			"priority": {
				Desc:     "任务优先级: high/medium/low",
				Type:     schema.String,
				Required: true,
			},
			"estimated_time": {
				Desc:     "预估完成时间(分钟)",
				Type:     schema.Integer,
				Required: true,
			},
			"dependencies": {
				Desc:     "依赖的任务ID列表",
				Type:     schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.String},
				Required: false,
			},
			"tags": {
				Desc:     "任务标签",
				Type:     schema.Array,
				ElemInfo: &schema.ParameterInfo{Type: schema.String},
				Required: false,
			},
		}),
	}, nil
}

func (t *AddTaskTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params AddTaskParams
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}
	
	return AddTaskFunc(ctx, &params)
}

func NewAddTaskTool() tool.InvokableTool {
	return &AddTaskTool{}
}

func AddTaskFunc(ctx context.Context, params *AddTaskParams) (string, error) {
	plan, exists := plans[params.PlanID]
	if !exists {
		return "", fmt.Errorf("计划不存在: %s", params.PlanID)
	}
	
	taskID := fmt.Sprintf("task_%d", time.Now().Unix())
	
	task := Task{
		ID:           taskID,
		Content:      params.Content,
		Priority:     params.Priority,
		Status:       "planned",
		Dependencies: params.Dependencies,
		EstimatedTime: params.EstimatedTime,
		Tags:         params.Tags,
	}
	
	plan.Tasks = append(plan.Tasks, task)
	tasks[taskID] = &task
	plan.UpdatedAt = time.Now()
	
	result := fmt.Sprintf("已向计划 '%s' 添加任务: %s (ID: %s)\n优先级: %s, 预估时间: %d分钟", 
		plan.Title, task.Content, task.ID, task.Priority, task.EstimatedTime)
	
	return result, nil
}

// AnalyzeGoalTool 分析目标工具
type AnalyzeGoalTool struct{}

func (t *AnalyzeGoalTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "analyze_goal",
		Desc: "分析目标的可行性、复杂度和分解建议",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"goal": {
				Desc:     "需要分析的目标",
				Type:     schema.String,
				Required: true,
			},
			"context": {
				Desc:     "相关上下文信息",
				Type:     schema.String,
				Required: false,
			},
			"constraints": {
				Desc:     "约束条件",
				Type:     schema.String,
				Required: false,
			},
		}),
	}, nil
}

func (t *AnalyzeGoalTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params AnalyzeGoalParams
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}
	
	return AnalyzeGoalFunc(ctx, &params)
}

func NewAnalyzeGoalTool() tool.InvokableTool {
	return &AnalyzeGoalTool{}
}

func AnalyzeGoalFunc(ctx context.Context, params *AnalyzeGoalParams) (string, error) {
	analysis := strings.Builder{}
	
	analysis.WriteString(fmt.Sprintf("目标分析: %s\n\n", params.Goal))
	
	// 简单的目标分析逻辑 (实际项目中可以集成AI分析)
	goalLen := len(params.Goal)
	complexity := "简单"
	if goalLen > 50 {
		complexity = "中等"
	}
	if goalLen > 100 {
		complexity = "复杂"
	}
	
	analysis.WriteString(fmt.Sprintf("复杂度评估: %s\n", complexity))
	
	// 关键词分析
	keywords := []string{"实现", "开发", "设计", "测试", "部署", "优化", "学习", "研究"}
	foundKeywords := []string{}
	goalLower := strings.ToLower(params.Goal)
	
	for _, keyword := range keywords {
		if strings.Contains(goalLower, keyword) {
			foundKeywords = append(foundKeywords, keyword)
		}
	}
	
	if len(foundKeywords) > 0 {
		analysis.WriteString(fmt.Sprintf("识别关键活动: %s\n", strings.Join(foundKeywords, ", ")))
	}
	
	// 分解建议
	analysis.WriteString("\n分解建议:\n")
	if strings.Contains(goalLower, "开发") || strings.Contains(goalLower, "实现") {
		analysis.WriteString("1. 需求分析和设计\n2. 核心功能开发\n3. 测试验证\n4. 部署上线\n")
	} else if strings.Contains(goalLower, "学习") {
		analysis.WriteString("1. 制定学习计划\n2. 收集学习资源\n3. 系统性学习\n4. 实践验证\n")
	} else {
		analysis.WriteString("1. 详细需求分析\n2. 制定实施策略\n3. 分步骤执行\n4. 结果验证\n")
	}
	
	if params.Context != "" {
		analysis.WriteString(fmt.Sprintf("\n上下文考虑: %s\n", params.Context))
	}
	
	if params.Constraints != "" {
		analysis.WriteString(fmt.Sprintf("\n约束条件: %s\n", params.Constraints))
	}
	
	return analysis.String(), nil
}

// ViewPlanTool 查看计划工具
type ViewPlanTool struct{}

func (t *ViewPlanTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "view_plan",
		Desc: "查看指定计划的详细信息和任务列表",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"plan_id": {
				Desc:     "计划ID",
				Type:     schema.String,
				Required: true,
			},
		}),
	}, nil
}

func (t *ViewPlanTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}
	
	return ViewPlanFunc(ctx, params)
}

func NewViewPlanTool() tool.InvokableTool {
	return &ViewPlanTool{}
}

func ViewPlanFunc(ctx context.Context, params map[string]interface{}) (string, error) {
	planID, ok := params["plan_id"].(string)
	if !ok {
		return "", fmt.Errorf("无效的计划ID")
	}
	
	plan, exists := plans[planID]
	if !exists {
		return "", fmt.Errorf("计划不存在: %s", planID)
	}
	
	result := strings.Builder{}
	result.WriteString(fmt.Sprintf("计划: %s\n", plan.Title))
	result.WriteString(fmt.Sprintf("ID: %s\n", plan.ID))
	result.WriteString(fmt.Sprintf("目标: %s\n", plan.Goal))
	result.WriteString(fmt.Sprintf("描述: %s\n", plan.Description))
	result.WriteString(fmt.Sprintf("创建时间: %s\n", plan.CreatedAt.Format("2006-01-02 15:04:05")))
	result.WriteString(fmt.Sprintf("更新时间: %s\n\n", plan.UpdatedAt.Format("2006-01-02 15:04:05")))
	
	result.WriteString("任务列表:\n")
	if len(plan.Tasks) == 0 {
		result.WriteString("  暂无任务\n")
	} else {
		for i, task := range plan.Tasks {
			result.WriteString(fmt.Sprintf("  %d. [%s] %s (ID: %s)\n", 
				i+1, task.Priority, task.Content, task.ID))
			result.WriteString(fmt.Sprintf("     状态: %s, 预估时间: %d分钟\n", 
				task.Status, task.EstimatedTime))
			if len(task.Dependencies) > 0 {
				result.WriteString(fmt.Sprintf("     依赖: %s\n", strings.Join(task.Dependencies, ", ")))
			}
			if len(task.Tags) > 0 {
				result.WriteString(fmt.Sprintf("     标签: %s\n", strings.Join(task.Tags, ", ")))
			}
			result.WriteString("\n")
		}
	}
	
	return result.String(), nil
}

// UpdateTaskTool 更新任务工具
type UpdateTaskTool struct{}

func (t *UpdateTaskTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "update_task",
		Desc: "更新任务状态",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"task_id": {
				Desc:     "任务ID",
				Type:     schema.String,
				Required: true,
			},
			"status": {
				Desc:     "新的任务状态: planned/in_progress/completed",
				Type:     schema.String,
				Required: true,
			},
		}),
	}, nil
}

func (t *UpdateTaskTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var params UpdateTaskParams
	if err := json.Unmarshal([]byte(argumentsInJSON), &params); err != nil {
		return "", fmt.Errorf("参数解析失败: %w", err)
	}
	
	return UpdateTaskFunc(ctx, &params)
}

func NewUpdateTaskTool() tool.InvokableTool {
	return &UpdateTaskTool{}
}

func UpdateTaskFunc(ctx context.Context, params *UpdateTaskParams) (string, error) {
	task, exists := tasks[params.TaskID]
	if !exists {
		return "", fmt.Errorf("任务不存在: %s", params.TaskID)
	}
	
	oldStatus := task.Status
	if params.Status != nil {
		task.Status = *params.Status
		
		// 更新时间记录
		now := time.Now()
		if *params.Status == "in_progress" && task.StartTime == nil {
			task.StartTime = &now
		}
		if *params.Status == "completed" && task.EndTime == nil {
			task.EndTime = &now
		}
	}
	
	result := fmt.Sprintf("任务 '%s' 状态已从 '%s' 更新为 '%s'", 
		task.Content, oldStatus, task.Status)
	
	return result, nil
}

// 获取所有planning工具
func GetPlanningTools() []tool.BaseTool {
	planningTools := []tool.BaseTool{
		NewCreatePlanTool(),
		NewAddTaskTool(),
		NewAnalyzeGoalTool(),
		NewViewPlanTool(),
		NewUpdateTaskTool(),
	}

	// 尝试添加向量搜索工具
	if vectorTool, err := NewVectorSearchTool(); err == nil {
		planningTools = append(planningTools, vectorTool)
	}

	// 添加章节读取工具
	planningTools = append(planningTools, NewNovelReadChapterTool())

	return planningTools
}