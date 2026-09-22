package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/auth"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/ipresolver"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type RouterOptions struct {
	DB          *database.ManagerDB
	IPResolver  *ipresolver.Resolver
	RateLimiter *auth.RateLimiter
	JWTSecret   []byte
	Pepper      string
	WebFS       fs.FS
}

// NewRouter constructs and configures the Chi router.
func NewRouter(opts RouterOptions) *chi.Mux {
	r := chi.NewRouter()

	mw := NewMiddleware(opts.DB, opts.IPResolver, opts.RateLimiter, opts.JWTSecret)
	authHandler := NewAuthHandler(opts.DB, opts.RateLimiter, opts.JWTSecret)
	systemHandler := NewSystemHandler(opts.DB)
	serverHandler := NewServerHandler(opts.DB)
	knockHandler := NewKnockHandler(opts.DB, opts.RateLimiter, opts.Pepper)

	// Global Middlewares
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Recoverer)
	r.Use(mw.ResolveIPMiddleware)

	// API Routes
	r.Route("/api", func(api chi.Router) {
		// Health & Status
		api.Get("/health", systemHandler.Health)
		api.Get("/setup/status", authHandler.SetupStatus)
		api.Post("/setup", authHandler.Setup)

		// Public Knock Portal endpoints
		api.Route("/knock/{id}", func(k chi.Router) {
			k.Get("/config", knockHandler.GetConfig)
			k.Post("/", knockHandler.Knock)
			k.Post("/heartbeat", knockHandler.Heartbeat)
			k.Get("/status", knockHandler.Status)
		})

		// Public Auth
		api.With(mw.RateLimitLoginMiddleware).Post("/auth/login", authHandler.Login)
		api.Post("/auth/logout", authHandler.Logout)

		// Authenticated Routes
		api.Group(func(authGroup chi.Router) {
			authGroup.Use(mw.RequireAuth)

			authGroup.Get("/auth/me", authHandler.Me)
			authGroup.Post("/auth/refresh", authHandler.Refresh)
			authGroup.Get("/system/info", systemHandler.SystemInfo)

			// Servers
			authGroup.Get("/servers", serverHandler.List)
			authGroup.With(mw.RequireServerAccess).Get("/servers/{id}", serverHandler.Get)

			// Admin-only operations
			authGroup.Group(func(adminGroup chi.Router) {
				adminGroup.Use(mw.RequireAdmin)

				adminGroup.Post("/servers", serverHandler.Create)
				adminGroup.Put("/servers/{id}", serverHandler.Update)
				adminGroup.Delete("/servers/{id}", serverHandler.Delete)

				adminGroup.Get("/system/audit", systemHandler.ListAuditLogs)
				adminGroup.Get("/system/settings", systemHandler.GetSettings)
				adminGroup.Post("/system/settings", systemHandler.UpdateSetting)
			})
		})
	})

	// Static Web Assets / Single-Page Application (SPA) Serving
	if opts.WebFS != nil {
		fileServer := http.FileServer(http.FS(opts.WebFS))
		r.NotFound(func(w http.ResponseWriter, req *http.Request) {
			path := strings.TrimPrefix(req.URL.Path, "/")
			if path == "" {
				fileServer.ServeHTTP(w, req)
				return
			}

			// If file exists in WebFS, serve it; otherwise serve index.html for client-side routing
			if f, err := opts.WebFS.Open(path); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, req)
				return
			}

			// Fallback to index.html for SPA routes (unless it was an /api route)
			if !strings.HasPrefix(req.URL.Path, "/api") {
				req.URL.Path = "/"
				fileServer.ServeHTTP(w, req)
				return
			}

			http.NotFound(w, req)
		})
	}

	return r
}
