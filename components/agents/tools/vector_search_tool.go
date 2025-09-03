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

// VectorSearchConfig 向量搜索工具配置
type VectorSearchConfig struct {
	SessionID string // 会话ID，必须注入
	Tenant    string // Chroma租户，注入
	Database  string // Chroma数据库，注入
}

// WithSearchConfig 创建向量搜索工具配置选项
func WithSearchConfig(sessionID, tenant, database string) tool.Option {
	return tool.WrapImplSpecificOptFn(func(config *VectorSearchConfig) {
		config.SessionID = sessionID
		config.Tenant = tenant
		config.Database = database
	})
}

// ResolveSearchCollection 基于注入的sessionID和contentType解析集合名称
func (config *VectorSearchConfig) ResolveSearchCollection(contentType string) string {
	// 验证sessionID格式，防止注入攻击
	if !isValidSearchSessionID(config.SessionID) {
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

// isValidSearchSessionID 验证会话ID格式
func isValidSearchSessionID(sessionID string) bool {
	if sessionID == "" {
		return false
	}
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]{3,50}$`, sessionID)
	return matched
}

// VectorSearchService 向量搜索服务
type VectorSearchService struct {
	embedder embedder.Embedder
}

// NewVectorSearchService 创建向量搜索服务实例
func NewVectorSearchService() (*VectorSearchService, error) {
	// 创建embedder
	emb, err := embedder.New("ollama").
		WithBaseURL("http://localhost:11434").
		WithModel("nomic-embed-text").
		Build()
	if err != nil {
		return nil, fmt.Errorf("failed to create embedder: %w", err)
	}

	return &VectorSearchService{
		embedder: emb,
	}, nil
}

// createSearchChromaClient 根据配置动态创建Chroma客户端
func (v *VectorSearchService) createSearchChromaClient(config *VectorSearchConfig, collection string) (chroma.VectorStore, error) {
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

	return chromaStore, nil
}

// SearchDocuments 搜索相似文档
func (v *VectorSearchService) SearchDocuments(ctx context.Context, config *VectorSearchConfig, query, contentType string, topK int, threshold float64, filters map[string]interface{}) ([]map[string]interface{}, error) {
	collection := config.ResolveSearchCollection(contentType)

	// 创建动态配置的客户端
	chromaStore, err := v.createSearchChromaClient(config, collection)
	if err != nil {
		return nil, err
	}
	defer chromaStore.Close()

	// 执行搜索
	var result *chroma.SearchResult
	if len(filters) > 0 {
		if client, ok := chromaStore.(*chroma.ChromaClient); ok {
			result, err = client.SearchWithFilter(ctx, query, filters, topK)
		} else {
			result, err = chromaStore.Search(ctx, query, topK)
		}
	} else {
		result, err = chromaStore.Search(ctx, query, topK)
	}

	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// 转换结果格式并应用阈值过滤
	var results []map[string]interface{}
	for _, doc := range result.Documents {
		// 应用相似度阈值过滤
		if float64(-doc.Score) >= threshold {
			resultItem := map[string]interface{}{
				"id":      doc.ID,
				"content": doc.Content,
				"score":   doc.Score,
			}

			if doc.Metadata != nil {
				resultItem["metadata"] = doc.Metadata
			}

			results = append(results, resultItem)
		}
	}

	return results, nil
}

// Health 健康检查
func (v *VectorSearchService) Health(ctx context.Context) error {
	return v.embedder.Health(ctx)
}

// VectorSearchTool 向量搜索工具
type VectorSearchTool struct {
	service *VectorSearchService
}

// NewVectorSearchTool 创建向量搜索工具
func NewVectorSearchTool() (*VectorSearchTool, error) {
	service, err := NewVectorSearchService()
	if err != nil {
		return nil, err
	}

	return &VectorSearchTool{
		service: service,
	}, nil
}

// Info 返回搜索工具信息
func (s *VectorSearchTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "vector_search",
		Desc: "Search for similar documents from vector database",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     "string",
				Desc:     "Search query text",
				Required: true,
			},
			"content_type": {
				Type:     "string", 
				Desc:     "Content type: chapter, summary, plan (default: chapter)",
				Required: false,
			},
			"top_k": {
				Type:     "integer",
				Desc:     "Number of documents to return (default: 5)",
				Required: false,
			},
			"threshold": {
				Type:     "number",
				Desc:     "Similarity threshold (default: 0.7)",
				Required: false,
			},
			"filters": {
				Type:     "object",
				Desc:     "Document filter conditions",
				Required: false,
			},
		}),
	}, nil
}

// InvokableRun 执行搜索操作
func (s *VectorSearchTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	start := time.Now()

	// 获取注入的配置
	config := tool.GetImplSpecificOptions(&VectorSearchConfig{
		Tenant:   "novel_system", // 默认值
		Database: "novel_db",     // 默认值
	}, opts...)

	// 验证必须的注入参数
	if config.SessionID == "" {
		return "", fmt.Errorf("session_id must be injected via workflow configuration")
	}

	var input struct {
		Query       string                 `json:"query"`
		ContentType string                 `json:"content_type"`
		TopK        int                    `json:"top_k"`
		Threshold   float64                `json:"threshold"`
		Filters     map[string]interface{} `json:"filters,omitempty"`
	}

	if err := SafeParseJSON(argumentsInJSON, &input); err != nil {
		return BuildErrorResponse(fmt.Errorf("failed to parse arguments: %w", err))
	}

	if err := ValidateStringParam(input.Query, "query", true); err != nil {
		return BuildErrorResponse(err)
	}

	// 设置默认值
	if input.ContentType == "" {
		input.ContentType = "chapter"
	}
	if input.TopK <= 0 {
		input.TopK = 5
	}
	if input.Threshold <= 0 {
		input.Threshold = 0.7
	}

	// 验证数值参数
	if err := ValidateIntParam(input.TopK, "top_k", 1, 100); err != nil {
		return BuildErrorResponse(err)
	}

	results, err := s.service.SearchDocuments(ctx, config, input.Query, input.ContentType, input.TopK, input.Threshold, input.Filters)
	if err != nil {
		return BuildErrorResponse(err, "Vector search failed")
	}

	_ = time.Since(start) // Duration recorded but metrics removed

	data := map[string]interface{}{
		"documents":   results,
		"total_found": len(results),
		"query":       input.Query,
		"collection":  config.ResolveSearchCollection(input.ContentType),
		"session_id":  config.SessionID,
	}

	return BuildSuccessResponse(data)
}