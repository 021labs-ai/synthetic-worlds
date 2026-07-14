package handlers

import (
	"github.com/gofiber/fiber/v2"

	"github.com/021labs-ai/synthetic-worlds/internal/adapters/http/middleware"
	"github.com/021labs-ai/synthetic-worlds/internal/domain"
	"github.com/021labs-ai/synthetic-worlds/internal/ports"
	"github.com/021labs-ai/synthetic-worlds/pkg/apierror"
)

type SyntheticHandler struct {
	service ports.SyntheticService
}

func NewSyntheticHandler(service ports.SyntheticService) *SyntheticHandler {
	return &SyntheticHandler{service: service}
}

func (h *SyntheticHandler) CreateWorld(c *fiber.Ctx) error {
	authCtx := middleware.GetAuthContext(c)
	if authCtx == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apierror.MissingAuth().ToResponse())
	}

	var params domain.CreateWorldParams
	if err := c.BodyParser(&params); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(apierror.Validation("invalid request body", nil).ToResponse())
	}

	if params.Mode == "" {
		return c.Status(fiber.StatusBadRequest).JSON(apierror.Validation("mode is required", nil).ToResponse())
	}

	result, err := h.service.CreateWorld(c.Context(), authCtx, params)
	if err != nil {
		return handleError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(result)
}

func (h *SyntheticHandler) ExecuteCall(c *fiber.Ctx) error {
	authCtx := middleware.GetAuthContext(c)
	if authCtx == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apierror.MissingAuth().ToResponse())
	}

	var req domain.CallRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(apierror.Validation("invalid request body", nil).ToResponse())
	}

	if req.WorldID == "" || req.ToolName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(apierror.Validation("world_id and tool_name are required", nil).ToResponse())
	}

	result, err := h.service.ExecuteCall(c.Context(), authCtx, req)
	if err != nil {
		return handleError(c, err)
	}

	return c.JSON(result)
}

func (h *SyntheticHandler) ResetWorld(c *fiber.Ctx) error {
	authCtx := middleware.GetAuthContext(c)
	if authCtx == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apierror.MissingAuth().ToResponse())
	}

	worldID := c.Params("id")
	if worldID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(apierror.Validation("world id required", nil).ToResponse())
	}

	var req domain.ResetWorldRequest
	_ = c.BodyParser(&req)

	result, err := h.service.ResetWorld(c.Context(), authCtx, worldID, req.Hard)
	if err != nil {
		return handleError(c, err)
	}

	return c.JSON(result)
}

func (h *SyntheticHandler) DeleteWorld(c *fiber.Ctx) error {
	authCtx := middleware.GetAuthContext(c)
	if authCtx == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apierror.MissingAuth().ToResponse())
	}

	worldID := c.Params("id")
	if worldID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(apierror.Validation("world id required", nil).ToResponse())
	}

	result, err := h.service.CloseWorld(c.Context(), authCtx, worldID)
	if err != nil {
		return handleError(c, err)
	}

	return c.JSON(result)
}

func (h *SyntheticHandler) ListWorlds(c *fiber.Ctx) error {
	authCtx := middleware.GetAuthContext(c)
	if authCtx == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apierror.MissingAuth().ToResponse())
	}

	params := domain.WorldListParams{
		ProjectID: c.Query("project_id"),
		Status:    c.Query("status"),
		Limit:     c.QueryInt("limit", 50),
		Offset:    c.QueryInt("offset", 0),
	}
	if params.Limit < 1 || params.Limit > 200 {
		params.Limit = 50
	}

	result, err := h.service.ListWorlds(c.Context(), authCtx, params)
	if err != nil {
		return handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"worlds": result.Worlds,
		"pagination": fiber.Map{
			"total":    result.Total,
			"limit":    result.Limit,
			"offset":   result.Offset,
			"has_more": result.HasMore,
		},
	})
}

func (h *SyntheticHandler) GetWorld(c *fiber.Ctx) error {
	authCtx := middleware.GetAuthContext(c)
	if authCtx == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apierror.MissingAuth().ToResponse())
	}

	worldID := c.Params("id")
	if worldID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(apierror.Validation("world id required", nil).ToResponse())
	}

	result, err := h.service.GetWorld(c.Context(), authCtx, worldID)
	if err != nil {
		return handleError(c, err)
	}

	return c.JSON(result)
}

