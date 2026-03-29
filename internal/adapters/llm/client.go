package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	DefaultModel     = "claude-sonnet-4-6"
	MaxGenTokens     = 2048
	anthropicBaseURL = "https://api.anthropic.com/v1"
	openAIBaseURL    = "https://api.openai.com/v1"
	xaiBaseURL       = "https://api.x.ai/v1"
	anthropicVersion = "2023-06-01"
)

// Provider represents an LLM provider.
type Provider string

const (
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
	ProviderXAI       Provider = "xai"
)

// Client makes calls to LLM APIs for structured output generation.
type Client struct {
	anthropicKey string
	openAIKey    string
	xaiKey       string
	httpClient   *http.Client
}

// NewClient creates a new LLM client with BYOK (Bring Your Own Key).
func NewClient(anthropicKey, openAIKey, xaiKey string) *Client {
	return &Client{
		anthropicKey: anthropicKey,
		openAIKey:    openAIKey,
		xaiKey:       xaiKey,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// DetectProvider determines the LLM provider from the model name.
func DetectProvider(model string) Provider {
	lower := strings.ToLower(model)
	switch {
	case strings.HasPrefix(lower, "gpt-") || strings.HasPrefix(lower, "o1-") || strings.HasPrefix(lower, "o3-") || strings.HasPrefix(lower, "o4-"):
		return ProviderOpenAI
	case strings.HasPrefix(lower, "claude-") || strings.HasPrefix(lower, "anthropic."):
		return ProviderAnthropic
	case strings.HasPrefix(lower, "grok-"):
		return ProviderXAI
	default:
		return ProviderAnthropic
	}
}

// GenerateStructuredOutput generates a structured JSON response from an LLM.
func (c *Client) GenerateStructuredOutput(ctx context.Context, model, systemPrompt, userPrompt string, returnSchema map[string]any, temperature float64) (map[string]any, error) {
	provider := DetectProvider(model)

	switch provider {
	case ProviderOpenAI:
		return c.callOpenAI(ctx, openAIBaseURL, c.openAIKey, model, systemPrompt, userPrompt, returnSchema, temperature)
	case ProviderXAI:
		return c.callOpenAI(ctx, xaiBaseURL, c.xaiKey, model, systemPrompt, userPrompt, returnSchema, temperature)
	case ProviderAnthropic:
		return c.callAnthropic(ctx, model, systemPrompt, userPrompt, returnSchema, temperature)
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", provider)
	}
}

// callAnthropic calls the Anthropic Messages API directly with BYOK.
func (c *Client) callAnthropic(ctx context.Context, model, systemPrompt, userPrompt string, returnSchema map[string]any, temperature float64) (map[string]any, error) {
	if c.anthropicKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not configured")
	}

	reqBody := map[string]any{
		"model":      model,
		"system":     systemPrompt,
		"max_tokens": MaxGenTokens,
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
		"temperature": temperature,
	}

	// Use tool_use trick for structured output if schema has properties
	if returnSchema != nil {
		if props, _ := returnSchema["properties"].(map[string]any); len(props) > 0 {
			toolSchema := map[string]any{
				"type":       "object",
				"properties": props,
			}
			if req, ok := returnSchema["required"]; ok {
				toolSchema["required"] = req
			}

			reqBody["tools"] = []map[string]any{
				{
					"name":         "respond",
					"description":  "Return the structured JSON response",
					"input_schema": toolSchema,
				},
			}
			reqBody["tool_choice"] = map[string]any{
				"type": "tool",
				"name": "respond",
			}
		}
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal Anthropic request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicBaseURL+"/messages", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create Anthropic request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.anthropicKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Anthropic request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Anthropic response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("Anthropic API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Content []struct {
			Type  string         `json:"type"`
			Text  string         `json:"text"`
			Input map[string]any `json:"input"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode Anthropic response: %w", err)
	}

	// Extract from tool_use block if present
	for _, block := range result.Content {
		if block.Type == "tool_use" && block.Input != nil {
			return block.Input, nil
		}
	}

	// Fallback: extract from text blocks
	var text string
	for _, block := range result.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}
	if text == "" {
		text = "{}"
	}
	text = stripMarkdownFences(text)

	var jsonOutput map[string]any
	if err := json.Unmarshal([]byte(text), &jsonOutput); err != nil {
		return nil, fmt.Errorf("failed to parse Anthropic JSON output: %w", err)
	}

	return jsonOutput, nil
}

// callOpenAI calls the OpenAI-compatible API with structured output.
func (c *Client) callOpenAI(ctx context.Context, baseURL, apiKey, model, systemPrompt, userPrompt string, returnSchema map[string]any, temperature float64) (map[string]any, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key not configured for model %s", model)
	}

	reqBody := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature":           temperature,
		"max_completion_tokens": MaxGenTokens,
	}

	strictSchema := ensureStrictSchema(returnSchema)
	if strictSchema != nil && len(strictSchema) > 0 {
		if props, ok := strictSchema["properties"]; ok && props != nil {
			reqBody["response_format"] = map[string]any{
				"type": "json_schema",
				"json_schema": map[string]any{
					"name":   "tool_response",
					"strict": true,
					"schema": strictSchema,
				},
			}
		} else {
			reqBody["response_format"] = map[string]any{"type": "json_object"}
		}
	} else {
		reqBody["response_format"] = map[string]any{"type": "json_object"}
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenAI request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenAI request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("OpenAI request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read OpenAI response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("OpenAI API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("failed to decode OpenAI response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("OpenAI returned no choices")
	}

	content := result.Choices[0].Message.Content
	if content == "" {
		content = "{}"
	}

	var jsonOutput map[string]any
	if err := json.Unmarshal([]byte(content), &jsonOutput); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAI JSON output: %w", err)
	}

	return jsonOutput, nil
}

// ensureStrictSchema makes a JSON Schema OpenAI-strict-compatible.
func ensureStrictSchema(schema map[string]any) map[string]any {
	if schema == nil {
		return nil
	}

	schemaType, _ := schema["type"].(string)
	if schemaType != "object" {
		return schema
	}

	result := make(map[string]any, len(schema))
	for k, v := range schema {
		result[k] = v
	}

	if props, ok := result["properties"].(map[string]any); ok && len(props) > 0 {
		if _, hasRequired := result["required"]; !hasRequired {
			keys := make([]string, 0, len(props))
			for k := range props {
				keys = append(keys, k)
			}
			result["required"] = keys
		}
		result["additionalProperties"] = false
	}

	return result
}

// stripMarkdownFences removes ```json ... ``` wrapping from LLM output.
func stripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s, "\n"); idx != -1 {
			s = s[idx+1:]
		}
		if idx := strings.LastIndex(s, "```"); idx != -1 {
			s = s[:idx]
		}
		s = strings.TrimSpace(s)
	}
	return s
}
