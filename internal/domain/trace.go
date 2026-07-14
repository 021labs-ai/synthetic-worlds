package domain

import "time"

// Trace represents an imported trace record.
type Trace struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// ToolSpan represents a tool call span extracted from a trace.
// This is the minimal data needed by the synthetic worlds feature.
type ToolSpan struct {
	ID        string    `json:"id"`
	TraceID   string    `json:"trace_id"`
	Name      string    `json:"name"`
	Input     any       `json:"input"`
	Output    any       `json:"output"`
	CreatedAt time.Time `json:"created_at"`
}

// TraceImportBatch represents a batch of traces to import.
type TraceImportBatch struct {
	Traces []TraceImport `json:"traces" validate:"required,min=1,max=100"`
}

// TraceImport represents a single trace with its tool spans for import.
type TraceImport struct {
	Name  string           `json:"name"`
	Spans []ToolSpanImport `json:"spans" validate:"required,min=1"`
}

// ToolSpanImport represents a tool span to import.
type ToolSpanImport struct {
	Name   string `json:"name" validate:"required"`
	Input  any    `json:"input"`
	Output any    `json:"output"`
}

// TraceImportResult represents the result of a trace import operation.
type TraceImportResult struct {
	TracesImported int `json:"traces_imported"`
	SpansImported  int `json:"spans_imported"`
}
