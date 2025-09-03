package common

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"github.com/Kizunad/modular-workflow-v2/logger"
)

// WriterState 写作状态
type WriterState struct {
	RetryCount int
	LastResult string
	SavedInput map[string]any
}

// WorkflowConfig 工作流配置
type WorkflowConfig struct {
	Logger           *logger.ZapLogger
	CheckPointStore  compose.CheckPointStore
	EnableProgress   bool // 是否启用进度输出
	ProgressPrefix   string
}

// DefaultWorkflowConfig 默认工作流配置
func DefaultWorkflowConfig() *WorkflowConfig {
	return &WorkflowConfig{
		Logger:         logger.New(),
		CheckPointStore: NewInMemoryCheckPointStore(),
		EnableProgress: true,
		ProgressPrefix: "📋",
	}
}

// WorkflowBuilder 通用工作流构建器
type WorkflowBuilder struct {
	config *WorkflowConfig
}

// NewWorkflowBuilder 创建工作流构建器
func NewWorkflowBuilder(config *WorkflowConfig) *WorkflowBuilder {
	if config == nil {
		config = DefaultWorkflowConfig()
	}
	return &WorkflowBuilder{config: config}
}

// CreateStepLambda 创建带进度提示的步骤Lambda
func (wb *WorkflowBuilder) CreateStepLambda(
	stepName string,
	stepDesc string,
	handler func(ctx context.Context, input interface{}) (interface{}, error),
) *compose.Lambda {
	return compose.InvokableLambda(func(ctx context.Context, input interface{}) (interface{}, error) {
		if wb.config.EnableProgress {
			fmt.Printf("\n%s %s...\n", wb.config.ProgressPrefix, stepDesc)
		}
		
		if wb.config.Logger != nil {
			wb.config.Logger.Info(fmt.Sprintf("开始执行步骤: %s", stepName))
		}
		
		result, err := handler(ctx, input)
		
		if err != nil {
			if wb.config.Logger != nil {
				wb.config.Logger.Error(fmt.Sprintf("步骤 %s 执行失败: %v", stepName, err))
			}
			return nil, fmt.Errorf("步骤 %s 失败: %w", stepName, err)
		}
		
		if wb.config.EnableProgress {
			fmt.Printf("✅ %s完成\n", stepDesc)
		}
		
		if wb.config.Logger != nil {
			wb.config.Logger.Info(fmt.Sprintf("步骤 %s 执行成功", stepName))
		}
		
		return result, nil
	})
}

// CreateGenericWorkflow 创建通用工作流
func (wb *WorkflowBuilder) CreateGenericWorkflow() *compose.Workflow[string, string] {
	return compose.NewWorkflow[string, string]()
}

// CompileWorkflow 编译工作流
func (wb *WorkflowBuilder) CompileWorkflow(workflow *compose.Workflow[string, string]) (compose.Runnable[string, string], error) {
	ctx := context.Background()
	
	var opts []compose.GraphCompileOption
	if wb.config.CheckPointStore != nil {
		opts = append(opts, compose.WithCheckPointStore(wb.config.CheckPointStore))
	}
	
	compiled, err := workflow.Compile(ctx, opts...)
	if err != nil {
		if wb.config.Logger != nil {
			wb.config.Logger.Error(fmt.Sprintf("工作流编译失败: %v", err))
		}
		return nil, fmt.Errorf("工作流编译失败: %w", err)
	}
	
	return compiled, nil
}

// CreateGraph 创建带状态的Graph
func (wb *WorkflowBuilder) CreateGraph() *compose.Graph[map[string]any, *schema.Message] {
	return compose.NewGraph[map[string]any, *schema.Message](
		compose.WithGenLocalState(func(ctx context.Context) *WriterState {
			return &WriterState{
				RetryCount: 0,
				LastResult: "",
				SavedInput: make(map[string]any),
			}
		}),
	)
}

// CompileGraph 编译Graph
func (wb *WorkflowBuilder) CompileGraph(graph *compose.Graph[map[string]any, *schema.Message]) (compose.Runnable[map[string]any, *schema.Message], error) {
	ctx := context.Background()
	
	var opts []compose.GraphCompileOption
	if wb.config.CheckPointStore != nil {
		opts = append(opts, compose.WithCheckPointStore(wb.config.CheckPointStore))
	}
	
	compiled, err := graph.Compile(ctx, opts...)
	if err != nil {
		if wb.config.Logger != nil {
			wb.config.Logger.Error(fmt.Sprintf("Graph编译失败: %v", err))
		}
		return nil, fmt.Errorf("Graph编译失败: %w", err)
	}
	
	return compiled, nil
}