package apierror

import (
	"fmt"
	"net/http"
)

// Error codes
const (
	CodeValidation    = "VALIDATION_ERROR"
	CodeMissingAuth   = "MISSING_AUTH"
	CodeInvalidKey    = "INVALID_KEY"
	CodeInvalidFormat = "INVALID_KEY_FORMAT"
	CodeKeyRevoked    = "KEY_REVOKED"
	CodeKeyExpired    = "KEY_EXPIRED"
	CodeForbidden     = "FORBIDDEN"
	CodeNotFound      = "NOT_FOUND"
	CodeRateLimited   = "RATE_LIMITED"
	CodeInternal      = "INTERNAL_ERROR"
)

// APIError represents a structured API error.
type APIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
	Details    any    `json:"details,omitempty"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// ErrorResponse represents the JSON error response structure.
type ErrorResponse struct {
	Success bool      `json:"success"`
	Error   *APIError `json:"error"`
}

// New creates a new APIError.
func New(code, message string, statusCode int) *APIError {
	return &APIError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// WithDetails adds details to the error.
func (e *APIError) WithDetails(details any) *APIError {
	e.Details = details
	return e
}

// Validation creates a validation error.
func Validation(message string, details any) *APIError {
	return &APIError{
		Code:       CodeValidation,
		Message:    message,
		StatusCode: http.StatusBadRequest,
		Details:    details,
	}
}

// MissingAuth creates a missing authentication error.
func MissingAuth() *APIError {
	return &APIError{
		Code:       CodeMissingAuth,
		Message:    "Authorization header required",
		StatusCode: http.StatusUnauthorized,
	}
}

// InvalidKeyFormat creates an invalid key format error.
func InvalidKeyFormat() *APIError {
	return &APIError{
		Code:       CodeInvalidFormat,
		Message:    "Invalid API key format",
		StatusCode: http.StatusUnauthorized,
	}
}

// InvalidKey creates an invalid key error.
func InvalidKey() *APIError {
	return &APIError{
		Code:       CodeInvalidKey,
		Message:    "Invalid API key",
		StatusCode: http.StatusUnauthorized,
	}
}

// KeyRevoked creates a key revoked error.
func KeyRevoked() *APIError {
	return &APIError{
		Code:       CodeKeyRevoked,
		Message:    "API key has been revoked",
		StatusCode: http.StatusUnauthorized,
	}
}

// KeyExpired creates a key expired error.
func KeyExpired() *APIError {
	return &APIError{
		Code:       CodeKeyExpired,
		Message:    "API key has expired",
		StatusCode: http.StatusUnauthorized,
	}
}

// Forbidden creates a forbidden error.
func Forbidden(message string) *APIError {
	if message == "" {
		message = "Insufficient permissions"
	}
	return &APIError{
		Code:       CodeForbidden,
		Message:    message,
		StatusCode: http.StatusForbidden,
	}
}

// NotFound creates a not found error.
func NotFound(resource string) *APIError {
	return &APIError{
		Code:       CodeNotFound,
		Message:    resource + " not found",
		StatusCode: http.StatusNotFound,
	}
}

// RateLimited creates a rate limit error.
func RateLimited(retryAfter int64) *APIError {
	return &APIError{
		Code:       CodeRateLimited,
		Message:    "Rate limit exceeded",
		StatusCode: http.StatusTooManyRequests,
		Details:    map[string]int64{"retryAfter": retryAfter},
	}
}

// Internal creates an internal server error.
func Internal(message string) *APIError {
	if message == "" {
		message = "An internal error occurred"
	}
	return &APIError{
		Code:       CodeInternal,
		Message:    message,
		StatusCode: http.StatusInternalServerError,
	}
}

// ToResponse converts an APIError to an ErrorResponse.
func (e *APIError) ToResponse() *ErrorResponse {
	return &ErrorResponse{
		Success: false,
		Error:   e,
	}
}
