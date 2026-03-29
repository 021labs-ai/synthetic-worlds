package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/google/uuid"

	"github.com/021labs-ai/synthetic-worlds/internal/domain"
	"github.com/021labs-ai/synthetic-worlds/internal/ports"
	"github.com/021labs-ai/synthetic-worlds/pkg/apierror"
)

const (
	defaultModel          = "claude-sonnet-4-6"
	maxExamples           = 5
	promptTemplateVersion = "v1"
)

// SyntheticService implements ports.SyntheticService.
type SyntheticService struct {
	worldRepo ports.SyntheticWorldRepository
	callRepo  ports.SyntheticCallRepository
	state     ports.SyntheticStateAdapter
	llm       ports.LLMClient
	examples  ports.ExampleProvider // nullable — degrades to schema-only if nil
	log       *zap.Logger
	worldTTL  time.Duration
}

// NewSyntheticService creates a new SyntheticService.
func NewSyntheticService(
	worldRepo ports.SyntheticWorldRepository,
	callRepo ports.SyntheticCallRepository,
	state ports.SyntheticStateAdapter,
	llm ports.LLMClient,
	examples ports.ExampleProvider,
	log *zap.Logger,
	worldTTLSeconds int,
) *SyntheticService {
	if worldTTLSeconds <= 0 {
		worldTTLSeconds = 3600
	}
	return &SyntheticService{
		worldRepo: worldRepo,
		callRepo:  callRepo,
		state:     state,
		llm:       llm,
		examples:  examples,
		log:       log,
		worldTTL:  time.Duration(worldTTLSeconds) * time.Second,
	}
}

// CreateWorld creates a new synthetic world.
func (s *SyntheticService) CreateWorld(ctx context.Context, auth *domain.AuthContext, params domain.CreateWorldParams) (*domain.WorldResponse, error) {
	worldID := uuid.New().String()
	now := time.Now().UTC()

	seedStr := ""
	if params.Seed != nil {
		seedStr = fmt.Sprintf("%d", *params.Seed)
	}
	modelStr := ""
	if params.Model != nil {
		modelStr = *params.Model
	}
	fpStr := ""
	if params.FailureProfile != nil {
		fpData, _ := json.Marshal(params.FailureProfile)
		fpStr = string(fpData)
	}

	// Bootstrap world context from imported traces
	worldContext := s.buildWorldContext(ctx, auth.ProjectID, modelStr)

	worldState := &domain.WorldState{
		OrganizationID: auth.OrganizationID,
		ProjectID:      auth.ProjectID,
		APIKeyID:       auth.APIKeyID,
		Mode:           string(params.Mode),
		Seed:           seedStr,
		Model:          modelStr,
		FailureProfile: fpStr,
		WorldContext:   worldContext,
		CreatedAt:      now.Format(time.RFC3339Nano),
		LastAccessAt:   now.Format(time.RFC3339Nano),
	}

	if err := s.state.CreateWorld(ctx, worldID, worldState, s.worldTTL); err != nil {
		return nil, apierror.Internal(fmt.Sprintf("failed to create world state: %v", err))
	}

	world := &domain.SyntheticWorld{
		ID:             worldID,
		OrganizationID: auth.OrganizationID,
		ProjectID:      auth.ProjectID,
		APIKeyID:       auth.APIKeyID,
		Mode:           params.Mode,
		Seed:           params.Seed,
		Model:          params.Model,
		FailureProfile: params.FailureProfile,
		Status:         "active",
		TotalCalls:     0,
	}
	if err := s.worldRepo.Create(ctx, world); err != nil {
		s.log.Error("failed to persist world to postgres", zap.Error(err))
	}

	return &domain.WorldResponse{
		WorldID:   worldID,
		ProjectID: auth.ProjectID,
		Mode:      params.Mode,
		Seed:      params.Seed,
		ExpiresAt: now.Add(s.worldTTL),
	}, nil
}

