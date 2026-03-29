package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/021labs-ai/synthetic-worlds/internal/ports"
)

type PostgresExampleProvider struct {
	traceRepo *TraceRepository
}

func NewPostgresExampleProvider(traceRepo *TraceRepository) *PostgresExampleProvider {
	return &PostgresExampleProvider{traceRepo: traceRepo}
}

func (p *PostgresExampleProvider) GetWorldContext(ctx context.Context, projectID string) (string, error) {
	spans, err := p.traceRepo.GetRecentToolSpans(ctx, 20)
	if err != nil {
		return "", err
	}
	if len(spans) == 0 {
		return "", nil
	}

	var snippets []string
	for _, span := range spans {
		inputJSON, _ := json.Marshal(span.Input)
		outputJSON, _ := json.Marshal(span.Output)
		snippet := fmt.Sprintf("Tool: %s\n  Input: %s\n  Output: %s", span.Name, string(inputJSON), string(outputJSON))
		snippets = append(snippets, snippet)
	}

	return "Recent tool usage from imported traces:\n\n" + strings.Join(snippets, "\n\n"), nil
}

func (p *PostgresExampleProvider) GetToolExamples(ctx context.Context, projectID, toolName string, limit int) ([]ports.ToolExample, error) {
	spans, err := p.traceRepo.ListToolSpansByTraceName(ctx, toolName, limit)
	if err != nil {
		return nil, err
	}

	var examples []ports.ToolExample
	for _, span := range spans {
		examples = append(examples, ports.ToolExample{
			Input:  span.Input,
			Output: span.Output,
		})
	}

	return examples, nil
}
