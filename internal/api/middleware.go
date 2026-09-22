package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/auth"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/ipresolver"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/go-chi/chi/v5"
)

type Middleware struct {
	db          *database.ManagerDB
	ipResolver  *ipresolver.Resolver
	rateLimiter *auth.RateLimiter
	jwtSecret   []byte
}

func NewMiddleware(db *database.ManagerDB, ipResolver *ipresolver.Resolver, rateLimiter *auth.RateLimiter, jwtSecret []byte) *Middleware {
	return &Middleware{
		db:          db,
		ipResolver:  ipResolver,
		rateLimiter: rateLimiter,
		jwtSecret:   jwtSecret,
	}
}

// ResolveIPMiddleware extracts client IP using the configured resolver and injects into context.
func (m *Middleware) ResolveIPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := m.ipResolver.ResolveClientIP(r)
		next.ServeHTTP(w, WithClientIP(r, ip))
	})
}

// RequireAuth ensures the request has a valid JWT token.
func (m *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tokenStr string

		// Check Authorization header
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenStr = strings.TrimPrefix(authHeader, "Bearer ")
		} else if cookie, err := r.Cookie("bsm_session"); err == nil {
			tokenStr = cookie.Value
		}

		if tokenStr == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Authentication required"})
			return
		}

		claims, err := auth.ValidateJWT(m.jwtSecret, tokenStr)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid or expired session"})
			return
		}

		next.ServeHTTP(w, WithUserClaims(r, claims))
	})
}

// RequireAdmin verifies that the authenticated user has the Admin role.
func (m *Middleware) RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := GetUserClaims(r)
		if claims == nil || claims.Role != models.RoleAdmin {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Admin privileges required"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireServerAccess verifies that the user is an Admin or has explicit Operator access to the server.
func (m *Middleware) RequireServerAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := GetUserClaims(r)
		if claims == nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Authentication required"})
			return
		}

		if claims.Role == models.RoleAdmin {
			next.ServeHTTP(w, r)
			return
		}

		serverID := chi.URLParam(r, "id")
		if serverID == "" {
			serverID = chi.URLParam(r, "server_id")
		}

		if serverID != "" {
			hasAccess, err := m.db.CheckUserServerAccess(r.Context(), claims.UserID, serverID)
			if err != nil || !hasAccess {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "Access denied for this server instance"})
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// RateLimitLoginMiddleware applies progressive backoff for login attempts.
func (m *Middleware) RateLimitLoginMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := GetClientIP(r).String()
		res := m.rateLimiter.CheckLogin(ip, time.Now())
		if res.Blocked {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":               res.Reason,
				"retry_after_seconds": res.RetryAfterSeconds,
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}