// ExecuteCall executes a synthetic tool call.
func (s *SyntheticService) ExecuteCall(ctx context.Context, auth *domain.AuthContext, req domain.CallRequest) (*domain.CallResult, error) {
	start := time.Now()

	if err := checkToolAllowed(req.ToolName); err != nil {
		return nil, err
	}

	world, err := s.state.GetWorld(ctx, req.WorldID)
	if err != nil {
		return nil, apierror.Internal(fmt.Sprintf("failed to get world: %v", err))
	}
	if world == nil {
		return nil, apierror.NotFound("World")
	}

	if world.OrganizationID != auth.OrganizationID {
		return nil, apierror.Forbidden("World belongs to a different organization")
	}

	if err := s.state.RefreshTTL(ctx, req.WorldID, s.worldTTL); err != nil {
		s.log.Warn("failed to refresh world TTL", zap.Error(err))
	}

	// Check idempotency cache
	if req.IdempotencyKey != nil && *req.IdempotencyKey != "" {
		cached, err := s.state.GetCachedResponse(ctx, req.WorldID, *req.IdempotencyKey)
		if err != nil {
			s.log.Warn("cache lookup failed", zap.Error(err))
		}
		if cached != "" {
			var cachedOutput map[string]any
			if json.Unmarshal([]byte(cached), &cachedOutput) == nil {
				modelUsed := ptrStringOrNil(world.Model)
				return &domain.CallResult{
					Output:    cachedOutput,
					StepCount: 0,
					CacheHit:  true,
					ModeUsed:  world.Mode,
					ModelUsed: modelUsed,
				}, nil
			}
		}
	}

	stepCount, err := s.state.IncrStepCount(ctx, req.WorldID)
	if err != nil {
		return nil, apierror.Internal(fmt.Sprintf("failed to increment step count: %v", err))
	}

	model := world.Model
	if model == "" {
		model = defaultModel
	}

	var seed *int
	if world.Seed != "" {
		var seedVal int
		if _, err := fmt.Sscanf(world.Seed, "%d", &seedVal); err == nil {
			seed = &seedVal
		}
	}

	if world.FailureProfile != "" {
		var fp domain.FailureProfile
		if err := json.Unmarshal([]byte(world.FailureProfile), &fp); err == nil {
			if apiErr := maybeInjectError(&fp, seed, int(stepCount)); apiErr != nil {
				return nil, apiErr
			}
		}
	}

	var taskState *domain.TaskState
	isStateful := world.Mode == string(domain.SyntheticModeStateful)
	if isStateful {
		taskState, err = s.getTaskState(ctx, req.WorldID)
		if err != nil {
			s.log.Warn("failed to get task state", zap.Error(err))
		}
	}

	if validationResult := s.ValidateArguments(ctx, req.ToolName, req.ToolSchema, req.Input, taskState); validationResult != nil {
		var messages []string
		for _, e := range validationResult.Errors {
			messages = append(messages, e.Message)
		}
		errOutput := map[string]any{
			"error":   "bad_request",
			"message": strings.Join(messages, "; "),
			"status":  400,
		}
		duration := time.Since(start)
		durationMs := uint32(duration.Milliseconds())
		go s.persistCall(req, auth, world, errOutput, int(stepCount), false, nil, &durationMs, nil, validationResult, nil)

		return &domain.CallResult{
			Output:     errOutput,
			StepCount:  int(stepCount),
			CacheHit:   false,
			ModeUsed:   world.Mode,
			Validation: validationResult,
		}, nil
	}

	// Fetch examples for "examples" and "stateful" modes
	var examples []map[string]any
	if world.Mode == string(domain.SyntheticModeExamples) || isStateful {
		examples = s.fetchToolExamples(ctx, world.ProjectID, req.ToolName)
	}

	prompt := buildPrompt(req.ToolName, req.ToolSchema, req.Input, world.WorldContext, taskState, examples)
	returnSchema, _ := req.ToolSchema["returns"].(map[string]any)
	temperature := 0.2
	if seed != nil {
		temperature = 0.0
	}

	// Start feedback generation concurrently
	type feedbackResult struct{ fb *domain.TaskFeedback }
	feedbackCh := make(chan feedbackResult, 1)
	if isStateful && taskState != nil {
		go func() {
			feedbackCh <- feedbackResult{fb: s.generateTaskFeedback(ctx, taskState, world.WorldContext)}
		}()
	} else {
		feedbackCh <- feedbackResult{}
	}

	output, err := s.llm.GenerateStructuredOutput(ctx, model, systemPrompt, prompt, returnSchema, temperature)
	if err != nil {
		return nil, apierror.New("LLM_ERROR", fmt.Sprintf("LLM generation failed: %v", err), 502)
	}

	if isStateful && taskState != nil {
		s.updateTaskStateAsync(req.WorldID, req.ToolName, req.Input, output, int(stepCount), taskState)
	}

	fbResult := <-feedbackCh
	feedback := fbResult.fb

	// Cache response
	if req.IdempotencyKey != nil && *req.IdempotencyKey != "" {
		outputJSON, _ := json.Marshal(output)
		if err := s.state.SetCachedResponse(ctx, req.WorldID, *req.IdempotencyKey, string(outputJSON), s.worldTTL); err != nil {
			s.log.Warn("failed to cache response", zap.Error(err))
		}
	}

	duration := time.Since(start)
	durationMs := uint32(duration.Milliseconds())
	modelUsed := &model
	go s.persistCall(req, auth, world, output, int(stepCount), false, modelUsed, &durationMs, nil, nil, feedback)

	s.log.Info("synthetic_call",
		zap.String("world_id", req.WorldID),
		zap.String("tool_name", req.ToolName),
		zap.String("mode", world.Mode),
		zap.Int64("step_count", stepCount),
	)

	return &domain.CallResult{
		Output:    output,
		StepCount: int(stepCount),
		CacheHit:  false,
		ModeUsed:  world.Mode,
		ModelUsed: modelUsed,
		Feedback:  feedback,
	}, nil
}

