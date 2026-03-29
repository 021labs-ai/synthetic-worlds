package services

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/021labs-ai/synthetic-worlds/internal/domain"
)

const (
	validationModel = "claude-haiku-4-5"
)

// ValidateArguments runs syntactic validation first, then semantic if syntactic passes
// and state is present. Returns nil if no validation issues found.
func (s *SyntheticService) ValidateArguments(
	ctx context.Context,
	toolName string,
	toolSchema map[string]any,
	input map[string]any,
	taskState *domain.TaskState,
) *domain.ValidationResult {
	synErrs := validateSyntactic(toolSchema, input)
	if len(synErrs) > 0 {
		return &domain.ValidationResult{
			Valid:  false,
			Errors: synErrs,
		}
	}

	if taskState != nil && (len(taskState.Entities) > 0 || len(taskState.RuntimeVars) > 0) {
		semErrs := s.validateSemantic(ctx, toolName, toolSchema, input, taskState)
		if len(semErrs) > 0 {
			return &domain.ValidationResult{
				Valid:  false,
				Errors: semErrs,
			}
		}
	}

	return nil
}

func validateSyntactic(toolSchema map[string]any, input map[string]any) []domain.ValidationError {
	params, ok := toolSchema["parameters"].(map[string]any)
	if !ok {
		return nil
	}

	var errors []domain.ValidationError

	if required, ok := params["required"].([]any); ok {
		for _, r := range required {
			fieldName, ok := r.(string)
			if !ok {
				continue
			}
			if _, exists := input[fieldName]; !exists {
				errors = append(errors, domain.ValidationError{
					Type:    domain.ValidationErrorMissingRequired,
					Field:   fieldName,
					Message: fmt.Sprintf("Required parameter '%s' is missing", fieldName),
				})
			}
		}
	}

	properties, ok := params["properties"].(map[string]any)
	if !ok {
		return errors
	}

	for fieldName, value := range input {
		propDef, exists := properties[fieldName]
		if !exists {
			continue
		}

		prop, ok := propDef.(map[string]any)
		if !ok {
			continue
		}

		if expectedType, ok := prop["type"].(string); ok {
			if typeErr := checkType(fieldName, value, expectedType); typeErr != nil {
				errors = append(errors, *typeErr)
				continue
			}
		}

		if enumVals, ok := prop["enum"].([]any); ok {
			if !isInEnum(value, enumVals) {
				errors = append(errors, domain.ValidationError{
					Type:    domain.ValidationErrorEnumInvalid,
					Field:   fieldName,
					Message: fmt.Sprintf("Value '%v' is not a valid enum value for '%s'", value, fieldName),
				})
			}
		}

		if num, ok := toFloat64(value); ok {
			if min, ok := toFloat64(prop["minimum"]); ok && num < min {
				errors = append(errors, domain.ValidationError{
					Type:    domain.ValidationErrorRangeExceeded,
					Field:   fieldName,
					Message: fmt.Sprintf("Value %v is below minimum %v for '%s'", value, prop["minimum"], fieldName),
				})
			}
			if max, ok := toFloat64(prop["maximum"]); ok && num > max {
				errors = append(errors, domain.ValidationError{
					Type:    domain.ValidationErrorRangeExceeded,
					Field:   fieldName,
					Message: fmt.Sprintf("Value %v exceeds maximum %v for '%s'", value, prop["maximum"], fieldName),
				})
			}
		}
	}

	return errors
}

func (s *SyntheticService) validateSemantic(
	ctx context.Context,
	toolName string,
	toolSchema map[string]any,
	input map[string]any,
	taskState *domain.TaskState,
) []domain.ValidationError {
	stateJSON, _ := json.Marshal(taskState)
	schemaJSON, _ := json.MarshalIndent(toolSchema, "", "  ")
	inputJSON, _ := json.MarshalIndent(input, "", "  ")

	prompt := fmt.Sprintf(`Tool: %s
Schema:
%s

Current world state:
%s

Arguments to validate:
%s

Are these arguments logically valid given the tool specification and current world state?
Return {"errors": []} if valid, or {"errors": [{"type": "semantic", "field": "...", "message": "..."}]} if not.`,
		toolName, string(schemaJSON), string(stateJSON), string(inputJSON))

	result, err := s.llm.GenerateStructuredOutput(ctx, validationModel, semanticValidatorSystemPrompt, prompt, nil, 0.0)
	if err != nil {
		s.log.Warn("semantic validation LLM call failed, skipping", zap.Error(err))
		return nil
	}

	errorsRaw, ok := result["errors"].([]any)
	if !ok || len(errorsRaw) == 0 {
		return nil
	}

	var valErrors []domain.ValidationError
	for _, e := range errorsRaw {
		errMap, ok := e.(map[string]any)
		if !ok {
			continue
		}
		valErrors = append(valErrors, domain.ValidationError{
			Type:    domain.ValidationErrorType(stringFromMap(errMap, "type", string(domain.ValidationErrorSemantic))),
			Field:   stringFromMap(errMap, "field", "unknown"),
			Message: stringFromMap(errMap, "message", "semantic validation failed"),
		})
	}

	return valErrors
}

func checkType(field string, value any, expectedType string) *domain.ValidationError {
	valid := false
	switch expectedType {
	case "string":
		_, valid = value.(string)
	case "integer":
		valid = isInteger(value)
	case "number":
		_, valid = toFloat64(value)
	case "boolean":
		_, valid = value.(bool)
	case "array":
		_, valid = value.([]any)
	case "object":
		_, valid = value.(map[string]any)
	default:
		return nil
	}

	if !valid {
		return &domain.ValidationError{
			Type:    domain.ValidationErrorTypeMismatch,
			Field:   field,
			Message: fmt.Sprintf("Expected type '%s' for '%s', got %T", expectedType, field, value),
		}
	}
	return nil
}

func isInteger(v any) bool {
	switch n := v.(type) {
	case float64:
		return n == float64(int64(n))
	case int:
		return true
	case int64:
		return true
	case json.Number:
		_, err := n.Int64()
		return err == nil
	}
	return false
}

func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	}
	return 0, false
}

func isInEnum(value any, enumVals []any) bool {
	for _, e := range enumVals {
		if fmt.Sprintf("%v", value) == fmt.Sprintf("%v", e) {
			return true
		}
	}
	return false
}

func stringFromMap(m map[string]any, key, defaultVal string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return defaultVal
}
