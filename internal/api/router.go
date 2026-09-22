package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/allocator"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/auth"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/firewall"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/ipresolver"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/player"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/scheduler"
	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type RouterOptions struct {
	DB            *database.ManagerDB
	IPResolver    *ipresolver.Resolver
	RateLimiter   *auth.RateLimiter
	Engine        engine.ServerEngine
	Firewall      firewall.FirewallDriver
	Scheduler     *scheduler.TaskScheduler
	PortAllocator *allocator.PortAllocator
	PlayerManager *player.Manager
	DataDir       string
	JWTSecret     []byte
	Pepper        string
	WebFS         fs.FS
}

// NewRouter constructs and configures the Chi router.
func NewRouter(opts RouterOptions) *chi.Mux {
	r := chi.NewRouter()

	if opts.PlayerManager == nil {
		opts.PlayerManager = player.NewManager(nil, nil)
	}

	mw := NewMiddleware(opts.DB, opts.IPResolver, opts.RateLimiter, opts.JWTSecret)
	authHandler := NewAuthHandler(opts.DB, opts.RateLimiter, opts.JWTSecret)
	systemHandler := NewSystemHandler(opts.DB)
	serverHandler := NewServerHandler(opts.DB, opts.Engine, opts.PortAllocator, opts.DataDir)
	knockHandler := NewKnockHandler(opts.DB, opts.RateLimiter, opts.Pepper, opts.Firewall)
	playerHubHandler := NewPlayerHubHandler(serverHandler, opts.PlayerManager)
	backupHandler := NewBackupHandler(opts.DB, opts.Engine, opts.DataDir)
	addonHandler := NewAddonHandler(opts.DB, opts.DataDir)
	taskHandler := NewTaskHandler(opts.DB, opts.Scheduler)
	globalPlayerHandler := NewGlobalPlayerHandler(opts.DB, opts.Engine, opts.DataDir)

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

			// Presets and Version checking
			authGroup.Get("/presets", ListPresets)
			authGroup.Get("/updater/check", CheckUpdates)

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

				// Config Editor
				srvGroup.Get("/servers/{id}/properties", serverHandler.GetProperties)
				srvGroup.Put("/servers/{id}/properties", serverHandler.UpdateProperties)
				srvGroup.Get("/servers/{id}/allowlist", serverHandler.GetAllowlist)
				srvGroup.Put("/servers/{id}/allowlist", serverHandler.UpdateAllowlist)
				srvGroup.Get("/servers/{id}/permissions", serverHandler.GetPermissions)
				srvGroup.Put("/servers/{id}/permissions", serverHandler.UpdatePermissions)

				// Player Hub & Live Chat
				srvGroup.Get("/servers/{id}/players", playerHubHandler.GetPlayers)
				srvGroup.Get("/servers/{id}/chat", playerHubHandler.GetChatFeed)
				srvGroup.Post("/servers/{id}/broadcast", playerHubHandler.Broadcast)
				srvGroup.Post("/servers/{id}/players/kick", playerHubHandler.KickPlayer)
				srvGroup.Post("/servers/{id}/players/op", playerHubHandler.OpPlayer)
				srvGroup.Post("/servers/{id}/players/deop", playerHubHandler.DeopPlayer)

				// RakNet Ping
				srvGroup.Get("/servers/{id}/ping", playerHubHandler.Ping)

				// Port Gate Leases
				srvGroup.Get("/servers/{id}/leases", knockHandler.ListLeases)
				srvGroup.Post("/servers/{id}/leases/{leaseId}/revoke", knockHandler.RevokeLease)
				srvGroup.Post("/servers/{id}/leases/manual", knockHandler.CreateManualLease)

				// Backups & Worlds
				srvGroup.Get("/servers/{id}/backups", backupHandler.ListBackups)
				srvGroup.Post("/servers/{id}/backups", backupHandler.CreateBackup)
				srvGroup.Post("/servers/{id}/backups/{backupId}/lock", backupHandler.ToggleLock)
				srvGroup.Delete("/servers/{id}/backups/{backupId}", backupHandler.DeleteBackup)
				srvGroup.Get("/servers/{id}/backups/{backupId}/download", backupHandler.DownloadBackup)
				srvGroup.Post("/servers/{id}/backups/{backupId}/restore", backupHandler.RestoreBackup)
				srvGroup.Get("/servers/{id}/world/export", backupHandler.ExportWorld)
				srvGroup.Post("/servers/{id}/world/import", backupHandler.ImportWorld)

				// Addon Manager
				srvGroup.Get("/servers/{id}/addons", addonHandler.List)
				srvGroup.Post("/servers/{id}/addons", addonHandler.Install)
				srvGroup.Delete("/servers/{id}/addons/{type}/{folder}", addonHandler.Delete)

				// Multi-level Player Sync & Promotion
				srvGroup.Post("/servers/{id}/sync-global", globalPlayerHandler.SyncServer)
				srvGroup.Post("/servers/{id}/players/promote-global", globalPlayerHandler.PromotePlayer)
			})

			// Admin-only operations
			authGroup.Group(func(adminGroup chi.Router) {
				adminGroup.Use(mw.RequireAdmin)

				adminGroup.Get("/servers/suggest-ports", serverHandler.SuggestPorts)
				adminGroup.Post("/servers", serverHandler.Create)
				adminGroup.Put("/servers/{id}", serverHandler.Update)
				adminGroup.Delete("/servers/{id}", serverHandler.Delete)
				adminGroup.Post("/servers/{id}/clone", serverHandler.Clone)
				adminGroup.Post("/servers/{id}/copy-configs", serverHandler.CopyConfigs)
				adminGroup.Get("/servers/{id}/export", serverHandler.Export)

				// Global Player Access Control
				adminGroup.Get("/global-players", globalPlayerHandler.List)
				adminGroup.Post("/global-players", globalPlayerHandler.Create)
				adminGroup.Get("/global-players/{id}", globalPlayerHandler.Get)
				adminGroup.Put("/global-players/{id}", globalPlayerHandler.Update)
				adminGroup.Delete("/global-players/{id}", globalPlayerHandler.Delete)
				adminGroup.Post("/global-players/remove-by-name", globalPlayerHandler.RemoveByName)
				adminGroup.Post("/global-players/sync-all", globalPlayerHandler.SyncAll)

				// Task Scheduler
				adminGroup.Get("/tasks", taskHandler.List)
				adminGroup.Post("/tasks", taskHandler.Create)
				adminGroup.Put("/tasks/{id}", taskHandler.Update)
				adminGroup.Delete("/tasks/{id}", taskHandler.Delete)
				adminGroup.Post("/tasks/{id}/toggle", taskHandler.Toggle)
				adminGroup.Post("/tasks/{id}/run", taskHandler.RunNow)

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
