package api

import (
	"context"
	"net/http"
	"net/netip"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/auth"
)

type contextKey string

const (
	clientIPKey contextKey = "bsm_client_ip"
	userKey     contextKey = "bsm_user_claims"
)

// WithClientIP injects resolved client IP into the request context.
func WithClientIP(r *http.Request, ip netip.Addr) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), clientIPKey, ip))
}

// GetClientIP retrieves the resolved client IP from request context.
func GetClientIP(r *http.Request) netip.Addr {
	if val, ok := r.Context().Value(clientIPKey).(netip.Addr); ok {
		return val
	}
	return netip.Addr{}
}

// WithUserClaims injects authenticated user claims into context.
func WithUserClaims(r *http.Request, claims *auth.Claims) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), userKey, claims))
}

// GetUserClaims retrieves authenticated user claims from context.
func GetUserClaims(r *http.Request) *auth.Claims {
	if val, ok := r.Context().Value(userKey).(*auth.Claims); ok {
		return val
	}
	return nil
}
