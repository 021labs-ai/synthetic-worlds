package domain

import "time"

// SyntheticMode defines how synthetic responses are generated.
type SyntheticMode string

const (
	SyntheticModeSchemaOnly SyntheticMode = "schema_only"
	SyntheticModeExamples   SyntheticMode = "examples"
	SyntheticModeStateful   SyntheticMode = "stateful"
)

// SyntheticErrorCode represents a simulated error type.
type SyntheticErrorCode string

const (
	SyntheticErrorTimeout    SyntheticErrorCode = "timeout"
	SyntheticErrorInternal   SyntheticErrorCode = "internal_error"
	SyntheticErrorRateLimit  SyntheticErrorCode = "rate_limit"
	SyntheticErrorNotFound   SyntheticErrorCode = "not_found"
	SyntheticErrorBadRequest SyntheticErrorCode = "bad_request"
)

// ErrorCodeToHTTPStatus maps synthetic error codes to HTTP status codes.
var ErrorCodeToHTTPStatus = map[SyntheticErrorCode]int{
	SyntheticErrorTimeout:    504,
	SyntheticErrorInternal:   500,
	SyntheticErrorRateLimit:  429,
	SyntheticErrorNotFound:   404,
	SyntheticErrorBadRequest: 400,
}

// FailureProfile configures deterministic error injection.
type FailureProfile struct {
	Rate  float64              `json:"rate"`
	Codes []SyntheticErrorCode `json:"codes"`
}

// SyntheticWorld represents a synthetic world stored in Postgres.
type SyntheticWorld struct {
	ID             string          `json:"id"`
	OrganizationID string          `json:"organizationId"`
	ProjectID      string          `json:"projectId"`
	APIKeyID       string          `json:"apiKeyId"`
	Mode           SyntheticMode   `json:"mode"`
	Seed           *int            `json:"seed,omitempty"`
	Model          *string         `json:"model,omitempty"`
	FailureProfile *FailureProfile `json:"failureProfile,omitempty"`
	Status         string          `json:"status"`
	TotalCalls     int             `json:"totalCalls"`
	CreatedAt      time.Time       `json:"createdAt"`
	ClosedAt       *time.Time      `json:"closedAt,omitempty"`
}

// CreateWorldParams represents parameters for creating a synthetic world.
type CreateWorldParams struct {
	ProjectID      string          `json:"project_id" validate:"required"`
	Mode           SyntheticMode   `json:"mode" validate:"required,oneof=schema_only examples stateful"`
	Seed           *int            `json:"seed,omitempty"`
	Model          *string         `json:"model,omitempty"`
	FailureProfile *FailureProfile `json:"failure_profile,omitempty"`
}

// WorldResponse is the API response after creating a world.
type WorldResponse struct {
	WorldID   string        `json:"world_id"`
	ProjectID string        `json:"project_id"`
	Mode      SyntheticMode `json:"mode"`
	Seed      *int          `json:"seed,omitempty"`
	ExpiresAt time.Time     `json:"expires_at"`
}

