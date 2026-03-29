package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/021labs-ai/synthetic-worlds/internal/domain"
)

type TraceRepository struct {
	client *Client
}

func NewTraceRepository(client *Client) *TraceRepository {
	return &TraceRepository{client: client}
}

func (r *TraceRepository) InsertTrace(ctx context.Context, trace *domain.Trace) error {
	if trace.ID == "" {
		trace.ID = uuid.New().String()
	}
	query := `INSERT INTO traces (id, name) VALUES ($1, $2) RETURNING created_at`
	return r.client.pool.QueryRow(ctx, query, trace.ID, trace.Name).Scan(&trace.CreatedAt)
}

func (r *TraceRepository) InsertToolSpans(ctx context.Context, spans []domain.ToolSpan) error {
	if len(spans) == 0 {
		return nil
	}

	batch := &pgx.Batch{}
	for _, span := range spans {
		if span.ID == "" {
			span.ID = uuid.New().String()
		}
		batch.Queue(
			`INSERT INTO tool_spans (id, trace_id, name, input, output) VALUES ($1, $2, $3, $4, $5)`,
			span.ID, span.TraceID, span.Name, span.Input, span.Output,
		)
	}

	br := r.client.pool.SendBatch(ctx, batch)
	defer br.Close()

	for range spans {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("failed to insert tool span: %w", err)
		}
	}

	return nil
}

func (r *TraceRepository) ListTraces(ctx context.Context, limit, offset int) ([]domain.Trace, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `SELECT id, name, created_at FROM traces ORDER BY created_at DESC LIMIT $1 OFFSET $2`
	rows, err := r.client.pool.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list traces: %w", err)
	}
	defer rows.Close()

	var traces []domain.Trace
	for rows.Next() {
		var t domain.Trace
		if err := rows.Scan(&t.ID, &t.Name, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan trace: %w", err)
		}
		traces = append(traces, t)
	}
	return traces, nil
}

func (r *TraceRepository) ListToolSpansByTraceName(ctx context.Context, toolName string, limit int) ([]domain.ToolSpan, error) {
	if limit <= 0 {
		limit = 5
	}
	query := `SELECT id, trace_id, name, input, output, created_at FROM tool_spans WHERE name = $1 ORDER BY created_at DESC LIMIT $2`
	rows, err := r.client.pool.Query(ctx, query, toolName, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list tool spans: %w", err)
	}
	defer rows.Close()

	var spans []domain.ToolSpan
	for rows.Next() {
		var s domain.ToolSpan
		if err := rows.Scan(&s.ID, &s.TraceID, &s.Name, &s.Input, &s.Output, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan tool span: %w", err)
		}
		spans = append(spans, s)
	}
	return spans, nil
}

func (r *TraceRepository) GetRecentToolSpans(ctx context.Context, limit int) ([]domain.ToolSpan, error) {
	if limit <= 0 {
		limit = 20
	}
	query := `SELECT id, trace_id, name, input, output, created_at FROM tool_spans ORDER BY created_at DESC LIMIT $1`
	rows, err := r.client.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent tool spans: %w", err)
	}
	defer rows.Close()

	var spans []domain.ToolSpan
	for rows.Next() {
		var s domain.ToolSpan
		if err := rows.Scan(&s.ID, &s.TraceID, &s.Name, &s.Input, &s.Output, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan tool span: %w", err)
		}
		spans = append(spans, s)
	}
	return spans, nil
}
