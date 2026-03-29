package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/021labs-ai/synthetic-worlds/internal/domain"
)

// SyntheticWorldRepository implements ports.SyntheticWorldRepository.
type SyntheticWorldRepository struct {
	client *Client
}

// NewSyntheticWorldRepository creates a new SyntheticWorldRepository.
func NewSyntheticWorldRepository(client *Client) *SyntheticWorldRepository {
	return &SyntheticWorldRepository{client: client}
}

// Create creates a new synthetic world record.
func (r *SyntheticWorldRepository) Create(ctx context.Context, world *domain.SyntheticWorld) error {
	if world.ID == "" {
		world.ID = uuid.New().String()
	}

	fpJSON, err := json.Marshal(world.FailureProfile)
	if err != nil {
		return fmt.Errorf("failed to marshal failure_profile: %w", err)
	}

	query := `
		INSERT INTO synthetic_worlds (id, organization_id, project_id, api_key_id, mode, seed, model, failure_profile, status, total_calls)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at
	`

	err = r.client.pool.QueryRow(ctx, query,
		world.ID,
		world.OrganizationID,
		world.ProjectID,
		world.APIKeyID,
		world.Mode,
		world.Seed,
		world.Model,
		fpJSON,
		world.Status,
		world.TotalCalls,
	).Scan(&world.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create synthetic world: %w", err)
	}

	return nil
}

// GetByID retrieves a synthetic world by ID.
func (r *SyntheticWorldRepository) GetByID(ctx context.Context, id string) (*domain.SyntheticWorld, error) {
	query := `
		SELECT id, organization_id, project_id, api_key_id, mode, seed, model, failure_profile, status, total_calls, created_at, closed_at
		FROM synthetic_worlds
		WHERE id = $1
	`

	return r.scanWorld(ctx, query, id)
}

// List lists synthetic worlds with filtering.
func (r *SyntheticWorldRepository) List(ctx context.Context, params domain.WorldListParams) (*domain.WorldListResult, error) {
	conditions := "organization_id = $1"
	args := []any{params.OrganizationID}
	argIdx := 2

	if params.ProjectID != "" {
		conditions += fmt.Sprintf(" AND project_id = $%d", argIdx)
		args = append(args, params.ProjectID)
		argIdx++
	}
	if params.Status != "" {
		conditions += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, params.Status)
		argIdx++
	}

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM synthetic_worlds WHERE %s", conditions)
	var total int64
	if err := r.client.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count synthetic worlds: %w", err)
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
		SELECT id, organization_id, project_id, api_key_id, mode, seed, model, failure_profile, status, total_calls, created_at, closed_at
		FROM synthetic_worlds
		WHERE %s
		ORDER BY created_at DESC
		LIMIT $%d OFFSET $%d
	`, conditions, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.client.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list synthetic worlds: %w", err)
	}
	defer rows.Close()

	var worlds []domain.SyntheticWorld
	for rows.Next() {
		w, err := r.scanWorldFromRow(rows)
		if err != nil {
			return nil, err
		}
		worlds = append(worlds, *w)
	}

	if worlds == nil {
		worlds = []domain.SyntheticWorld{}
	}

	return &domain.WorldListResult{
		Worlds:  worlds,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
		HasMore: int64(offset+len(worlds)) < total,
	}, nil
}

// Close marks a synthetic world as closed.
func (r *SyntheticWorldRepository) Close(ctx context.Context, id string) error {
	query := `
		UPDATE synthetic_worlds
		SET status = 'closed', closed_at = NOW()
		WHERE id = $1 AND status = 'active'
	`

	result, err := r.client.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to close synthetic world: %w", err)
	}
	if result.RowsAffected() == 0 {
		return fmt.Errorf("synthetic world not found or already closed")
	}

	return nil
}

// IncrementCalls atomically increments the total_calls counter.
func (r *SyntheticWorldRepository) IncrementCalls(ctx context.Context, id string) error {
	query := `UPDATE synthetic_worlds SET total_calls = total_calls + 1 WHERE id = $1`
	_, err := r.client.pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to increment calls: %w", err)
	}
	return nil
}

func (r *SyntheticWorldRepository) scanWorld(ctx context.Context, query string, arg any) (*domain.SyntheticWorld, error) {
	row := r.client.pool.QueryRow(ctx, query, arg)
	w := &domain.SyntheticWorld{}
	var fpJSON []byte

	err := row.Scan(
		&w.ID, &w.OrganizationID, &w.ProjectID, &w.APIKeyID,
		&w.Mode, &w.Seed, &w.Model, &fpJSON,
		&w.Status, &w.TotalCalls, &w.CreatedAt, &w.ClosedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to scan synthetic world: %w", err)
	}

	if len(fpJSON) > 0 && string(fpJSON) != "null" {
		if err := json.Unmarshal(fpJSON, &w.FailureProfile); err != nil {
			return nil, fmt.Errorf("failed to unmarshal failure_profile: %w", err)
		}
	}

	return w, nil
}

func (r *SyntheticWorldRepository) scanWorldFromRow(rows pgx.Rows) (*domain.SyntheticWorld, error) {
	w := &domain.SyntheticWorld{}
	var fpJSON []byte

	err := rows.Scan(
		&w.ID, &w.OrganizationID, &w.ProjectID, &w.APIKeyID,
		&w.Mode, &w.Seed, &w.Model, &fpJSON,
		&w.Status, &w.TotalCalls, &w.CreatedAt, &w.ClosedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to scan synthetic world row: %w", err)
	}

	if len(fpJSON) > 0 && string(fpJSON) != "null" {
		if err := json.Unmarshal(fpJSON, &w.FailureProfile); err != nil {
			return nil, fmt.Errorf("failed to unmarshal failure_profile: %w", err)
		}
	}

	return w, nil
}

// worldNow is used for testing overrides.
var worldNow = time.Now
