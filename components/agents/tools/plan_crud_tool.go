package tools

import (
	"context"
	"fmt"

	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/Kizunad/modular-workflow-v2/components/content/managers"
)

// PlanCRUDTool 计划增删改查工具，基于simple_tools.go优化，确保正确实现BaseTool接口
type PlanCRUDTool struct {
	novelDir string
}

// NewPlanCRUDTool 创建计划CRUD工具
func NewPlanCRUDTool(novelDir string) *PlanCRUDTool {
	return &PlanCRUDTool{
		novelDir: novelDir,
	}
}

// Info 实现BaseTool接口，使用简化的工具信息定义
func (t *PlanCRUDTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "plan_crud",
		Desc: "规划管理工具，用于创建、读取、更新、删除章节规划",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"action": {
				Type:     schema.String,
				Desc:     "操作类型: create/read/update/delete/set_finished/list/get_unfinished",
				Required: true,
			},
			"chapter": {
				Type:     schema.String,
				Desc:     "章节编号",
				Required: false,
			},
			"plan": {
				Type:     schema.String,
				Desc:     "规划标题(除标题外不能存放任何内容)",
				Required: false,
			},
			"content": {
				Type:     schema.String,
				Desc:     "规划内容",
				Required: false,
			},
			"finished": {
				Type:     schema.Boolean,
				Desc:     "是否已完成",
				Required: false,
			},
		}),
	}, nil
}

// InvokableRun 实现InvokableTool接口，保留原有的完整实现逻辑
func (t *PlanCRUDTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	// 解析输入参数
	var input map[string]interface{}
	if err := sonic.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", compose.NewInterruptAndRerunErr("JSON参数解析失败，请检查格式并重新调用。原始参数: " + argumentsInJSON + "，错误: " + err.Error())
	}

	// 调用内部方法处理
	return t.invoke(ctx, input)
}

