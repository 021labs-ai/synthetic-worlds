package ports

import (
	"context"
	"time"

	"github.com/021labs-ai/synthetic-worlds/internal/domain"
)

// SyntheticWorldRepository defines operations for synthetic world persistence.
type SyntheticWorldRepository interface {
	Create(ctx context.Context, world *domain.SyntheticWorld) error
	GetByID(ctx context.Context, id string) (*domain.SyntheticWorld, error)
	List(ctx context.Context, params domain.WorldListParams) (*domain.WorldListResult, error)
	Close(ctx context.Context, id string) error
	IncrementCalls(ctx context.Context, id string) error
}

// SyntheticCallRepository defines operations for synthetic call persistence.
type SyntheticCallRepository interface {
	Insert(ctx context.Context, call *domain.SyntheticCall) error
	List(ctx context.Context, params domain.CallListParams) (*domain.CallListResult, error)
	ListByWorldID(ctx context.Context, orgID, worldID string, limit, offset int) (*domain.CallListResult, error)
	GetWorldStats(ctx context.Context, worldID string) (*domain.WorldStats, error)
}

// TraceRepository defines operations for imported trace persistence.
type TraceRepository interface {
	InsertTrace(ctx context.Context, trace *domain.Trace) error
	InsertToolSpans(ctx context.Context, spans []domain.ToolSpan) error
	ListTraces(ctx context.Context, limit, offset int) ([]domain.Trace, error)
	ListToolSpansByTraceName(ctx context.Context, toolName string, limit int) ([]domain.ToolSpan, error)
	GetRecentToolSpans(ctx context.Context, limit int) ([]domain.ToolSpan, error)
}

// SyntheticStateAdapter defines ephemeral state operations (Redis).
type SyntheticStateAdapter interface {
	CreateWorld(ctx context.Context, worldID string, state *domain.WorldState, ttl time.Duration) error
	GetWorld(ctx context.Context, worldID string) (*domain.WorldState, error)
	RefreshTTL(ctx context.Context, worldID string, ttl time.Duration) error
	IncrStepCount(ctx context.Context, worldID string) (int64, error)
	GetFixtures(ctx context.Context, worldID string) (string, error)
	SetFixtures(ctx context.Context, worldID string, fixtures string) error
	GetTaskState(ctx context.Context, worldID string) (string, error)
	SetTaskState(ctx context.Context, worldID string, taskState string) error
	GetCachedResponse(ctx context.Context, worldID, idempotencyKey string) (string, error)
	SetCachedResponse(ctx context.Context, worldID, idempotencyKey, response string, ttl time.Duration) error
	DeleteWorld(ctx context.Context, worldID string) error
}