func (h *SyntheticHandler) GetWorldState(c *fiber.Ctx) error {
	authCtx := middleware.GetAuthContext(c)
	if authCtx == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apierror.MissingAuth().ToResponse())
	}

	worldID := c.Params("id")
	if worldID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(apierror.Validation("world id required", nil).ToResponse())
	}

	result, err := h.service.GetWorldState(c.Context(), authCtx, worldID)
	if err != nil {
		return handleError(c, err)
	}

	return c.JSON(result)
}

func (h *SyntheticHandler) ListWorldCalls(c *fiber.Ctx) error {
	authCtx := middleware.GetAuthContext(c)
	if authCtx == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apierror.MissingAuth().ToResponse())
	}

	worldID := c.Params("id")
	params := domain.CallListParams{
		WorldID: worldID,
		Limit:   c.QueryInt("limit", 50),
		Offset:  c.QueryInt("offset", 0),
	}
	if params.Limit < 1 || params.Limit > 200 {
		params.Limit = 50
	}

	result, err := h.service.ListCalls(c.Context(), authCtx, params)
	if err != nil {
		return handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"calls": result.Calls,
		"pagination": fiber.Map{
			"total":    result.Total,
			"limit":    result.Limit,
			"offset":   result.Offset,
			"has_more": result.HasMore,
		},
	})
}

func (h *SyntheticHandler) ListCalls(c *fiber.Ctx) error {
	authCtx := middleware.GetAuthContext(c)
	if authCtx == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apierror.MissingAuth().ToResponse())
	}

	params := domain.CallListParams{
		ProjectID: c.Query("project_id"),
		ToolName:  c.Query("tool_name"),
		Limit:     c.QueryInt("limit", 50),
		Offset:    c.QueryInt("offset", 0),
	}
	if params.Limit < 1 || params.Limit > 200 {
		params.Limit = 50
	}

	result, err := h.service.ListCalls(c.Context(), authCtx, params)
	if err != nil {
		return handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"calls": result.Calls,
		"pagination": fiber.Map{
			"total":    result.Total,
			"limit":    result.Limit,
			"offset":   result.Offset,
			"has_more": result.HasMore,
		},
	})
}

func handleError(c *fiber.Ctx, err error) error {
	if apiErr, ok := err.(*apierror.APIError); ok {
		return c.Status(apiErr.StatusCode).JSON(apiErr.ToResponse())
	}
	return c.Status(fiber.StatusInternalServerError).JSON(apierror.Internal("").ToResponse())
}

// SeedFixtures handles POST /v1/synthetic/worlds/:id/fixtures — replaces the
// world's deterministic replay fixtures.
func (h *SyntheticHandler) SeedFixtures(c *fiber.Ctx) error {
	authCtx := middleware.GetAuthContext(c)
	if authCtx == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apierror.MissingAuth().ToResponse())
	}
	worldID := c.Params("id")
	if worldID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(apierror.Validation("world id required", nil).ToResponse())
	}
	var req domain.SeedFixturesRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(apierror.Validation("invalid request body", nil).ToResponse())
	}
	count, err := h.service.SeedFixtures(c.Context(), authCtx, worldID, req.Fixtures)
	if err != nil {
		return handleError(c, err)
	}
	return c.JSON(domain.SeedFixturesResponse{WorldID: worldID, FixtureCount: count})
}

// InvokeReplay handles POST /v1/synthetic/worlds/:id/invoke/:tool — serves the
// call verbatim from the world's fixtures (no LLM), 404 when unseeded.
func (h *SyntheticHandler) InvokeReplay(c *fiber.Ctx) error {
	authCtx := middleware.GetAuthContext(c)
	if authCtx == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(apierror.MissingAuth().ToResponse())
	}
	worldID := c.Params("id")
	toolName := c.Params("tool")
	if worldID == "" || toolName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(apierror.Validation("world id and tool required", nil).ToResponse())
	}
	input := map[string]any{}
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&input); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(apierror.Validation("invalid JSON body", nil).ToResponse())
		}
	}
	result, err := h.service.InvokeReplay(c.Context(), authCtx, worldID, toolName, input)
	if err != nil {
		return handleError(c, err)
	}
	// the OUTPUT alone is the response body — this endpoint plays the role of
	// a provider API, so callers (n8n httpRequest nodes) see the payload itself
	return c.JSON(result.Output)
}