// invoke 内部调用方法，保留原有逻辑
func (t *PlanCRUDTool) invoke(_ context.Context, input map[string]any) (string, error) {
	action, ok := input["action"].(string)
	if !ok {
		return "", compose.NewInterruptAndRerunErr("缺少必要参数 action（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	// 创建规划内容管理器
	pcm := managers.NewPlannerContentManager(t.novelDir)

	switch action {
	case "create":
		return t.handleCreate(pcm, input)
	case "read":
		return t.handleRead(pcm, input)
	case "update":
		return t.handleUpdate(pcm, input)
	case "delete":
		return t.handleDelete(pcm, input)
	case "set_finished":
		return t.handleSetFinished(pcm, input)
	case "list":
		return t.handleList(pcm)
	case "get_unfinished":
		return t.handleGetUnfinished(pcm)
	default:
		return "", compose.NewInterruptAndRerunErr("不支持的操作类型: " + action + "，支持的操作: create/read/update/delete/set_finished/list/get_unfinished，当前参数: " + fmt.Sprintf("%v", input))
	}
}

// handleCreate 处理创建章节
func (t *PlanCRUDTool) handleCreate(pcm *managers.PlannerContentManager, input map[string]any) (string, error) {
	chapter, chapterOk := input["chapter"].(string)
	plan, planOk := input["plan"].(string)
	content, contentOk := input["content"].(string)

	if !chapterOk || chapter == "" {
		return "", compose.NewInterruptAndRerunErr("创建计划需要提供有效的 chapter 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	if !planOk || plan == "" {
		return "", compose.NewInterruptAndRerunErr("创建计划需要提供有效的 plan 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	if !contentOk || content == "" {
		return "", compose.NewInterruptAndRerunErr("创建计划需要提供有效的 content 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	// 实际创建计划 -- Create plan 只能是 未 Finished，不然没意义
	err := pcm.UpsertPlan(chapter, plan, content, false)
	if err != nil {
		return "", fmt.Errorf("%s", fmt.Sprintf("创建计划失败: %v", err))
	}

	finished := false
	// 构建成功响应
	planEntry := &managers.PlanEntry{
		Chapter:  chapter,
		Plan:     plan,
		Content:  content,
		Finished: finished,
	}

	return t.successResponse("计划创建成功", planEntry, nil), nil
}

// handleRead 处理读取章节
func (t *PlanCRUDTool) handleRead(pcm *managers.PlannerContentManager, input map[string]any) (string, error) {
	chapter, chapterOk := input["chapter"].(string)
	if !chapterOk || chapter == "" {
		return "", compose.NewInterruptAndRerunErr("读取计划需要提供有效的 chapter 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	// 读取完整的计划条目
	planEntry, exists := pcm.GetPlanEntry(chapter)
	if !exists {
		return "", fmt.Errorf("%s", fmt.Sprintf("计划不存在: %s", chapter))
	}

	return t.successResponse("计划读取成功", planEntry, nil), nil
}

// handleUpdate 处理更新章节
func (t *PlanCRUDTool) handleUpdate(pcm *managers.PlannerContentManager, input map[string]any) (string, error) {
	chapter, chapterOk := input["chapter"].(string)
	content, contentOk := input["content"].(string)
	plan, planOk := input["plan"].(string)
	finished, finishedOk := input["finished"].(bool)

	if !chapterOk || chapter == "" {
		return "", compose.NewInterruptAndRerunErr("更新计划需要提供有效的 chapter 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	// 检查计划是否存在
	existingPlan, exists := pcm.GetPlanEntry(chapter)
	if !exists {
		return "", compose.NewInterruptAndRerunErr("计划不存在，无法更新。如需创建新计划请使用create操作，章节: " + chapter + "，当前参数: " + fmt.Sprintf("%v", input))
	}

	// 防止意外覆盖：如果要修改plan或content且提供了新值，给出警告
	if (planOk && plan != "" && plan != existingPlan.Plan) || (contentOk && content != "" && content != existingPlan.Content) {
		return "", compose.NewInterruptAndRerunErr("检测到尝试覆盖现有计划内容！如需创建新计划，请使用create操作并使用新的章节编号。当前章节: " + chapter + "，当前参数: " + fmt.Sprintf("%v", input))
	}

	// 只允许更新finished状态和补充空字段
	if !contentOk || content == "" {
		content = existingPlan.Content
	}

	if !planOk || plan == "" {
		plan = existingPlan.Plan
	}

	if !finishedOk {
		finished = existingPlan.Finished
	}

	// 实际更新计划
	err := pcm.UpsertPlan(chapter, plan, content, finished)
	if err != nil {
		return "", fmt.Errorf("%s", fmt.Sprintf("更新计划失败: %v", err))
	}

	// 构建响应
	planEntry := &managers.PlanEntry{
		Chapter:  chapter,
		Plan:     plan,
		Content:  content,
		Finished: finished,
	}

	return t.successResponse("计划更新成功", planEntry, nil), nil
}

// handleDelete 处理删除章节
func (t *PlanCRUDTool) handleDelete(pcm *managers.PlannerContentManager, input map[string]any) (string, error) {
	chapter, chapterOk := input["chapter"].(string)
	if !chapterOk || chapter == "" {
		return "", compose.NewInterruptAndRerunErr("删除计划需要提供有效的 chapter 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	// 实际删除计划
	err := pcm.DeletePlan(chapter)
	if err != nil {
		return "", fmt.Errorf("%s", fmt.Sprintf("删除计划失败: %v", err))
	}

	return t.successResponse("计划删除成功", nil, nil), nil
}

// handleSetFinished 处理设置完成状态
func (t *PlanCRUDTool) handleSetFinished(pcm *managers.PlannerContentManager, input map[string]any) (string, error) {
	chapter, chapterOk := input["chapter"].(string)
	finished, finishedOk := input["finished"].(bool)

	if !chapterOk || chapter == "" {
		return "", compose.NewInterruptAndRerunErr("设置完成状态需要提供有效的 chapter 参数（字符串类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	if !finishedOk {
		return "", compose.NewInterruptAndRerunErr("设置完成状态需要提供有效的 finished 参数（布尔类型），当前参数: " + fmt.Sprintf("%v", input))
	}

	// 实际设置状态
	err := pcm.SetPlanFinished(chapter, finished)
	if err != nil {
		return "", fmt.Errorf("%s", fmt.Sprintf("设置完成状态失败: %v", err))
	}

	return t.successResponse("完成状态设置成功", nil, nil), nil
}

// handleList 处理列表所有计划
func (t *PlanCRUDTool) handleList(pcm *managers.PlannerContentManager) (string, error) {
	plans := pcm.GetAllPlans()
	return t.successResponse("计划列表获取成功", nil, plans), nil
}

// handleGetUnfinished 处理获取未完成计划
func (t *PlanCRUDTool) handleGetUnfinished(pcm *managers.PlannerContentManager) (string, error) {
	plans := pcm.GetUnfinishedPlans()
	return t.successResponse("未完成计划获取成功", nil, plans), nil
}

// successResponse 构建成功响应
func (t *PlanCRUDTool) successResponse(message string, data *managers.PlanEntry, plans []managers.PlanEntry) string {
	response := map[string]interface{}{
		"success": true,
		"message": message,
	}

	if data != nil {
		response["data"] = data
	}

	if plans != nil {
		response["plans"] = plans
		response["count"] = len(plans)
	}

	jsonBytes, _ := sonic.MarshalIndent(response, "", "  ")
	return string(jsonBytes)
}