// ResetWorld resets a synthetic world.
func (s *SyntheticService) ResetWorld(ctx context.Context, auth *domain.AuthContext, worldID string, hard bool) (*domain.ResetWorldResponse, error) {
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

	if err := s.state.DeleteWorld(ctx, worldID); err != nil {
		return nil, apierror.Internal(fmt.Sprintf("failed to reset world: %v", err))
	}

	return &domain.ResetWorldResponse{
		WorldID:   worldID,
		StepCount: 0,
		ResetAt:   time.Now().UTC(),
	}, nil
}

// CloseWorld permanently closes a synthetic world.
func (s *SyntheticService) CloseWorld(ctx context.Context, auth *domain.AuthContext, worldID string) (*domain.DeleteWorldResponse, error) {
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

	if err := s.state.DeleteWorld(ctx, worldID); err != nil {
		s.log.Warn("failed to delete world from Redis", zap.Error(err))
	}

	if err := s.worldRepo.Close(ctx, worldID); err != nil {
		s.log.Warn("failed to close world in Postgres", zap.Error(err))
	}

	return &domain.DeleteWorldResponse{Success: true}, nil
}

// ListWorlds lists synthetic worlds.
func (s *SyntheticService) ListWorlds(ctx context.Context, auth *domain.AuthContext, params domain.WorldListParams) (*domain.WorldListResult, error) {
	params.OrganizationID = auth.OrganizationID
	return s.worldRepo.List(ctx, params)
}

// ListCalls lists synthetic calls.
func (s *SyntheticService) ListCalls(ctx context.Context, auth *domain.AuthContext, params domain.CallListParams) (*domain.CallListResult, error) {
	params.OrganizationID = auth.OrganizationID
	return s.callRepo.List(ctx, params)
}

// GetWorld returns detailed information about a single world including task state.
func (s *SyntheticService) GetWorld(ctx context.Context, auth *domain.AuthContext, worldID string) (*domain.WorldDetailResponse, error) {
	world, err := s.worldRepo.GetByID(ctx, worldID)
	if err != nil {
		return nil, err
	}
	if world == nil {
		return nil, apierror.NotFound("World")
	}
	if world.OrganizationID != auth.OrganizationID {
		return nil, apierror.Forbidden("World belongs to a different organization")
	}

	var taskState *domain.TaskState
	if world.Status == "active" {
		taskState, err = s.getTaskState(ctx, worldID)
		if err != nil {
			s.log.Warn("failed to get task state for world detail", zap.Error(err))
		}
	}

	stats, err := s.callRepo.GetWorldStats(ctx, worldID)
	if err != nil {
		s.log.Warn("failed to get world stats", zap.Error(err))
	}

	return &domain.WorldDetailResponse{
		World:     *world,
		TaskState: taskState,
		Stats:     stats,
	}, nil
}

