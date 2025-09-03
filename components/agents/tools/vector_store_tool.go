package tools

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"

	chroma "github.com/Kizunad/modular-chroma"
	embedder "github.com/Kizunad/modular-embedder"
)

// VectorStoreConfig 向量存储工具配置
type VectorStoreConfig struct {
	SessionID string // 会话ID，必须注入
	Tenant    string // Chroma租户，注入
	Database  string // Chroma数据库，注入
}

// WithStoreConfig 创建向量存储工具配置选项
func WithStoreConfig(sessionID, tenant, database string) tool.Option {
	return tool.WrapImplSpecificOptFn(func(config *VectorStoreConfig) {
		config.SessionID = sessionID
		config.Tenant = tenant
		config.Database = database
	})
}

// ResolveStoreCollection 基于注入的sessionID和contentType解析集合名称
func (config *VectorStoreConfig) ResolveStoreCollection(contentType string) string {
	// 验证sessionID格式，防止注入攻击
	if !isValidStoreSessionID(config.SessionID) {
		return "default"
	}

	switch contentType {
	case "chapter", "":
		return fmt.Sprintf("novel_%s", config.SessionID)
	case "summary":
		return fmt.Sprintf("summary_%s", config.SessionID)
	case "plan":
		return fmt.Sprintf("plan_%s", config.SessionID)
	default:
		return fmt.Sprintf("novel_%s", config.SessionID)
	}
}

