package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/021labs-ai/synthetic-worlds/internal/domain"
)

const (
	stateUpdaterModel  = "claude-haiku-4-5"
	maxHistoryEntries  = 20
	maxStateBytes      = 64 * 1024 // 64KB
	stateUpdateTimeout = 15 * time.Second
)

// getTaskState reads and deserializes the task state from Redis.
func (s *SyntheticService) getTaskState(ctx context.Context, worldID string) (*domain.TaskState, error) {
	raw, err := s.state.GetTaskState(ctx, worldID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task state: %w", err)
	}

	if raw == "" || raw == "{}" {
		return domain.NewTaskState(), nil
	}

	var ts domain.TaskState
	if err := json.Unmarshal([]byte(raw), &ts); err != nil {
		s.log.Warn("failed to parse task state, returning empty", zap.Error(err))
		return domain.NewTaskState(), nil
	}

	if ts.Entities == nil {
		ts.Entities = make(map[string][]map[string]any)
	}
	if ts.RuntimeVars == nil {
		ts.RuntimeVars = make(map[string]any)
	}

	return &ts, nil
}

// updateTaskStateAsync fires a goroutine to update the task state after a call completes.
func (s *SyntheticService) updateTaskStateAsync(
	worldID string,
	toolName string,
	input map[string]any,
	output map[string]any,
	stepCount int,
	currentState *domain.TaskState,
) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), stateUpdateTimeout)
		defer cancel()

		if err := s.updateTaskState(ctx, worldID, toolName, input, output, stepCount, currentState); err != nil {
			s.log.Error("async task state update failed",
				zap.String("world_id", worldID),
				zap.String("tool_name", toolName),
				zap.Error(err),
			)
		}
	}()
}

// updateTaskState asks the LLM for a state patch, then merges it in Go.
func (s *SyntheticService) updateTaskState(
	ctx context.Context,
	worldID string,
	toolName string,
	input map[string]any,
	output map[string]any,
	stepCount int,
	currentState *domain.TaskState,
) error {
	freshState, err := s.getTaskState(ctx, worldID)
	if err != nil {
		s.log.Warn("failed to re-read task state, using passed state", zap.Error(err))
		freshState = currentState
	}

	stateJSON, _ := json.Marshal(freshState)
	inputJSON, _ := json.Marshal(input)
	outputJSON, _ := json.Marshal(output)

	prompt := fmt.Sprintf(`Current task state:
%s

Tool call just completed:
  Tool: %s
  Step: %d
  Input: %s
  Output: %s

Produce a state PATCH — only the changes. Do NOT return the full state.`,
		string(stateJSON), toolName, stepCount, string(inputJSON), string(outputJSON))

	result, err := s.llm.GenerateStructuredOutput(ctx, stateUpdaterModel, stateUpdaterSystemPrompt, prompt, nil, 0.0)
	if err != nil {
		return fmt.Errorf("state updater LLM call failed: %w", err)
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal LLM result: %w", err)
	}

	var patch domain.StatePatch
	if err := json.Unmarshal(resultJSON, &patch); err != nil {
		return fmt.Errorf("failed to parse state patch: %w", err)
	}

	applyPatch(freshState, &patch, toolName, stepCount)

	if len(freshState.History) > maxHistoryEntries {
		freshState.History = freshState.History[len(freshState.History)-maxHistoryEntries:]
	}

	finalJSON, err := json.Marshal(freshState)
	if err != nil {
		return fmt.Errorf("failed to serialize updated state: %w", err)
	}

	if len(finalJSON) > maxStateBytes {
		if len(freshState.History) > 2 {
			freshState.History = freshState.History[len(freshState.History)/2:]
			finalJSON, _ = json.Marshal(freshState)
		}
		for len(finalJSON) > maxStateBytes && len(freshState.History) > 1 {
			freshState.History = freshState.History[1:]
			finalJSON, _ = json.Marshal(freshState)
		}
		if len(finalJSON) > maxStateBytes {
			s.log.Warn("task state exceeds 64KB after trimming history",
				zap.String("world_id", worldID),
				zap.Int("size_bytes", len(finalJSON)),
			)
		}
	}

	return s.state.SetTaskState(ctx, worldID, string(finalJSON))
}

// applyPatch merges a StatePatch into a TaskState deterministically.
func applyPatch(state *domain.TaskState, patch *domain.StatePatch, toolName string, stepCount int) {
	for category, entities := range patch.UpsertEntities {
		existing := state.Entities[category]
		for _, newEnt := range entities {
			newID, _ := newEnt["id"].(string)
			if newID == "" {
				existing = append(existing, newEnt)
				continue
			}
			found := false
			for i, old := range existing {
				if oldID, _ := old["id"].(string); oldID == newID {
					for k, v := range newEnt {
						existing[i][k] = v
					}
					found = true
					break
				}
			}
			if !found {
				existing = append(existing, newEnt)
			}
		}
		state.Entities[category] = existing
	}

	for category, idsToDelete := range patch.DeleteEntities {
		existing := state.Entities[category]
		deleteSet := make(map[string]bool, len(idsToDelete))
		for _, id := range idsToDelete {
			deleteSet[id] = true
		}
		filtered := existing[:0]
		for _, ent := range existing {
			if id, _ := ent["id"].(string); !deleteSet[id] {
				filtered = append(filtered, ent)
			}
		}
		if len(filtered) == 0 {
			delete(state.Entities, category)
		} else {
			state.Entities[category] = filtered
		}
	}

	for k, v := range patch.SetVars {
		state.RuntimeVars[k] = v
	}

	for _, k := range patch.DeleteVars {
		delete(state.RuntimeVars, k)
	}

	if patch.HistoryEntry != "" {
		state.History = append(state.History, domain.StateHistoryEntry{
			Step:     stepCount,
			ToolName: toolName,
			Action:   patch.HistoryEntry,
		})
	}
}

// generateTaskFeedback assesses workflow progress based on the current task state.
func (s *SyntheticService) generateTaskFeedback(
	ctx context.Context,
	taskState *domain.TaskState,
	worldContext string,
) *domain.TaskFeedback {
	if taskState == nil || (len(taskState.Entities) == 0 && len(taskState.History) == 0) {
		return nil
	}

	stateJSON, _ := json.Marshal(taskState)

	prompt := fmt.Sprintf(`Product context:
%s

Current task state:
%s

Assess the workflow progress. What has been completed? What typically remains?
Return JSON: {"task_complete": bool, "completed_items": [...], "remaining_items": [...], "progress_percent": 0-100}`,
		worldContext, string(stateJSON))

	result, err := s.llm.GenerateStructuredOutput(ctx, stateUpdaterModel, taskFeedbackSystemPrompt, prompt, nil, 0.0)
	if err != nil {
		s.log.Warn("task feedback generation failed", zap.Error(err))
		return nil
	}

	resultJSON, _ := json.Marshal(result)
	var feedback domain.TaskFeedback
	if err := json.Unmarshal(resultJSON, &feedback); err != nil {
		s.log.Warn("failed to parse task feedback", zap.Error(err))
		return nil
	}

	return &feedback
}