// GetWorldState returns the current task state of a world.
func (s *SyntheticService) GetWorldState(ctx context.Context, auth *domain.AuthContext, worldID string) (*domain.TaskState, error) {
	worldState, err := s.state.GetWorld(ctx, worldID)
	if err != nil {
		return nil, apierror.Internal(fmt.Sprintf("failed to get world: %v", err))
	}
	if worldState == nil {
		return nil, apierror.NotFound("World")
	}
	if worldState.OrganizationID != auth.OrganizationID {
		return nil, apierror.Forbidden("World belongs to a different organization")
	}

	return s.getTaskState(ctx, worldID)
}

// persistCall saves a call record in the background.
func (s *SyntheticService) persistCall(
	req domain.CallRequest,
	auth *domain.AuthContext,
	world *domain.WorldState,
	output map[string]any,
	stepCount int,
	cacheHit bool,
	modelUsed *string,
	durationMs *uint32,
	callError *string,
	validation *domain.ValidationResult,
	feedback *domain.TaskFeedback,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	toolSchemaJSON, _ := json.Marshal(req.ToolSchema)
	inputJSON, _ := json.Marshal(req.Input)
	outputJSON, _ := json.Marshal(output)

	var validationStr *string
	if validation != nil {
		data, _ := json.Marshal(validation)
		s := string(data)
		validationStr = &s
	}
	var feedbackStr *string
	if feedback != nil {
		data, _ := json.Marshal(feedback)
		s := string(data)
		feedbackStr = &s
	}

	call := &domain.SyntheticCall{
		ID:             uuid.New().String(),
		WorldID:        req.WorldID,
		OrganizationID: auth.OrganizationID,
		ProjectID:      world.ProjectID,
		ToolName:       req.ToolName,
		ToolSchema:     string(toolSchemaJSON),
		InputData:      string(inputJSON),
		OutputData:     string(outputJSON),
		StepCount:      uint32(stepCount),
		ModeUsed:       world.Mode,
		CacheHit:       cacheHit,
		ModelUsed:      modelUsed,
		IdempotencyKey: req.IdempotencyKey,
		DurationMs:     durationMs,
		Error:          callError,
		ValidationData: validationStr,
		FeedbackData:   feedbackStr,
		CreatedAt:      time.Now().UTC(),
	}

	if err := s.callRepo.Insert(ctx, call); err != nil {
		s.log.Error("failed to persist synthetic call", zap.Error(err))
	}

	if err := s.worldRepo.IncrementCalls(ctx, req.WorldID); err != nil {
		s.log.Error("failed to increment world calls", zap.Error(err))
	}
}

// buildWorldContext uses the ExampleProvider to create a product context for the world.
func (s *SyntheticService) buildWorldContext(ctx context.Context, projectID, model string) string {
	if s.examples == nil {
		return ""
	}

	worldCtx, err := s.examples.GetWorldContext(ctx, projectID)
	if err != nil {
		s.log.Warn("failed to get world context from example provider", zap.Error(err))
		return ""
	}

	return worldCtx
}

// fetchToolExamples fetches recent tool span examples from the example provider.
func (s *SyntheticService) fetchToolExamples(ctx context.Context, projectID, toolName string) []map[string]any {
	if s.examples == nil {
		return nil
	}

	toolExamples, err := s.examples.GetToolExamples(ctx, projectID, toolName, maxExamples)
	if err != nil {
		s.log.Warn("failed to fetch tool examples", zap.Error(err))
		return nil
	}

	var examples []map[string]any
	for _, ex := range toolExamples {
		examples = append(examples, map[string]any{
			"input":  sanitizeObj(ex.Input),
			"output": sanitizeObj(ex.Output),
		})
	}

	return examples
}

func checkToolAllowed(toolName string) *apierror.APIError {
	raw := os.Getenv("SYNTHETIC_ALLOWED_TOOLS")
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	for _, p := range parts {
		if strings.TrimSpace(p) == toolName {
			return nil
		}
	}

	return apierror.Forbidden(fmt.Sprintf("Tool not allowed: %s", toolName))
}

func mustJSON(v any) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}

func ptrStringOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
