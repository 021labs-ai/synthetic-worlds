package services

import (
	"context"
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	"github.com/google/uuid"

	"github.com/021labs-ai/synthetic-worlds/internal/domain"
	"github.com/021labs-ai/synthetic-worlds/internal/ports"
)

type TraceImportService struct {
	traceRepo ports.TraceRepository
	log       *zap.Logger
}

func NewTraceImportService(traceRepo ports.TraceRepository, log *zap.Logger) *TraceImportService {
	return &TraceImportService{traceRepo: traceRepo, log: log}
}

func (s *TraceImportService) ImportNative(ctx context.Context, batch domain.TraceImportBatch) (*domain.TraceImportResult, error) {
	var totalTraces, totalSpans int

	for _, traceImport := range batch.Traces {
		trace := &domain.Trace{
			ID:   uuid.New().String(),
			Name: traceImport.Name,
		}
		if err := s.traceRepo.InsertTrace(ctx, trace); err != nil {
			return nil, fmt.Errorf("failed to insert trace: %w", err)
		}
		totalTraces++

		var spans []domain.ToolSpan
		for _, spanImport := range traceImport.Spans {
			spans = append(spans, domain.ToolSpan{
				ID:      uuid.New().String(),
				TraceID: trace.ID,
				Name:    spanImport.Name,
				Input:   spanImport.Input,
				Output:  spanImport.Output,
			})
		}

		if err := s.traceRepo.InsertToolSpans(ctx, spans); err != nil {
			return nil, fmt.Errorf("failed to insert tool spans: %w", err)
		}
		totalSpans += len(spans)
	}

	s.log.Info("traces imported",
		zap.Int("traces", totalTraces),
		zap.Int("spans", totalSpans),
	)

	return &domain.TraceImportResult{
		TracesImported: totalTraces,
		SpansImported:  totalSpans,
	}, nil
}

// ImportLangfuse converts Langfuse export format to native format and imports.
func (s *TraceImportService) ImportLangfuse(ctx context.Context, data []byte) (*domain.TraceImportResult, error) {
	// Langfuse export format: array of traces with observations
	var langfuseData []struct {
		Name         string `json:"name"`
		Observations []struct {
			Name   string `json:"name"`
			Type   string `json:"type"`
			Input  any    `json:"input"`
			Output any    `json:"output"`
		} `json:"observations"`
	}

	if err := json.Unmarshal(data, &langfuseData); err != nil {
		return nil, fmt.Errorf("failed to parse Langfuse data: %w", err)
	}

	batch := domain.TraceImportBatch{}
	for _, trace := range langfuseData {
		traceImport := domain.TraceImport{Name: trace.Name}
		for _, obs := range trace.Observations {
			// Only import GENERATION and TOOL observations
			if obs.Type == "GENERATION" || obs.Type == "TOOL" || obs.Type == "TOOL_CALL" {
				traceImport.Spans = append(traceImport.Spans, domain.ToolSpanImport{
					Name:   obs.Name,
					Input:  obs.Input,
					Output: obs.Output,
				})
			}
		}
		if len(traceImport.Spans) > 0 {
			batch.Traces = append(batch.Traces, traceImport)
		}
	}

	if len(batch.Traces) == 0 {
		return &domain.TraceImportResult{}, nil
	}

	return s.ImportNative(ctx, batch)
}

// ImportLangsmith converts LangSmith export format to native format and imports.
func (s *TraceImportService) ImportLangsmith(ctx context.Context, data []byte) (*domain.TraceImportResult, error) {
	// LangSmith export format: array of runs
	var langsmithData []struct {
		Name      string `json:"name"`
		RunType   string `json:"run_type"`
		Inputs    any    `json:"inputs"`
		Outputs   any    `json:"outputs"`
		ChildRuns []struct {
			Name    string `json:"name"`
			RunType string `json:"run_type"`
			Inputs  any    `json:"inputs"`
			Outputs any    `json:"outputs"`
		} `json:"child_runs"`
	}

	if err := json.Unmarshal(data, &langsmithData); err != nil {
		return nil, fmt.Errorf("failed to parse LangSmith data: %w", err)
	}

	batch := domain.TraceImportBatch{}
	for _, run := range langsmithData {
		traceImport := domain.TraceImport{Name: run.Name}

		// Top-level tool runs
		if run.RunType == "tool" {
			traceImport.Spans = append(traceImport.Spans, domain.ToolSpanImport{
				Name:   run.Name,
				Input:  run.Inputs,
				Output: run.Outputs,
			})
		}

		// Child tool runs
		for _, child := range run.ChildRuns {
			if child.RunType == "tool" {
				traceImport.Spans = append(traceImport.Spans, domain.ToolSpanImport{
					Name:   child.Name,
					Input:  child.Inputs,
					Output: child.Outputs,
				})
			}
		}

		if len(traceImport.Spans) > 0 {
			batch.Traces = append(batch.Traces, traceImport)
		}
	}

	if len(batch.Traces) == 0 {
		return &domain.TraceImportResult{}, nil
	}

	return s.ImportNative(ctx, batch)
}
