package agents

import (
	"context"
	"fmt"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"go.uber.org/zap"

	"github.com/Kizunad/modular-workflow-v2/logger"
)

// DialogueAgent 定义对话Agent接口，符合eino规范
type DialogueAgent interface {
	compose.Runnable[[]*schema.Message, *schema.Message] // 符合eino的Runnable接口
	GetName() string
	GetRole() string
}

// DialogueTurn 对话轮次信息
type DialogueTurn struct {
	TurnNumber int             `json:"turn_number"`
	Speaker    string          `json:"speaker"`
	Message    *schema.Message `json:"message"`
	Timestamp  time.Time       `json:"timestamp"`
}

// DialogueContext 对话上下文
type DialogueContext struct {
	SessionID         string          `json:"session_id"`
	History           []*DialogueTurn `json:"history"`
	CurrentTurn       int             `json:"current_turn"`
	MaxTurns          int             `json:"max_turns"`
	IsActive          bool            `json:"is_active"`
	TerminationReason string          `json:"termination_reason,omitempty"`
}

// DialogueCoordinator 对话协调器，符合eino编排规范
type DialogueCoordinator struct {
	agentA  DialogueAgent
	agentB  DialogueAgent
	context *DialogueContext
	logger  *logger.ZapLogger

	// 符合eino Graph编排
	graph compose.Runnable[*DialogueContext, *schema.Message]
}

// NewDialogueCoordinator 创建对话协调器
func NewDialogueCoordinator(
	agentA, agentB DialogueAgent,
	sessionID string,
	maxTurns int,
	logger *logger.ZapLogger,
) (*DialogueCoordinator, error) {

	if agentA == nil || agentB == nil {
		return nil, fmt.Errorf("both agents must be provided")
	}

	if maxTurns <= 0 {
		maxTurns = 10 // 默认最大轮次
	}

	coordinator := &DialogueCoordinator{
		agentA: agentA,
		agentB: agentB,
		context: &DialogueContext{
			SessionID:   sessionID,
			History:     make([]*DialogueTurn, 0),
			CurrentTurn: 0,
			MaxTurns:    maxTurns,
			IsActive:    true,
		},
		logger: logger,
	}

	// 构建eino Graph
	if err := coordinator.buildGraph(); err != nil {
		return nil, fmt.Errorf("failed to build dialogue graph: %w", err)
	}

	return coordinator, nil
}

// buildGraph 构建符合eino规范的对话Graph
func (d *DialogueCoordinator) buildGraph() error {
	// 创建Graph
	graph := compose.NewGraph[*DialogueContext, *schema.Message]()

	// 添加Lambda节点
	_ = graph.AddLambdaNode("init_dialogue", compose.InvokableLambda(d.initDialogue))
	_ = graph.AddLambdaNode("agent_a_turn", compose.InvokableLambda(d.agentATurn))
	_ = graph.AddLambdaNode("agent_b_turn", compose.InvokableLambda(d.agentBTurn))
	_ = graph.AddLambdaNode("check_continuation", compose.InvokableLambda(d.checkContinuation))
	_ = graph.AddLambdaNode("finalize_dialogue", compose.InvokableLambda(d.finalizeDialogue))

	// 建立Graph边连接
	_ = graph.AddEdge(compose.START, "init_dialogue")
	_ = graph.AddEdge("init_dialogue", "agent_a_turn")
	_ = graph.AddEdge("agent_a_turn", "agent_b_turn")
	_ = graph.AddEdge("agent_b_turn", "check_continuation")

	// 添加条件分支 - 修正 API 用法
	// endNodes 定义所有可能的目标节点
	endNodes := map[string]bool{
		"agent_a_turn":       true,
		"finalize_dialogue": true,
	}
	branch := compose.NewGraphBranch(d.shouldContinue, endNodes)
	_ = graph.AddBranch("check_continuation", branch)

	_ = graph.AddEdge("finalize_dialogue", compose.END)

	// 编译Graph
	compiledGraph, err := graph.Compile(context.Background())
	if err != nil {
		return fmt.Errorf("failed to compile dialogue graph: %w", err)
	}

	d.graph = compiledGraph
	return nil
}

// Invoke 执行对话，符合eino Runnable接口
func (d *DialogueCoordinator) Invoke(ctx context.Context, input []*schema.Message, opts ...compose.Option) (*schema.Message, error) {
	d.logger.Info("开始执行对话协调",
		zap.String("session_id", d.context.SessionID),
		zap.String("agent_a", d.agentA.GetName()),
		zap.String("agent_b", d.agentB.GetName()))

	// 设置初始消息到上下文
	if len(input) > 0 {
		d.context.History = append(d.context.History, &DialogueTurn{
			TurnNumber: 0,
			Speaker:    "user",
			Message:    input[0],
			Timestamp:  time.Now(),
		})
	}

	// 使用Graph执行对话流程
	result, err := d.graph.Invoke(ctx, d.context, opts...)
	if err != nil {
		d.logger.Error("对话执行失败", zap.Error(err))
		return nil, err
	}

	d.logger.Info("对话执行完成",
		zap.Int("total_turns", len(d.context.History)),
		zap.String("termination_reason", d.context.TerminationReason))

	return result, nil
}

// Stream 流式执行对话
func (d *DialogueCoordinator) Stream(ctx context.Context, input []*schema.Message, opts ...compose.Option) (*schema.StreamReader[*schema.Message], error) {
	return d.graph.Stream(ctx, d.context, opts...)
}