// SyntheticCall represents a synthetic call record.
type SyntheticCall struct {
	ID             string    `json:"id"`
	WorldID        string    `json:"worldId"`
	OrganizationID string    `json:"organizationId"`
	ProjectID      string    `json:"projectId"`
	ToolName       string    `json:"toolName"`
	ToolSchema     string    `json:"toolSchema,omitempty"`
	InputData      string    `json:"inputData,omitempty"`
	OutputData     string    `json:"outputData,omitempty"`
	StepCount      uint32    `json:"stepCount"`
	ModeUsed       string    `json:"modeUsed"`
	CacheHit       bool      `json:"cacheHit"`
	ModelUsed      *string   `json:"modelUsed,omitempty"`
	IdempotencyKey *string   `json:"idempotencyKey,omitempty"`
	DurationMs     *uint32   `json:"durationMs,omitempty"`
	Error          *string   `json:"error,omitempty"`
	ValidationData *string   `json:"validationData,omitempty"`
	FeedbackData   *string   `json:"feedbackData,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

// CallRequest represents a synthetic tool call request.
type CallRequest struct {
	WorldID        string         `json:"world_id" validate:"required"`
	ToolName       string         `json:"tool_name" validate:"required"`
	ToolSchema     map[string]any `json:"tool_schema" validate:"required"`
	Input          map[string]any `json:"input" validate:"required"`
	IdempotencyKey *string        `json:"idempotency_key,omitempty"`
}

// CallResult represents the result of a synthetic tool call.
type CallResult struct {
	Output     map[string]any    `json:"output"`
	StepCount  int               `json:"step_count"`
	CacheHit   bool              `json:"cache_hit"`
	ModeUsed   string            `json:"mode_used"`
	ModelUsed  *string           `json:"model_used,omitempty"`
	Validation *ValidationResult `json:"validation,omitempty"`
	Feedback   *TaskFeedback     `json:"feedback,omitempty"`
}

// WorldState represents ephemeral world state in Redis.
type WorldState struct {
	OrganizationID string `json:"org_id"`
	ProjectID      string `json:"project_id"`
	APIKeyID       string `json:"api_key_id"`
	Mode           string `json:"mode"`
	Seed           string `json:"seed"`
	Model          string `json:"model"`
	FailureProfile string `json:"failure_profile"`
	WorldContext   string `json:"world_context"`
	TaskState      string `json:"task_state"`
	StepCount      string `json:"step_count"`
	CreatedAt      string `json:"created_at"`
	LastAccessAt   string `json:"last_access_at"`
}

// TaskState holds explicit structured state that accumulates across calls.
type TaskState struct {
	Entities    map[string][]map[string]any `json:"entities"`
	RuntimeVars map[string]any              `json:"runtime_vars"`
	History     []StateHistoryEntry         `json:"history"`
}

// NewTaskState returns an initialized empty TaskState.
func NewTaskState() *TaskState {
	return &TaskState{
		Entities:    make(map[string][]map[string]any),
		RuntimeVars: make(map[string]any),
	}
}

// StatePatch represents a delta produced by the state updater LLM.
type StatePatch struct {
	UpsertEntities map[string][]map[string]any `json:"upsert_entities,omitempty"`
	DeleteEntities map[string][]string         `json:"delete_entities,omitempty"`
	SetVars        map[string]any              `json:"set_vars,omitempty"`
	DeleteVars     []string                    `json:"delete_vars,omitempty"`
	HistoryEntry   string                      `json:"history_entry"`
}

// StateHistoryEntry records a concise action log entry.
type StateHistoryEntry struct {
	Step     int    `json:"step"`
	ToolName string `json:"tool_name"`
	Action   string `json:"action"`
}

// ValidationErrorType categorizes validation failures.
type ValidationErrorType string

const (
	ValidationErrorMissingRequired ValidationErrorType = "missing_required"
	ValidationErrorTypeMismatch    ValidationErrorType = "type_mismatch"
	ValidationErrorEnumInvalid     ValidationErrorType = "enum_invalid"
	ValidationErrorRangeExceeded   ValidationErrorType = "range_exceeded"
	ValidationErrorSemantic        ValidationErrorType = "semantic"
)

// ValidationResult represents the outcome of argument validation.
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// ValidationError describes a single validation failure.
type ValidationError struct {
	Type    ValidationErrorType `json:"type"`
	Field   string              `json:"field"`
	Message string              `json:"message"`
}

// TaskFeedback assesses workflow progress based on accumulated state.
type TaskFeedback struct {
	TaskComplete    bool     `json:"task_complete"`
	CompletedItems  []string `json:"completed_items,omitempty"`
	RemainingItems  []string `json:"remaining_items,omitempty"`
	ProgressPercent int      `json:"progress_percent"`
}

// ResetWorldRequest represents a world reset request.
type ResetWorldRequest struct {
	Hard bool `json:"hard"`
}

// ResetWorldResponse represents the result of a world reset.
type ResetWorldResponse struct {
	WorldID   string    `json:"world_id"`
	StepCount int       `json:"step_count"`
	ResetAt   time.Time `json:"reset_at"`
}

// DeleteWorldResponse represents the result of deleting a world.
type DeleteWorldResponse struct {
	Success bool `json:"success"`
}

// WorldListParams represents query parameters for listing worlds.
type WorldListParams struct {
	OrganizationID string
	ProjectID      string
	Status         string
	Limit          int
	Offset         int
}

// WorldListResult represents the result of listing worlds.
type WorldListResult struct {
	Worlds  []SyntheticWorld
	Total   int64
	Limit   int
	Offset  int
	HasMore bool
}

// CallListParams represents query parameters for listing calls.
type CallListParams struct {
	OrganizationID string
	ProjectID      string
	WorldID        string
	ToolName       string
	Limit          int
	Offset         int
}

// CallListResult represents the result of listing calls.
type CallListResult struct {
	Calls   []SyntheticCall
	Total   int64
	Limit   int
	Offset  int
	HasMore bool
}

// WorldDetailResponse is the full detail response for a single world.
type WorldDetailResponse struct {
	World     SyntheticWorld `json:"world"`
	TaskState *TaskState     `json:"task_state,omitempty"`
	Stats     *WorldStats    `json:"stats,omitempty"`
}

// WorldStats aggregates call statistics for a world.
type WorldStats struct {
	TotalCalls    int            `json:"total_calls"`
	ToolCounts    map[string]int `json:"tool_counts"`
	AvgDurationMs float64        `json:"avg_duration_ms"`
	CacheHitRate  float64        `json:"cache_hit_rate"`
	ErrorCount    int            `json:"error_count"`
}
