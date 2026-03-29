package domain

// APIKeyScope represents the permissions granted to an API key.
type APIKeyScope string

const (
	ScopeTracesWrite    APIKeyScope = "traces:write"
	ScopeTracesRead     APIKeyScope = "traces:read"
	ScopeAnalyticsRead  APIKeyScope = "analytics:read"
	ScopeAdmin          APIKeyScope = "admin"
	ScopeSyntheticWrite APIKeyScope = "synthetic:write"
)

// AuthContext represents the authenticated context from a valid API key.
type AuthContext struct {
	OrganizationID string
	ProjectID      string
	APIKeyID       string
	Scopes         []APIKeyScope
	RateLimit      int
}

// HasScope returns true if the auth context has the given scope.
func (a *AuthContext) HasScope(scope APIKeyScope) bool {
	for _, s := range a.Scopes {
		if s == scope || s == ScopeAdmin {
			return true
		}
	}
	return false
}
