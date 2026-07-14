package services

// Deterministic replay: per-world fixtures seeded via the API and served
// VERBATIM on invoke — no LLM anywhere in this path. Built for RL grading,
// where the response to a matched call must be byte-identical to what the
// task generator computed ("inputs which are given, outputs known in
// advance"). Unmatched calls return a deterministic 404 so an agent hitting
// an unseeded endpoint fails reproducibly. Every invoke is persisted to the
// call log, which is the grading surface for the agent's WRITE calls.

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/021labs-ai/synthetic-worlds/internal/domain"
	"github.com/021labs-ai/synthetic-worlds/pkg/apierror"
	"go.uber.org/zap"
)

// SeedFixtures stores the world's fixtures (replacing any previous set).
func (s *SyntheticService) SeedFixtures(
	ctx context.Context, auth *domain.AuthContext, worldID string, fixtures []domain.Fixture,
) (int, error) {
	world, err := s.state.GetWorld(ctx, worldID)
	if err != nil {
		return 0, apierror.Internal(fmt.Sprintf("failed to get world: %v", err))
	}
	if world == nil {
		return 0, apierror.NotFound("World")
	}
	if world.OrganizationID != auth.OrganizationID {
		return 0, apierror.Forbidden("World belongs to a different organization")
	}
	for i := range fixtures {
		if fixtures[i].ToolName == "" {
			return 0, apierror.Validation("fixture tool_name is required", nil)
		}
	}
	data, err := json.Marshal(fixtures)
	if err != nil {
		return 0, apierror.Validation(fmt.Sprintf("fixtures not serializable: %v", err), nil)
	}
	if err := s.state.SetFixtures(ctx, worldID, string(data)); err != nil {
		return 0, apierror.Internal(fmt.Sprintf("failed to store fixtures: %v", err))
	}
	return len(fixtures), nil
}

// InvokeReplay serves a tool call from the world's fixtures, verbatim.
func (s *SyntheticService) InvokeReplay(
	ctx context.Context, auth *domain.AuthContext, worldID, toolName string, input map[string]any,
) (*domain.CallResult, error) {
	start := time.Now()

	world, err := s.state.GetWorld(ctx, worldID)
	if err != nil {
		return nil, apierror.Internal(fmt.Sprintf("failed to get world: %v", err))
	}
	if world == nil {
		return nil, apierror.NotFound("World")
	}
	if world.OrganizationID != auth.OrganizationID {
		return nil, apierror.Forbidden("World belongs to a different organization")
	}
	if err := s.state.RefreshTTL(ctx, worldID, s.worldTTL); err != nil {
		s.log.Warn("failed to refresh world TTL", zap.Error(err))
	}
	stepCount, err := s.state.IncrStepCount(ctx, worldID)
	if err != nil {
		return nil, apierror.Internal(fmt.Sprintf("failed to increment step count: %v", err))
	}

	raw, err := s.state.GetFixtures(ctx, worldID)
	if err != nil {
		s.log.Warn("fixture lookup failed", zap.Error(err))
	}
	var fixtures []domain.Fixture
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &fixtures); err != nil {
			return nil, apierror.Internal(fmt.Sprintf("stored fixtures corrupt: %v", err))
		}
	}

	req := domain.CallRequest{
		WorldID:    worldID,
		ToolName:   toolName,
		Input:      input,
		ToolSchema: map[string]any{"name": toolName, "description": "replay invoke"},
	}
	durationMs := uint32(time.Since(start).Milliseconds())

	output, matched := MatchFixture(fixtures, toolName, input)
	replay := "replay"
	if !matched {
		errMsg := fmt.Sprintf("no fixture for tool %q with the given input", toolName)
		go s.persistCall(req, auth, world, map[string]any{"error": "not_found"},
			int(stepCount), false, &replay, &durationMs, &errMsg, nil, nil)
		return nil, apierror.NotFound(fmt.Sprintf("fixture for %s", toolName))
	}

	go s.persistCall(req, auth, world, output, int(stepCount), false, &replay,
		&durationMs, nil, nil, nil)
	return &domain.CallResult{
		Output:    output,
		StepCount: int(stepCount),
		CacheHit:  false,
		ModeUsed:  "replay",
		ModelUsed: &replay,
	}, nil
}

// MatchFixture picks the fixture for a call: exact tool_name, then the first
// fixture whose input matches the call input exactly (canonical JSON
// comparison); a fixture WITHOUT an input acts as the tool-level default.
// Pure function — unit-tested without Redis.
func MatchFixture(fixtures []domain.Fixture, toolName string, input map[string]any) (map[string]any, bool) {
	var fallback map[string]any
	haveFallback := false
	callKey := canonicalJSON(input)
	for _, f := range fixtures {
		if f.ToolName != toolName {
			continue
		}
		if f.Input == nil {
			if !haveFallback {
				fallback = f.Output
				haveFallback = true
			}
			continue
		}
		if canonicalJSON(f.Input) == callKey {
			return f.Output, true
		}
	}
	if haveFallback {
		return fallback, true
	}
	return nil, false
}

// canonicalJSON renders a value with sorted keys so semantically-equal inputs
// compare equal regardless of key order.
func canonicalJSON(v any) string {
	// encoding/json sorts map keys deterministically for map[string]any
	data, err := json.Marshal(normalize(v))
	if err != nil {
		return ""
	}
	return string(data)
}

// normalize forces all numbers through float64 (JSON default) so ints and
// floats with equal value compare equal after a marshal round-trip.
func normalize(v any) any {
	data, err := json.Marshal(v)
	if err != nil {
		return v
	}
	var out any
	if err := json.Unmarshal(data, &out); err != nil {
		return v
	}
	return out
}
