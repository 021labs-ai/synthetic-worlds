package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/021labs-ai/synthetic-worlds/internal/domain"
)

// SyntheticCallRepository implements ports.SyntheticCallRepository using Postgres.
type SyntheticCallRepository struct {
	client *Client
}

// NewSyntheticCallRepository creates a new SyntheticCallRepository.
func NewSyntheticCallRepository(client *Client) *SyntheticCallRepository {
	return &SyntheticCallRepository{client: client}
}

// Insert inserts a single synthetic call record.
func (r *SyntheticCallRepository) Insert(ctx context.Context, call *domain.SyntheticCall) error {
	query := `
		INSERT INTO synthetic_calls (
			id, world_id, organization_id, project_id,
			tool_name, tool_schema, input_data, output_data,
			step_count, mode_used, cache_hit,
			model_used, idempotency_key, duration_ms, error,
			validation_data, feedback_data
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
	`

	_, err := r.client.pool.Exec(ctx, query,
		call.ID,
		call.WorldID,
		call.OrganizationID,
		call.ProjectID,
		call.ToolName,
		call.ToolSchema,
		call.InputData,
		call.OutputData,
		call.StepCount,
		call.ModeUsed,
		call.CacheHit,
		call.ModelUsed,
		call.IdempotencyKey,
		call.DurationMs,
		call.Error,
		call.ValidationData,
		call.FeedbackData,
	)
	if err != nil {
		return fmt.Errorf("failed to insert synthetic call: %w", err)
	}

	return nil
}

// List lists synthetic calls with filtering.
func (r *SyntheticCallRepository) List(ctx context.Context, params domain.CallListParams) (*domain.CallListResult, error) {
	conditions := "organization_id = $1"
	args := []any{params.OrganizationID}
	argIdx := 2

	if params.ProjectID != "" {
		conditions += fmt.Sprintf(" AND project_id = $%d", argIdx)
		args = append(args, params.ProjectID)
		argIdx++
	}
	if params.WorldID != "" {
		conditions += fmt.Sprintf(" AND world_id = $%d", argIdx)
		args = append(args, params.WorldID)
		argIdx++
	}
	if params.ToolName != "" {
		conditions += fmt.Sprintf(" AND tool_name = $%d", argIdx)
		args = append(args, params.ToolName)
		argIdx++
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM synthetic_calls WHERE %s", conditions)
	var total int64
	if err := r.client.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count synthetic calls: %w", err)
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
		SELECT id, world_id, organization_id, project_id,
			tool_name, tool_schema, input_data, output_data,
			step_count, mode_used, cache_hit,
			model_used, idempotency_key, duration_ms, error,
			validation_data, feedback_data, created_at
		FROM synthetic_calls
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, conditions, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.client.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list synthetic calls: %w", err)
	}
	defer rows.Close()

	var calls []domain.SyntheticCall
	for rows.Next() {
		c, err := r.scanCallFromRow(rows)
		if err != nil {
			return nil, err
		}
		calls = append(calls, *c)
	}

	if calls == nil {
		calls = []domain.SyntheticCall{}
	}

	return &domain.CallListResult{
		Calls:   calls,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
		HasMore: int64(offset+len(calls)) < total,
	}, nil
}

// ListByWorldID is a convenience method to list calls for a specific world.
func (r *SyntheticCallRepository) ListByWorldID(ctx context.Context, orgID, worldID string, limit, offset int) (*domain.CallListResult, error) {
	return r.List(ctx, domain.CallListParams{
		OrganizationID: orgID,
		WorldID:        worldID,
		Limit:          limit,
		Offset:         offset,
	})
}

// GetWorldStats aggregates statistics for a world's calls.
func (r *SyntheticCallRepository) GetWorldStats(ctx context.Context, worldID string) (*domain.WorldStats, error) {
	query := `
		SELECT
			COUNT(*) as total_calls,
			AVG(duration_ms) as avg_duration_ms,
			COUNT(*) FILTER (WHERE cache_hit = true) as cache_hits,
			COUNT(*) FILTER (WHERE error IS NOT NULL AND error != '') as error_count
		FROM synthetic_calls
		WHERE world_id = $1
	`

	var totalCalls int
	var avgDurationMs *float64
	var cacheHits int
	var errorCount int

	if err := r.client.pool.QueryRow(ctx, query, worldID).Scan(
		&totalCalls, &avgDurationMs, &cacheHits, &errorCount,
	); err != nil {
		return nil, fmt.Errorf("failed to get world stats: %w", err)
	}

	// Get tool counts
	toolQuery := `
		SELECT tool_name, COUNT(*) as cnt
		FROM synthetic_calls
		WHERE world_id = $1
		GROUP BY tool_name
	`
	rows, err := r.client.pool.Query(ctx, toolQuery, worldID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tool counts: %w", err)
	}
	defer rows.Close()

	toolCounts := make(map[string]int)
	for rows.Next() {
		var toolName string
		var cnt int
		if err := rows.Scan(&toolName, &cnt); err != nil {
			return nil, fmt.Errorf("failed to scan tool count: %w", err)
		}
		toolCounts[toolName] = cnt
	}

	var cacheHitRate float64
	if totalCalls > 0 {
		cacheHitRate = float64(cacheHits) / float64(totalCalls)
	}

	avgDuration := 0.0
	if avgDurationMs != nil {
		avgDuration = *avgDurationMs
	}

	return &domain.WorldStats{
		TotalCalls:    totalCalls,
		ToolCounts:    toolCounts,
		AvgDurationMs: avgDuration,
		CacheHitRate:  cacheHitRate,
		ErrorCount:    errorCount,
	}, nil
}

func (r *SyntheticCallRepository) scanCallFromRow(rows pgx.Rows) (*domain.SyntheticCall, error) {
	var c domain.SyntheticCall
	if err := rows.Scan(
		&c.ID, &c.WorldID, &c.OrganizationID, &c.ProjectID,
		&c.ToolName, &c.ToolSchema, &c.InputData, &c.OutputData,
		&c.StepCount, &c.ModeUsed, &c.CacheHit,
		&c.ModelUsed, &c.IdempotencyKey, &c.DurationMs, &c.Error,
		&c.ValidationData, &c.FeedbackData, &c.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to scan synthetic call: %w", err)
	}
	return &c, nil
}
