package middleware

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/021labs-ai/synthetic-worlds/internal/domain"
	"github.com/021labs-ai/synthetic-worlds/internal/ports"
	"github.com/021labs-ai/synthetic-worlds/pkg/apierror"
)

const AuthContextKey = "auth"

// StaticAuthProvider authenticates against a single static API key.
type StaticAuthProvider struct {
	apiKey string
}

func NewStaticAuthProvider(apiKey string) *StaticAuthProvider {
	return &StaticAuthProvider{apiKey: apiKey}
}

func (p *StaticAuthProvider) Authenticate(ctx context.Context, token string) (*domain.AuthContext, error) {
	if token != p.apiKey {
		return nil, apierror.InvalidKey()
	}
	return &domain.AuthContext{
		OrganizationID: "default",
		ProjectID:      "default",
		APIKeyID:       "static",
		Scopes:         []domain.APIKeyScope{domain.ScopeAdmin},
		RateLimit:      10000,
	}, nil
}

// Auth creates an authentication middleware using the given AuthProvider.
func Auth(provider ports.AuthProvider) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return sendError(c, apierror.MissingAuth())
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			return sendError(c, apierror.MissingAuth())
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			return sendError(c, apierror.MissingAuth())
		}

		authCtx, err := provider.Authenticate(c.Context(), token)
		if err != nil {
			if apiErr, ok := err.(*apierror.APIError); ok {
				return sendError(c, apiErr)
			}
			return sendError(c, apierror.Internal("authentication failed"))
		}

		c.Locals(AuthContextKey, authCtx)
		return c.Next()
	}
}

// RequireScope creates a middleware that requires specific scopes.
func RequireScope(scopes ...domain.APIKeyScope) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authCtx := GetAuthContext(c)
		if authCtx == nil {
			return sendError(c, apierror.MissingAuth())
		}

		for _, scope := range scopes {
			if authCtx.HasScope(scope) {
				return c.Next()
			}
		}

		return sendError(c, apierror.Forbidden(""))
	}
}

func GetAuthContext(c *fiber.Ctx) *domain.AuthContext {
	auth := c.Locals(AuthContextKey)
	if auth == nil {
		return nil
	}
	return auth.(*domain.AuthContext)
}

func sendError(c *fiber.Ctx, err *apierror.APIError) error {
	return c.Status(err.StatusCode).JSON(err.ToResponse())
}