// isValidStoreSessionID 验证会话ID格式
func isValidStoreSessionID(sessionID string) bool {
	if sessionID == "" {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]{3,50}$`, sessionID)
	return matched
}

// VectorStoreService 向量存储服务
type VectorStoreService struct {
	embedder embedder.Embedder
}

// NewVectorStoreService 创建向量存储服务实例
func NewVectorStoreService() (*VectorStoreService, error) {
	// 创建embedder
	emb, err := embedder.New("ollama").
		WithBaseURL("http://localhost:11434").
		WithModel("nomic-embed-text").
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	return &VectorStoreService{
		embedder: emb,
	}, nil
}

// createStoreChromaClient 根据配置动态创建Chroma客户端
func (v *VectorStoreService) createStoreChromaClient(config *VectorStoreConfig, collection string) (chroma.VectorStore, error) {
	chromaStore, err := chroma.NewChromaStore(v.embedder).
		WithHost("http://localhost").
		WithPort(8000).
		WithTenant(config.Tenant).
		WithDatabase(config.Database).
		WithCollection(collection).
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create chroma client for collection %s: %w", collection, err)
	}

	// 确保租户、数据库和集合存在
	if client, ok := chromaStore.(*chroma.ChromaClient); ok {
		// 创建租户（如果不存在）
		if err := client.CreateTenant(context.Background(), config.Tenant); err != nil {
			// 忽略已存在的错误
		}

		// 创建数据库（如果不存在）
		if err := client.CreateDatabase(context.Background(), config.Tenant, config.Database); err != nil {
			// 忽略已存在的错误
		}

		// 创建集合（如果不存在）
		if err := client.CreateCollection(context.Background(), collection); err != nil {
			// 忽略已存在的错误
		}
	}

	return chromaStore, nil
}

// StoreDocument 存储单个文档到指定集合（支持更新）
func (v *VectorStoreService) StoreDocument(ctx context.Context, config *VectorStoreConfig, contentType, id, content string, metadata map[string]interface{}) error {
	collection := config.ResolveStoreCollection(contentType)

	// 创建动态配置的客户端
	chromaStore, err := v.createStoreChromaClient(config, collection)
	if err != nil {
		return err
	}
	defer chromaStore.Close()

	// 创建文档对象
	doc := chroma.Document{
		ID:       id,
		Content:  content,
		Metadata: metadata,
	}

	return chromaStore.Store(ctx, []chroma.Document{doc})
}

// UpdateDocument 更新现有文档
func (v *VectorStoreService) UpdateDocument(ctx context.Context, config *VectorStoreConfig, contentType, id, content string, metadata map[string]interface{}) error {
	collection := config.ResolveStoreCollection(contentType)

	// 创建动态配置的客户端
	chromaStore, err := v.createStoreChromaClient(config, collection)
	if err != nil {
		return err
	}
	defer chromaStore.Close()

	// 对于Chroma，更新和存储使用相同的操作（upsert语义）
	doc := chroma.Document{
		ID:       id,
		Content:  content,
		Metadata: metadata,
	}

	return chromaStore.Store(ctx, []chroma.Document{doc})
}

// DeleteDocument 删除文档
func (v *VectorStoreService) DeleteDocument(ctx context.Context, config *VectorStoreConfig, contentType, id string) error {
	collection := config.ResolveStoreCollection(contentType)

	// 创建动态配置的客户端
	chromaStore, err := v.createStoreChromaClient(config, collection)
	if err != nil {
		return err
	}
	defer chromaStore.Close()

	// 如果客户端支持删除操作
	if client, ok := chromaStore.(*chroma.ChromaClient); ok {
		return client.Delete(ctx, []string{id})
	}

	return fmt.Errorf("delete operation not supported")
}

// Health 健康检查
func (v *VectorStoreService) Health(ctx context.Context) error {
	return v.embedder.Health(ctx)
}

// VectorStoreTool 向量存储工具（支持存储、更新、删除）
type VectorStoreTool struct {
	service *VectorStoreService
}

// NewVectorStoreTool 创建向量存储工具
func NewVectorStoreTool() (*VectorStoreTool, error) {
	service, err := NewVectorStoreService()
	if err != nil {
		return nil, err
	}

	return &VectorStoreTool{
		service: service,
	}, nil
}

// Info 返回存储工具信息
func (s *VectorStoreTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "vector_store",
		Desc: "Store, update or delete documents in vector database",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"operation": {
				Type:     "string",
				Desc:     "Operation type: store, update, delete (default: store)",
				Required: false,
			},
			"content_type": {
				Type:     "string",
				Desc:     "Content type: chapter, summary, plan (default: chapter)",
				Required: false,
			},
			"documents": {
				Type:     "array",
				Desc:     "List of documents to process",
				Required: true,
				ElemInfo: &schema.ParameterInfo{
					Type: "object",
					SubParams: map[string]*schema.ParameterInfo{
						"id":       {Type: "string", Desc: "Document ID", Required: true},
						"content":  {Type: "string", Desc: "Document content (not required for delete)", Required: false},
						"metadata": {Type: "object", Desc: "Document metadata (not required for delete)", Required: false},
					},
				},
			},
		}),
	}, nil
}

// InvokableRun 执行存储操作
func (s *VectorStoreTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	start := time.Now()

	// 获取注入的配置
	config := tool.GetImplSpecificOptions(&VectorStoreConfig{
		Tenant:   "novel_system", // 默认值
		Database: "novel_db",     // 默认值
	}, opts...)

	// 验证必须的注入参数
	if config.SessionID == "" {
		return "", fmt.Errorf("session_id must be injected via workflow configuration")
	}

	var input struct {
		Operation   string `json:"operation"`
		ContentType string `json:"content_type"`
		Documents   []struct {
			ID       string                 `json:"id"`
			Content  string                 `json:"content,omitempty"`
			Metadata map[string]interface{} `json:"metadata,omitempty"`
		} `json:"documents"`
	}

	if err := SafeParseJSON(argumentsInJSON, &input); err != nil {
		return BuildErrorResponse(fmt.Errorf("failed to parse arguments: %w", err))
	}

	// 设置默认值
	if input.Operation == "" {
		input.Operation = "store"
	}
	if input.ContentType == "" {
		input.ContentType = "chapter"
	}

	var results []map[string]interface{}
	var successCount int
	var failedIDs []string

	// 记录操作总数（每个document视为一个单元操作）
	var totalOperations int

	for _, doc := range input.Documents {
		var err error
		
		// 操作类型验证和前置检查
		switch input.Operation {
		case "store":
			if doc.Content == "" {
				// Validation error removed
				failedIDs = append(failedIDs, doc.ID)
				results = append(results, map[string]interface{}{
					"id":    doc.ID,
					"error": "content cannot be empty for store operation",
				})
				continue
			}
			totalOperations++
			err = s.service.StoreDocument(ctx, config, input.ContentType, doc.ID, doc.Content, doc.Metadata)
			
		case "update":
			if doc.Content == "" {
				// Validation error removed
				failedIDs = append(failedIDs, doc.ID)
				results = append(results, map[string]interface{}{
					"id":    doc.ID,
					"error": "content cannot be empty for update operation",
				})
				continue
			}
			totalOperations++
			err = s.service.UpdateDocument(ctx, config, input.ContentType, doc.ID, doc.Content, doc.Metadata)
			
		case "delete":
			totalOperations++
			err = s.service.DeleteDocument(ctx, config, input.ContentType, doc.ID)
			
		default:
			// Validation error removed
			failedIDs = append(failedIDs, doc.ID)
			results = append(results, map[string]interface{}{
				"id":    doc.ID,
				"error": fmt.Sprintf("unsupported operation: %s", input.Operation),
			})
			continue
		}

		if err != nil {
			// Tool failure removed
			failedIDs = append(failedIDs, doc.ID)
			results = append(results, map[string]interface{}{
				"id":    doc.ID,
				"error": err.Error(),
			})
			continue
		}

		successCount++
		results = append(results, map[string]interface{}{
			"id":        doc.ID,
			"status":    "success",
			"operation": input.Operation,
		})
	}

	duration := time.Since(start)
	// 只有在至少有一次操作时才标记成功
	if successCount > 0 {
		_ = duration // Duration recorded but metrics removed
	} else if totalOperations > 0 && successCount == 0 {
		// 全部操作失败时标记为失败
		// RecordToolFailure 已在单个操作失败时记录，这里避免重复计数
	}

	data := map[string]interface{}{
		"status":         "completed",
		"operation":      input.Operation,
		"processed_count": successCount,
		"failed_count":   len(failedIDs),
		"failed_ids":     failedIDs,
		"results":        results,
		"collection":     config.ResolveStoreCollection(input.ContentType),
		"session_id":     config.SessionID,
	}

	return BuildSuccessResponse(data)
}