package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/allocator"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/api"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/auth"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/config"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/firewall"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/ipresolver"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/player"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/scheduler"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/telemetry"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/version"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/webhook"
	"github.com/AhmadShamli/Bedrock-Server-Manager/web"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version", "-v", "--version":
			fmt.Printf("%s v%s (Linux static) - %s\n", version.AppName, version.Version, version.RepositoryURL)
			return
		case "help", "-h", "--help":
			fmt.Printf("Usage: %s [serve|version|help]\n\nCommands:\n  serve    Run the Bedrock Server Manager daemon (default)\n  version  Display current version and repository info\n  help     Display usage help\n", os.Args[0])
			return
		case "serve":
			// Proceed to daemon start
		default:
			if len(os.Args[1]) > 0 && os.Args[1][0] == '-' {
				fmt.Fprintf(os.Stderr, "Unknown flag: %s\nRun '%s help' for usage.\n", os.Args[1], os.Args[0])
				os.Exit(1)
			}
		}
	}

	cfg := config.LoadConfig()

	log.Println("======================================================")
	log.Printf("     %s v%s - DAEMON START      \n", version.AppName, version.Version)
	log.Println("======================================================")

	// 1. Open Primary Database
	mgrDB, err := database.OpenManagerDB(cfg.ManagerDBPath)
	if err != nil {
		log.Fatalf("[FATAL] Failed to open manager database at %s: %v", cfg.ManagerDBPath, err)
	}
	defer mgrDB.Close()
	log.Printf("[DB] Manager database initialized at %s", cfg.ManagerDBPath)

	// 2. Open Telemetry Database
	metricsDB, err := database.OpenMetricsDB(cfg.MetricsDBPath)
	if err != nil {
		log.Fatalf("[FATAL] Failed to open metrics database at %s: %v", cfg.MetricsDBPath, err)
	}
	defer metricsDB.Close()
	log.Printf("[DB] Metrics database initialized at %s", cfg.MetricsDBPath)

	ctx := context.Background()

	// 3. Resolve JWT Secret
	jwtSecretStr := cfg.JWTSecret
	if jwtSecretStr == "" {
		stored, err := mgrDB.GetSetting(ctx, "jwt_secret")
		if err == nil && stored != "" {
			jwtSecretStr = stored
		} else {
			generated, _ := auth.GenerateRandomToken(32)
			jwtSecretStr = generated
			_ = mgrDB.SetSetting(ctx, "jwt_secret", jwtSecretStr)
			log.Println("[Security] Generated and stored persistent JWT secret")
		}
	}

	// 4. Resolve Server Pepper for HMAC-SHA256 Knock Keys
	pepperStr := cfg.Pepper
	if pepperStr == "" {
		stored, err := mgrDB.GetSetting(ctx, "pepper")
		if err == nil && stored != "" {
			pepperStr = stored
		} else {
			generated, _ := auth.GenerateRandomToken(32)
			pepperStr = generated
			_ = mgrDB.SetSetting(ctx, "pepper", pepperStr)
			log.Println("[Security] Generated and stored persistent HMAC pepper")
		}
	}

	// 5. Ensure Heartbeat Interval Setting exists (default: 10s)
	if _, err := mgrDB.GetSetting(ctx, "heartbeat_interval_seconds"); err != nil {
		_ = mgrDB.SetSetting(ctx, "heartbeat_interval_seconds", strconv.Itoa(int(cfg.HeartbeatInterval.Seconds())))
	}

	// 6. Initialize Webhook Dispatcher
	webhookDispatcher := webhook.NewDispatcher()

	// 7. Initialize Player Manager
	playerMgr := player.NewManager(
		func(serverID, gamertag, xuid string) {
			log.Printf("[PlayerHub] Player '%s' (XUID: %s) joined %s", gamertag, xuid, serverID)
			webhookURL, _ := mgrDB.GetSetting(context.Background(), "discord_webhook_url")
			if webhookURL != "" {
				_ = webhookDispatcher.NotifyPlayerJoined(context.Background(), webhookURL, serverID, gamertag, 1)
			}
		},
		func(serverID, gamertag, xuid string) {
			log.Printf("[PlayerHub] Player '%s' (XUID: %s) left %s", gamertag, xuid, serverID)
			webhookURL, _ := mgrDB.GetSetting(context.Background(), "discord_webhook_url")
			if webhookURL != "" {
				_ = webhookDispatcher.NotifyPlayerLeft(context.Background(), webhookURL, serverID, gamertag, 0)
			}
		},
	)

	// 8. Initialize Container Orchestration Engine
	var serverEngine engine.ServerEngine
	dockerEng, err := engine.NewDockerEngine(cfg.DockerHost)
	if err == nil {
		serverEngine = dockerEng
		log.Println("[Engine] Docker container orchestrator initialized.")
	} else {
		log.Printf("[WARN] Docker daemon unreachable (%v). Running in simulated fallback mode.", err)
		serverEngine = engine.NewMockEngine()
	}

	// Connect real-time container log feed to Player Hub and Chat feed parser
	serverEngine.SetLogListener(func(serverID, line string) {
		playerMgr.ProcessLine(serverID, line)
	})

	// 9. Initialize Port Allocator & IP Resolver
	portAlloc := allocator.NewPortAllocator()
	resolver := ipresolver.NewResolver(cfg.ProxyMode, cfg.TrustedProxies)
	log.Printf("[Network] Client IP Resolver initialized (Mode: %s)", cfg.ProxyMode)

	// Two-tier rate limiter: Level 1: 5 attempts/5m; Level 2: 10 distinct failed IPs/5m -> 15m circuit breaker
	rateLimiter := auth.NewRateLimiter(5, 5*time.Minute, 10, 5*time.Minute, 15*time.Minute)

	// 10. Initialize Telemetry Collector & Poller
	telemetryCollector := telemetry.NewTelemetryCollector(metricsDB)
	telemetryCollector.Start(30*time.Second, 5*time.Minute, 1*time.Hour)
	defer telemetryCollector.Stop()

	// Background metrics sampler (every 2s default)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			servers, err := mgrDB.ListServers(context.Background())
			if err != nil {
				continue
			}
			for _, s := range servers {
				if s.Status == models.ServerStatusRunning {
					if stats, err := serverEngine.GetContainerStats(context.Background(), &s); err == nil && stats != nil {
						telemetryCollector.Ingest(s.ID, stats.CPUPercent, stats.RAMBytes, stats.PlayerCount)
					}
				}
			}
		}
	}()

	// 11. Boot Manager (auto-start servers marked autostart_on_boot = 1)
	bootMgr := engine.NewBootManager(mgrDB, serverEngine, func(ctx context.Context, serverID string) error {
		_, err := player.SyncServerWithGlobal(ctx, cfg.DataDir, serverID, mgrDB, nil)
		return err
	})
	go func() {
		time.Sleep(1 * time.Second) // Brief pause to let HTTP initialize
		if started, err := bootMgr.AutostartServers(context.Background()); err == nil && len(started) > 0 {
			log.Printf("[BootManager] Successfully autostarted %d servers: %v", len(started), started)
		}
	}()

	// 12. Initialize Dynamic Firewall Driver & Lease Auditor
	var fwDriver firewall.FirewallDriver
	detector := firewall.NewNetNSDetector("auto")
	switch cfg.FirewallDriver {
	case "iptables":
		ipt := firewall.NewIPTablesDriver(detector)
		if err := ipt.Validate(ctx); err == nil {
			fwDriver = ipt
			log.Println("[Firewall] Initialized IPTables driver (dedicated chain: BSM_PORT_GATE)")
		}
	case "custom":
		scriptPath := os.Getenv("BSM_FIREWALL_SCRIPT")
		cust := firewall.NewCustomScriptDriver(scriptPath)
		if err := cust.Validate(ctx); err == nil {
			fwDriver = cust
			log.Printf("[Firewall] Initialized Custom script driver: %s", scriptPath)
		}
	default: // "ufw" or default
		ufw := firewall.NewUFWDriver(detector)
		if err := ufw.Validate(ctx); err == nil {
			fwDriver = ufw
			log.Println("[Firewall] Initialized UFW driver")
		} else {
			ipt := firewall.NewIPTablesDriver(detector)
			if err := ipt.Validate(ctx); err == nil {
				fwDriver = ipt
				log.Println("[Firewall] UFW unavailable; fallen back to IPTables driver")
			}
		}
	}

	if fwDriver == nil {
		log.Println("[Firewall] No supported host firewall detected. Operating in simulated Mock driver mode.")
		fwDriver = firewall.NewMockFirewallDriver()
	}

	leaseAuditor := firewall.NewLeaseAuditor(mgrDB, fwDriver, 15*time.Second)
	leaseAuditor.Start(ctx)
	defer leaseAuditor.Stop()

	// 13. Initialize Task Scheduler (cron automation)
	taskScheduler := scheduler.NewTaskScheduler(mgrDB, serverEngine, cfg.DataDir)
	_ = taskScheduler.Start(ctx)
	defer taskScheduler.Stop()

	// 14. Build Router
	router := api.NewRouter(api.RouterOptions{
		DB:            mgrDB,
		IPResolver:    resolver,
		RateLimiter:   rateLimiter,
		Engine:        serverEngine,
		Firewall:      fwDriver,
		Scheduler:     taskScheduler,
		PortAllocator: portAlloc,
		PlayerManager: playerMgr,
		DataDir:       cfg.DataDir,
		JWTSecret:     []byte(jwtSecretStr),
		Pepper:        pepperStr,
		WebFS:         web.DistFS(),
	})

	// 13. Start HTTP Server
	serverAddr := fmt.Sprintf("0.0.0.0:%d", cfg.Port)
	srv := &http.Server{
		Addr:              serverAddr,
		Handler:           router,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("[HTTP] Web Dashboard & API listening on http://%s", serverAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] HTTP server error: %v", err)
		}
	}()

	// 14. Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[SHUTDOWN] Terminating Bedrock Server Manager gracefully...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[WARN] Server forced to shutdown: %v", err)
	}

	log.Println("[SHUTDOWN] Exited cleanly.")
}
