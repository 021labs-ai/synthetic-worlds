package http

import (
	"github.com/gofiber/fiber/v2"

	"github.com/021labs-ai/synthetic-worlds/internal/adapters/http/handlers"
	"github.com/021labs-ai/synthetic-worlds/internal/adapters/http/middleware"
	"github.com/021labs-ai/synthetic-worlds/internal/ports"
)

func RegisterRoutes(
	app *fiber.App,
	syntheticHandler *handlers.SyntheticHandler,
	traceImportHandler *handlers.TraceImportHandler,
	authProvider ports.AuthProvider,
) {
	// Health check
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Authenticated routes
	api := app.Group("/v1", middleware.Auth(authProvider))

	// Synthetic worlds
	synthetic := api.Group("/synthetic")
	synthetic.Post("/worlds", syntheticHandler.CreateWorld)
	synthetic.Post("/call", syntheticHandler.ExecuteCall)
	synthetic.Post("/worlds/:id/reset", syntheticHandler.ResetWorld)
	synthetic.Delete("/worlds/:id", syntheticHandler.DeleteWorld)
	synthetic.Get("/worlds", syntheticHandler.ListWorlds)
	synthetic.Get("/worlds/:id", syntheticHandler.GetWorld)
	synthetic.Get("/worlds/:id/state", syntheticHandler.GetWorldState)
	synthetic.Get("/worlds/:id/calls", syntheticHandler.ListWorldCalls)
	synthetic.Get("/calls", syntheticHandler.ListCalls)

	// Trace import
	traces := api.Group("/traces")
	traces.Post("/import", traceImportHandler.ImportNative)
	traces.Post("/import/langfuse", traceImportHandler.ImportLangfuse)
	traces.Post("/import/langsmith", traceImportHandler.ImportLangsmith)
}
