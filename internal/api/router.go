package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/allocator"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/auth"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/ipresolver"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type RouterOptions struct {
	DB            *database.ManagerDB
	IPResolver    *ipresolver.Resolver
	RateLimiter   *auth.RateLimiter
	Engine        engine.ServerEngine
	PortAllocator *allocator.PortAllocator
	DataDir       string
	JWTSecret     []byte
	Pepper        string
	WebFS         fs.FS
}

// NewRouter constructs and configures the Chi router.
func NewRouter(opts RouterOptions) *chi.Mux {
	r := chi.NewRouter()

	mw := NewMiddleware(opts.DB, opts.IPResolver, opts.RateLimiter, opts.JWTSecret)
	authHandler := NewAuthHandler(opts.DB, opts.RateLimiter, opts.JWTSecret)
	systemHandler := NewSystemHandler(opts.DB)
	serverHandler := NewServerHandler(opts.DB, opts.Engine, opts.PortAllocator, opts.DataDir)
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

			authGroup.Group(func(srvGroup chi.Router) {
				srvGroup.Use(mw.RequireServerAccess)
				srvGroup.Get("/servers/{id}", serverHandler.Get)
				srvGroup.Get("/servers/{id}/stats", serverHandler.Stats)
				srvGroup.Get("/servers/{id}/console/ws", serverHandler.ConsoleWS)
				srvGroup.Post("/servers/{id}/start", serverHandler.Start)
				srvGroup.Post("/servers/{id}/stop", serverHandler.Stop)
				srvGroup.Post("/servers/{id}/restart", serverHandler.Restart)
				srvGroup.Post("/servers/{id}/command", serverHandler.SendCommand)
			})

			// Admin-only operations
			authGroup.Group(func(adminGroup chi.Router) {
				adminGroup.Use(mw.RequireAdmin)

				adminGroup.Get("/servers/suggest-ports", serverHandler.SuggestPorts)
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

			if f, err := opts.WebFS.Open(path); err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, req)
				return
			}

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
