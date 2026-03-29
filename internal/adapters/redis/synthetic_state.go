package redis

import (
	"context"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"github.com/021labs-ai/synthetic-worlds/internal/domain"
)

const (
	syntheticWorldPrefix = "synthetic:world:"
	syntheticCachePrefix = "synthetic:cache:"
)

type SyntheticState struct {
	client *Client
}

func NewSyntheticState(client *Client) *SyntheticState {
	return &SyntheticState{client: client}
}

func worldKey(worldID string) string {
	return syntheticWorldPrefix + worldID
}

func cacheKey(worldID, idempotencyKey string) string {
	return syntheticCachePrefix + worldID + ":" + idempotencyKey
}

func (s *SyntheticState) CreateWorld(ctx context.Context, worldID string, state *domain.WorldState, ttl time.Duration) error {
	key := worldKey(worldID)
	rdb := s.client.Redis()

	fields := map[string]any{
		"org_id":          state.OrganizationID,
		"project_id":      state.ProjectID,
		"api_key_id":      state.APIKeyID,
		"mode":            state.Mode,
		"seed":            state.Seed,
		"model":           state.Model,
		"failure_profile": state.FailureProfile,
		"world_context":   state.WorldContext,
		"task_state":      "{}",
		"step_count":      "0",
		"created_at":      state.CreatedAt,
		"last_access_at":  state.LastAccessAt,
	}

	if err := rdb.HSet(ctx, key, fields).Err(); err != nil {
		return fmt.Errorf("failed to create world in Redis: %w", err)
	}
	if err := rdb.Expire(ctx, key, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set world TTL: %w", err)
	}

	return nil
}

func (s *SyntheticState) GetWorld(ctx context.Context, worldID string) (*domain.WorldState, error) {
	rdb := s.client.Redis()
	data, err := rdb.HGetAll(ctx, worldKey(worldID)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get world from Redis: %w", err)
	}
	if len(data) == 0 {
		return nil, nil
	}

	return &domain.WorldState{
		OrganizationID: data["org_id"],
		ProjectID:      data["project_id"],
		APIKeyID:       data["api_key_id"],
		Mode:           data["mode"],
		Seed:           data["seed"],
		Model:          data["model"],
		FailureProfile: data["failure_profile"],
		WorldContext:   data["world_context"],
		TaskState:      data["task_state"],
		StepCount:      data["step_count"],
		CreatedAt:      data["created_at"],
		LastAccessAt:   data["last_access_at"],
	}, nil
}

func (s *SyntheticState) RefreshTTL(ctx context.Context, worldID string, ttl time.Duration) error {
	key := worldKey(worldID)
	rdb := s.client.Redis()

	now := time.Now().UTC().Format(time.RFC3339Nano)
	if err := rdb.HSet(ctx, key, "last_access_at", now).Err(); err != nil {
		return fmt.Errorf("failed to update last_access_at: %w", err)
	}
	if err := rdb.Expire(ctx, key, ttl).Err(); err != nil {
		return fmt.Errorf("failed to refresh TTL: %w", err)
	}

	return nil
}

func (s *SyntheticState) IncrStepCount(ctx context.Context, worldID string) (int64, error) {
	rdb := s.client.Redis()
	val, err := rdb.HIncrBy(ctx, worldKey(worldID), "step_count", 1).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to increment step_count: %w", err)
	}
	return val, nil
}

func (s *SyntheticState) GetCachedResponse(ctx context.Context, worldID, idempotencyKey string) (string, error) {
	rdb := s.client.Redis()
	raw, err := rdb.Get(ctx, cacheKey(worldID, idempotencyKey)).Result()
	if err != nil {
		if err == goredis.Nil {
			return "", nil
		}
		return "", fmt.Errorf("failed to get cached response: %w", err)
	}
	return raw, nil
}

func (s *SyntheticState) SetCachedResponse(ctx context.Context, worldID, idempotencyKey, response string, ttl time.Duration) error {
	rdb := s.client.Redis()
	if err := rdb.Set(ctx, cacheKey(worldID, idempotencyKey), response, ttl).Err(); err != nil {
		return fmt.Errorf("failed to set cached response: %w", err)
	}
	return nil
}

func (s *SyntheticState) GetTaskState(ctx context.Context, worldID string) (string, error) {
	rdb := s.client.Redis()
	raw, err := rdb.HGet(ctx, worldKey(worldID), "task_state").Result()
	if err != nil {
		if err == goredis.Nil {
			return "{}", nil
		}
		return "", fmt.Errorf("failed to get task_state: %w", err)
	}
	if raw == "" {
		return "{}", nil
	}
	return raw, nil
}

func (s *SyntheticState) SetTaskState(ctx context.Context, worldID string, taskState string) error {
	rdb := s.client.Redis()
	if err := rdb.HSet(ctx, worldKey(worldID), "task_state", taskState).Err(); err != nil {
		return fmt.Errorf("failed to set task_state: %w", err)
	}
	return nil
}

func (s *SyntheticState) DeleteWorld(ctx context.Context, worldID string) error {
	rdb := s.client.Redis()
	if err := rdb.Del(ctx, worldKey(worldID)).Err(); err != nil {
		return fmt.Errorf("failed to delete world: %w", err)
	}
	// Clean up cache keys
	pattern := syntheticCachePrefix + worldID + ":*"
	var cursor uint64
	for {
		keys, nextCursor, err := rdb.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("failed to scan cache keys: %w", err)
		}
		if len(keys) > 0 {
			if err := rdb.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("failed to delete cache keys: %w", err)
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return nil
}
