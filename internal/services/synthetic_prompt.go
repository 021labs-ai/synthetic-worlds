package services

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/021labs-ai/synthetic-worlds/internal/domain"
)

const systemPrompt = "You are a synthetic tool simulator. " +
	"Given a tool specification, product context, and input, produce a realistic JSON response " +
	"that conforms to the tool's return schema. " +
	"Your responses must be consistent with the product context and any prior facts from this session."

const stateUpdaterSystemPrompt = "You are a state management engine for a synthetic API simulator. " +
	"Given the current task state, a tool call (name + input + output), and the step count, " +
	"produce a STATE PATCH — only the changes, not the full state. Follow these rules strictly:\n" +
	"1. Write operations (create, update, delete) MUST produce entity upserts or deletes.\n" +
	"2. Read operations that return synthesized data MUST upsert results as ground truth.\n" +
	"3. Only set runtime vars that actually changed (e.g., authenticated user, selected account).\n" +
	"4. Always include a concise history_entry describing what happened.\n" +
	"5. Only use delete_entities when the tool call explicitly deletes something.\n" +
	"6. Use plural category names as keys (e.g., \"tickets\", \"orders\", \"users\").\n" +
	"7. Each entity in upsert_entities MUST have an \"id\" field.\n" +
	"Return ONLY valid JSON matching this schema:\n" +
	"{\"upsert_entities\": {\"category\": [{\"id\": \"...\", ...}]}, " +
	"\"delete_entities\": {\"category\": [\"id1\"]}, " +
	"\"set_vars\": {\"key\": \"value\"}, " +
	"\"delete_vars\": [\"key\"], " +
	"\"history_entry\": \"concise description of what happened\"}"

const semanticValidatorSystemPrompt = "You are an argument validator for a synthetic API simulator. " +
	"Given a tool specification, the current world state, and the provided arguments, " +
	"determine if the argument values are logically valid in context. " +
	"Check for: references to non-existent entities, impossible values given current state, " +
	"logically contradictory combinations, and values that violate business rules visible in state.\n" +
	"Return a JSON object: {\"errors\": [{\"type\": \"semantic\", \"field\": \"...\", \"message\": \"...\"}]}. " +
	"Return {\"errors\": []} if all arguments are semantically valid."

const taskFeedbackSystemPrompt = "You are a workflow progress assessor for a synthetic API simulator. " +
	"Given the current task state (entities, runtime_vars, history) and a product context description, " +
	"assess how far along a typical workflow is. " +
	"Consider what operations have been completed and what typical remaining steps would be.\n" +
	"Return a JSON object: {\"task_complete\": bool, \"completed_items\": [\"...\"], " +
	"\"remaining_items\": [\"...\"], \"progress_percent\": 0-100}."

const worldContextSystemPrompt = "You are a product analyst. " +
	"Given recent trace data from a software product, produce a concise description of " +
	"what the product does, what entities exist, what the data model looks like, and " +
	"what realistic data values would be. " +
	"Focus on: domain vocabulary, entity relationships, typical field values, and business rules."

// buildPrompt constructs the user prompt for LLM generation.
func buildPrompt(
	toolName string,
	toolSchema map[string]any,
	inputData map[string]any,
	worldContext string,
	taskState *domain.TaskState,
	examples []map[string]any,
) string {
	var parts []string

	// World context comes first — grounds everything
	if worldContext != "" {
		parts = append(parts, fmt.Sprintf("Product context (generate data consistent with this):\n%s", worldContext))
	}

	// Task state — explicit structured state for cross-call consistency
	if taskState != nil && (len(taskState.Entities) > 0 || len(taskState.RuntimeVars) > 0) {
		var stateLines []string
		stateLines = append(stateLines, "Current world state (generate responses consistent with this):")
		if len(taskState.Entities) > 0 {
			entJSON, _ := json.MarshalIndent(taskState.Entities, "  ", "  ")
			stateLines = append(stateLines, fmt.Sprintf("  Entities: %s", string(entJSON)))
		}
		if len(taskState.RuntimeVars) > 0 {
			rtJSON, _ := json.MarshalIndent(taskState.RuntimeVars, "  ", "  ")
			stateLines = append(stateLines, fmt.Sprintf("  Runtime: %s", string(rtJSON)))
		}
		if len(taskState.History) > 0 {
			start := 0
			if len(taskState.History) > 10 {
				start = len(taskState.History) - 10
			}
			var histLines []string
			for _, h := range taskState.History[start:] {
				histLines = append(histLines, fmt.Sprintf("step %d: %s → %s", h.Step, h.ToolName, h.Action))
			}
			stateLines = append(stateLines, fmt.Sprintf("  Recent actions: %s", strings.Join(histLines, ", ")))
		}
		parts = append(parts, strings.Join(stateLines, "\n"))
	}

	parts = append(parts, fmt.Sprintf("Tool: %s", toolName))

	if desc, ok := toolSchema["description"].(string); ok && desc != "" {
		parts = append(parts, fmt.Sprintf("Description: %s", desc))
	}

	if params, ok := toolSchema["parameters"]; ok {
		paramsJSON, _ := json.MarshalIndent(params, "", "  ")
		parts = append(parts, fmt.Sprintf("Parameters schema:\n%s", string(paramsJSON)))
	}

	if len(examples) > 0 {
		var exLines []string
		for i, ex := range examples {
			inputJSON, _ := json.Marshal(ex["input"])
			outputJSON, _ := json.Marshal(ex["output"])
			exLines = append(exLines, fmt.Sprintf("Example %d:\n  Input:  %s\n  Output: %s", i+1, string(inputJSON), string(outputJSON)))
		}
		parts = append(parts, "Historical examples from real usage:\n"+strings.Join(exLines, "\n"))
	}

	inputJSON, _ := json.MarshalIndent(inputData, "", "  ")
	parts = append(parts, fmt.Sprintf("Input:\n%s", string(inputJSON)))

	return strings.Join(parts, "\n\n")
}

// buildWorldContextPrompt constructs the prompt to analyze traces and produce a world context.
func buildWorldContextPrompt(traceData string) string {
	return fmt.Sprintf(`Analyze these recent traces from a software product and produce a concise world context.

The context should describe:
1. What this product/service does (1-2 sentences)
2. Key entities and their relationships (e.g., users, orders, tickets)
3. Typical data patterns (e.g., ID formats, status values, naming conventions)
4. Business rules visible in the data (e.g., tier-based policies, workflows)

Be specific — use actual values and patterns you see in the traces.
Keep it under 500 words. This context will be used to generate consistent synthetic tool responses.

Traces:
%s`, traceData)
}
