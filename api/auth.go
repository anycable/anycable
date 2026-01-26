package api

import (
	"net/http"
	"strings"
)

// Authenticator handles API request authentication
type Authenticator struct {
	secret string
}

// NewAuthenticator creates a new Authenticator with the given secret.
func NewAuthenticator(secret string) (*Authenticator, error) {
	return &Authenticator{secret: secret}, nil
}

// Secret returns the configured secret (useful for logging/debugging)
func (a *Authenticator) Secret() string {
	return a.secret
}

// IsEnabled returns true if authentication is configured
func (a *Authenticator) IsEnabled() bool {
	return a.secret != ""
}

// Authenticate checks if the request has valid authentication.
// Returns true if authenticated, false otherwise.
func (a *Authenticator) Authenticate(r *http.Request) bool {
	if a.secret == "" {
		return true
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}

	// Expect "Bearer <token>" format
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return false
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	return token == a.secret
}
