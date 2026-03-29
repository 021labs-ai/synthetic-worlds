package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/021labs-ai/synthetic-worlds/internal/domain"
	"github.com/021labs-ai/synthetic-worlds/internal/ports"
	"github.com/021labs-ai/synthetic-worlds/pkg/apierror"
)

type TraceImportHandler struct {
	service ports.TraceImportService
}

func NewTraceImportHandler(service ports.TraceImportService) *TraceImportHandler {
	return &TraceImportHandler{service: service}
}

// ImportNative handles POST /v1/traces/import
func (h *TraceImportHandler) ImportNative(c *fiber.Ctx) error {
	var batch domain.TraceImportBatch
	if err := c.BodyParser(&batch); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(apierror.Validation("invalid request body", nil).ToResponse())
	}

	if len(batch.Traces) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(apierror.Validation("at least one trace is required", nil).ToResponse())
	}

	result, err := h.service.ImportNative(c.Context(), batch)
	if err != nil {
		if apiErr, ok := err.(*apierror.APIError); ok {
			return c.Status(apiErr.StatusCode).JSON(apiErr.ToResponse())
		}
		return c.Status(fiber.StatusInternalServerError).JSON(apierror.Internal("import failed").ToResponse())
	}

	return c.Status(fiber.StatusCreated).JSON(result)
}

// ImportLangfuse handles POST /v1/traces/import/langfuse
func (h *TraceImportHandler) ImportLangfuse(c *fiber.Ctx) error {
	result, err := h.service.ImportLangfuse(c.Context(), c.Body())
	if err != nil {
		if apiErr, ok := err.(*apierror.APIError); ok {
			return c.Status(apiErr.StatusCode).JSON(apiErr.ToResponse())
		}
		return c.Status(fiber.StatusInternalServerError).JSON(apierror.Internal("langfuse import failed").ToResponse())
	}

	return c.Status(fiber.StatusCreated).JSON(result)
}

// ImportLangsmith handles POST /v1/traces/import/langsmith
func (h *TraceImportHandler) ImportLangsmith(c *fiber.Ctx) error {
	result, err := h.service.ImportLangsmith(c.Context(), c.Body())
	if err != nil {
		if apiErr, ok := err.(*apierror.APIError); ok {
			return c.Status(apiErr.StatusCode).JSON(apiErr.ToResponse())
		}
		return c.Status(fiber.StatusInternalServerError).JSON(apierror.Internal("langsmith import failed").ToResponse())
	}

	return c.Status(fiber.StatusCreated).JSON(result)
}
