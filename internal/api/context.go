package api

import (
	"context"
	"encoding/json"
	"net"
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

// GetClientIP retrieves the resolved client IP from request context with fallback to RemoteAddr.
func GetClientIP(r *http.Request) netip.Addr {
	if r == nil {
		return netip.Addr{}
	}
	if val, ok := r.Context().Value(clientIPKey).(netip.Addr); ok && val.IsValid() {
		return val
	}
	if r.RemoteAddr != "" {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		if addr, err := netip.ParseAddr(host); err == nil && addr.IsValid() {
			return addr
		}
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

// writeJSONError safely writes a JSON response with properly encoded and escaped error message.
func writeJSONError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

