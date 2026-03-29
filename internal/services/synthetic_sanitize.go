package services

import (
	"encoding/json"
	"regexp"
)

// Prompt-boundary sanitization patterns.
// Defense-in-depth: trace data injected into LLM prompts is sanitized
// even though RDK redacts on ingestion.
var sanitizePatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{
		pattern:     regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`),
		replacement: "[REDACTED_EMAIL]",
	},
	{
		pattern:     regexp.MustCompile(`(?i)\b(sk-[A-Za-z0-9]{20,}|pk_[A-Za-z0-9]{20,}|ghp_[A-Za-z0-9]{36}|eyJ[A-Za-z0-9_\-]{20,}\.[A-Za-z0-9_\-]{20,})`),
		replacement: "[REDACTED_KEY]",
	},
	{
		pattern:     regexp.MustCompile(`(?i)(password|secret|token|api_key|apikey|authorization)\s*[=:]\s*\S+`),
		replacement: "${1}=[REDACTED]",
	},
}

// sanitizeText strips secrets, tokens, and emails from text before prompt injection.
func sanitizeText(text string) string {
	for _, sp := range sanitizePatterns {
		text = sp.pattern.ReplaceAllString(text, sp.replacement)
	}
	return text
}

// sanitizeObj recursively sanitizes string values in a map or slice.
func sanitizeObj(obj any) any {
	switch v := obj.(type) {
	case string:
		return sanitizeText(v)
	case map[string]any:
		result := make(map[string]any, len(v))
		for k, val := range v {
			result[k] = sanitizeObj(val)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, val := range v {
			result[i] = sanitizeObj(val)
		}
		return result
	default:
		return obj
	}
}

// sanitizeJSON sanitizes a JSON string and returns the sanitized version.
func sanitizeJSON(raw string) string {
	var obj any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return sanitizeText(raw)
	}
	sanitized := sanitizeObj(obj)
	data, err := json.Marshal(sanitized)
	if err != nil {
		return sanitizeText(raw)
	}
	return string(data)
}