// Collect 收集流式输入并执行对话
func (d *DialogueCoordinator) Collect(ctx context.Context, input *schema.StreamReader[[]*schema.Message], opts ...compose.Option) (*schema.Message, error) {
	// TODO: 需要将输入的消息流转换为对话上下文
	panic("not implemented: Collect method needs proper stream conversion")
}

// Transform 流式转换对话
func (d *DialogueCoordinator) Transform(ctx context.Context, input *schema.StreamReader[[]*schema.Message], opts ...compose.Option) (*schema.StreamReader[*schema.Message], error) {
	// TODO: 需要将输入的消息流转换为对话上下文流
	panic("not implemented: Transform method needs proper stream conversion")
}

// initDialogue 初始化对话
func (d *DialogueCoordinator) initDialogue(ctx context.Context, input *DialogueContext) (*DialogueContext, error) {
	d.logger.Info("初始化对话", zap.String("session_id", input.SessionID))
	return input, nil
}

// agentATurn Agent A的回合
func (d *DialogueCoordinator) agentATurn(ctx context.Context, input *DialogueContext) (*DialogueContext, error) {
	if !input.IsActive {
		return input, nil
	}

	// 准备消息历史
	messages := d.buildMessageHistory(input.History)
	
	// 调用真实的Agent A
	agentResponse, err := d.agentA.Invoke(ctx, messages)
	if err != nil {
		// 如果调用失败，使用占位符回应
		d.logger.Warn("Agent A调用失败，使用占位符", zap.Error(err))
		agentResponse = &schema.Message{
			Role:    schema.Assistant,
			Content: fmt.Sprintf("[Agent %s Turn %d]: Agent调用失败，占位符回应", d.agentA.GetName(), input.CurrentTurn+1),
		}
	}

	// 记录对话轮次
	input.CurrentTurn++
	input.History = append(input.History, &DialogueTurn{
		TurnNumber: input.CurrentTurn,
		Speaker:    d.agentA.GetName(),
		Message:    agentResponse,
		Timestamp:  time.Now(),
	})

	d.logger.Info("Agent A 完成回合",
		zap.Int("turn", input.CurrentTurn),
		zap.String("agent", d.agentA.GetName()))

	return input, nil
}

// agentBTurn Agent B的回合
func (d *DialogueCoordinator) agentBTurn(ctx context.Context, input *DialogueContext) (*DialogueContext, error) {
	if !input.IsActive {
		return input, nil
	}

	// 准备消息历史
	messages := d.buildMessageHistory(input.History)
	
	// 调用真实的Agent B
	agentResponse, err := d.agentB.Invoke(ctx, messages)
	if err != nil {
		// 如果调用失败，使用占位符回应
		d.logger.Warn("Agent B调用失败，使用占位符", zap.Error(err))
		agentResponse = &schema.Message{
			Role:    schema.Assistant,
			Content: fmt.Sprintf("[Agent %s Turn %d]: Agent调用失败，占位符回应", d.agentB.GetName(), input.CurrentTurn+1),
		}
	}

	// 记录对话轮次
	input.CurrentTurn++
	input.History = append(input.History, &DialogueTurn{
		TurnNumber: input.CurrentTurn,
		Speaker:    d.agentB.GetName(),
		Message:    agentResponse,
		Timestamp:  time.Now(),
	})

	d.logger.Info("Agent B 完成回合",
		zap.Int("turn", input.CurrentTurn),
		zap.String("agent", d.agentB.GetName()))

	return input, nil
}

// checkContinuation 检查是否继续对话
func (d *DialogueCoordinator) checkContinuation(ctx context.Context, input *DialogueContext) (*DialogueContext, error) {
	return input, nil
}

// shouldContinue 判断对话是否应该继续
func (d *DialogueCoordinator) shouldContinue(ctx context.Context, input *DialogueContext) (string, error) {
	// 检查最大轮次限制
	if input.CurrentTurn >= input.MaxTurns {
		input.IsActive = false
		input.TerminationReason = "max_turns_reached"
		return "finalize_dialogue", nil
	}

	// TODO: 添加更智能的终止条件判断
	// 例如：检查对话是否达成共识、是否出现重复等

	return "agent_a_turn", nil
}

// finalizeDialogue 完成对话
func (d *DialogueCoordinator) finalizeDialogue(ctx context.Context, input *DialogueContext) (*schema.Message, error) {
	input.IsActive = false

	// 构建最终响应
	summary := fmt.Sprintf("对话完成！总轮次: %d, 终止原因: %s",
		len(input.History), input.TerminationReason)

	response := &schema.Message{
		Role:    schema.Assistant,
		Content: summary,
	}

	d.logger.Info("对话已完成",
		zap.Int("total_turns", len(input.History)),
		zap.String("termination_reason", input.TerminationReason))

	return response, nil
}

// buildMessageHistory 构建消息历史
func (d *DialogueCoordinator) buildMessageHistory(history []*DialogueTurn) []*schema.Message {
	messages := make([]*schema.Message, 0, len(history))
	for _, turn := range history {
		messages = append(messages, turn.Message)
	}
	return messages
}

// GetDialogueHistory 获取对话历史
func (d *DialogueCoordinator) GetDialogueHistory() []*DialogueTurn {
	return d.context.History
}

// GetContext 获取对话上下文
func (d *DialogueCoordinator) GetContext() *DialogueContext {
	return d.context
}

// ExportRunnable 导出内部Runnable，用于嵌入其他编排
func (d *DialogueCoordinator) ExportRunnable() (compose.Runnable[*DialogueContext, *schema.Message], error) {
	if d.graph == nil {
		return nil, fmt.Errorf("graph not initialized")
	}
	return d.graph, nil
}