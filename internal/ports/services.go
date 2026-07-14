package ports

import (
	"context"

	"github.com/021labs-ai/synthetic-worlds/internal/domain"
)

// SyntheticService defines synthetic world management operations.
type SyntheticService interface {
	CreateWorld(ctx context.Context, auth *domain.AuthContext, params domain.CreateWorldParams) (*domain.WorldResponse, error)
	ExecuteCall(ctx context.Context, auth *domain.AuthContext, req domain.CallRequest) (*domain.CallResult, error)
	SeedFixtures(ctx context.Context, auth *domain.AuthContext, worldID string, fixtures []domain.Fixture) (int, error)
	InvokeReplay(ctx context.Context, auth *domain.AuthContext, worldID, toolName string, input map[string]any) (*domain.CallResult, error)
	ResetWorld(ctx context.Context, auth *domain.AuthContext, worldID string, hard bool) (*domain.ResetWorldResponse, error)
	CloseWorld(ctx context.Context, auth *domain.AuthContext, worldID string) (*domain.DeleteWorldResponse, error)
	ListWorlds(ctx context.Context, auth *domain.AuthContext, params domain.WorldListParams) (*domain.WorldListResult, error)
	ListCalls(ctx context.Context, auth *domain.AuthContext, params domain.CallListParams) (*domain.CallListResult, error)
	GetWorld(ctx context.Context, auth *domain.AuthContext, worldID string) (*domain.WorldDetailResponse, error)
	GetWorldState(ctx context.Context, auth *domain.AuthContext, worldID string) (*domain.TaskState, error)
}

// LLMClient defines operations for LLM structured output generation.
type LLMClient interface {
	GenerateStructuredOutput(ctx context.Context, model, systemPrompt, userPrompt string, returnSchema map[string]any, temperature float64) (map[string]any, error)
}

// ExampleProvider supplies domain context and tool examples from imported traces.
// In standalone mode, backed by PostgresExampleProvider (queries imported traces).
// In hosted mode, backed by QueryServiceExampleProvider (queries live trace pipeline).
type ExampleProvider interface {
	// GetWorldContext returns a product context string summarizing recent tool usage.
	GetWorldContext(ctx context.Context, projectID string) (string, error)
	// GetToolExamples returns real input/output example pairs for a specific tool.
	GetToolExamples(ctx context.Context, projectID, toolName string, limit int) ([]ToolExample, error)
}

// ToolExample represents a real-world tool call example (input/output pair).
type ToolExample struct {
	Input  any `json:"input"`
	Output any `json:"output"`
}

// AuthProvider authenticates incoming requests.
// StaticAuthProvider: compares against a static API key (standalone).
// DatabaseAuthProvider: validates against API key database (hosted).
type AuthProvider interface {
	Authenticate(ctx context.Context, token string) (*domain.AuthContext, error)
}

// TraceImportService defines trace import operations.
type TraceImportService interface {
	ImportNative(ctx context.Context, batch domain.TraceImportBatch) (*domain.TraceImportResult, error)
	ImportLangfuse(ctx context.Context, data []byte) (*domain.TraceImportResult, error)
	ImportLangsmith(ctx context.Context, data []byte) (*domain.TraceImportResult, error)
}
