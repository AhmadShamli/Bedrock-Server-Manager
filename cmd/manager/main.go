package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/allocator"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/api"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/auth"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/bds"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/config"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/ddns"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/firewall"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/ipresolver"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/mail"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/player"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/raknet"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/resourcemonitor"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/scheduler"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/telemetry"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/version"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/webhook"
	"github.com/AhmadShamli/Bedrock-Server-Manager/web"
)

func main() {
	if handled, err := handleServiceCommand(os.Args[1:]); handled {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Service error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	var customConfigFile string
	var openBrowser bool

	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		switch {
		case arg == "version" || arg == "-v" || arg == "--version":
			fmt.Printf("%s v%s - %s\n", version.AppName, version.Version, version.RepositoryURL)
			return
		case arg == "help" || arg == "-h" || arg == "--help":
			fmt.Printf("Usage: %s [serve] [options]\n\nCommands:\n  serve               Run the Bedrock Server Manager daemon (default)\n  service             Manage Windows background service (install, uninstall, start, stop, status)\n  version             Display current version and repository info\n  help                Display usage help\n\nOptions:\n  -c, --config <file> Specify custom environment/configuration file path\n  --data-dir <path>   Specify custom data directory\n  --open              Automatically open dashboard in browser on launch\n", os.Args[0])
			return
		case arg == "serve":
			// Proceed to daemon start
		case arg == "--open":
			openBrowser = true
		case arg == "-c" || arg == "--config":
			if i+1 < len(os.Args) {
				customConfigFile = os.Args[i+1]
				i++
			} else {
				fmt.Fprintf(os.Stderr, "Error: %s requires a file path\n", arg)
				os.Exit(1)
			}
		case strings.HasPrefix(arg, "--config="):
			customConfigFile = strings.TrimPrefix(arg, "--config=")
		case arg == "--data-dir":
			if i+1 < len(os.Args) {
				os.Setenv("DATA_DIR", os.Args[i+1])
				i++
			}
		case strings.HasPrefix(arg, "--data-dir="):
			os.Setenv("DATA_DIR", strings.TrimPrefix(arg, "--data-dir="))
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(os.Stderr, "Unknown flag: %s\nRun '%s help' for usage.\n", os.Args[i], os.Args[0])
				os.Exit(1)
			}
		}
	}

	cfg := config.LoadConfig(customConfigFile)

	log.Println("======================================================")
	log.Printf("     %s v%s - DAEMON START      \n", version.AppName, version.Version)
	log.Println("======================================================")
	if cfg.ConfigFile != "" {
		log.Printf("[Config] Loaded configuration from %s", cfg.ConfigFile)
	}
	log.Printf("[Config] Active DATA_DIR: %s", cfg.DataDir)

	// Ensure DATA_DIR and necessary subdirectories exist
	subdirs := []string{
		cfg.DataDir,
		filepath.Join(cfg.DataDir, "servers"),
		filepath.Join(cfg.DataDir, "backups"),
	}
	for _, dir := range subdirs {
		if err := os.MkdirAll(dir, 0750); err != nil {
			log.Fatalf("[FATAL] Failed to initialize DATA_DIR directory '%s': %v (ensure the directory exists and is writable)", dir, err)
		}
	}

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

	// 6. Initialize Webhook Dispatcher & Mailer
	webhookDispatcher := webhook.NewDispatcher()
	mailerInstance := mail.NewMailer(mgrDB)
	defer mailerInstance.Close()

	var serverEngine engine.ServerEngine

	// 7. Initialize Player Manager
	playerMgr := player.NewManager(
		func(serverID, gamertag, xuid string) {
			log.Printf("[PlayerHub] Player '%s' (XUID: %s) joined %s", gamertag, xuid, serverID)

			// Record player connection in persistent history
			if err := mgrDB.RecordPlayerConnection(context.Background(), serverID, gamertag, xuid); err != nil {
				log.Printf("[PlayerHub] Failed to record player connection: %v", err)
			}

			// Enforce automated ban if player is banned on instance or globally
			if banned, reason, _ := mgrDB.IsPlayerBanned(context.Background(), serverID, gamertag, xuid); banned {
				log.Printf("[PlayerHub] Enforcing ban on '%s' (XUID: %s) on server %s (Reason: %s)", gamertag, xuid, serverID, reason)
				if s, err := mgrDB.GetServer(context.Background(), serverID); err == nil && serverEngine != nil {
					_ = serverEngine.SendConsoleCommand(context.Background(), s, fmt.Sprintf("kick %s %s", gamertag, reason))
				}
				return
			}

			webhookURL, _ := mgrDB.GetSetting(context.Background(), "discord_webhook_url")
			if webhookURL != "" {
				_ = webhookDispatcher.NotifyPlayerJoined(context.Background(), webhookURL, serverID, gamertag, 1)
			}
		},
		func(serverID, gamertag, xuid string) {
			log.Printf("[PlayerHub] Player '%s' (XUID: %s) left %s", gamertag, xuid, serverID)

			// Update player last seen timestamp
			_ = mgrDB.UpdatePlayerLastSeen(context.Background(), serverID, gamertag)

			webhookURL, _ := mgrDB.GetSetting(context.Background(), "discord_webhook_url")
			if webhookURL != "" {
				_ = webhookDispatcher.NotifyPlayerLeft(context.Background(), webhookURL, serverID, gamertag, 0)
			}
		},
	)

	// 8. Initialize Container & Process Orchestration Engine
	bdsCache := bds.NewCacheManager(cfg.DataDir, nil)
	bdsResolver := bds.NewResolver(nil)
	nativeEng, err := engine.NewNativeEngine(cfg.DataDir, bdsCache, bdsResolver, mgrDB)
	if err != nil {
		log.Printf("[Engine] Warning: native engine init: %v", err)
	}

	var dockerEng engine.ServerEngine
	if d, err := engine.NewDockerEngine(cfg.DockerHost); err == nil {
		dockerEng = d
		log.Println("[Engine] Docker container orchestrator initialized.")
	} else {
		if runtime.GOOS == "windows" {
			log.Printf("[Engine] Docker unreachable (%v). Native BDS engine active; Docker instances unavailable.", err)
		} else {
			log.Printf("[WARN] Docker daemon unreachable (%v). Running in simulated fallback mode.", err)
			dockerEng = engine.NewMockEngine()
		}
	}

	serverEngine = engine.NewEngineRouter(dockerEng, nativeEng, mgrDB)

	// Connect real-time container log feed to Player Hub and Chat feed parser
	serverEngine.SetLogListener(func(serverID, line string) {
		playerMgr.ProcessLine(serverID, line)
	})

	// 9. Initialize Port Allocator & IP Resolver
	portAlloc := allocator.NewPortAllocator()
	resolver := ipresolver.NewResolver(cfg.ProxyMode, cfg.TrustedProxies)
	log.Printf("[Network] Client IP Resolver initialized (Mode: %s)", cfg.ProxyMode)

	// Two-tier rate limiter: Level 1: 5 attempts/5m; Level 2: 10 distinct failed IPs/5m -> 15m circuit breaker (default)
	maxAttempts := 5
	lockoutDuration := 5 * time.Minute
	cbThreshold := 10
	cbLockout := 15 * time.Minute

	if val, err := mgrDB.GetSetting(context.Background(), "rate_limit_max_attempts"); err == nil && val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			maxAttempts = n
		}
	}
	if val, err := mgrDB.GetSetting(context.Background(), "rate_limit_lockout_seconds"); err == nil && val != "" {
		if s, err := strconv.Atoi(val); err == nil && s > 0 {
			lockoutDuration = time.Duration(s) * time.Second
		}
	}
	if val, err := mgrDB.GetSetting(context.Background(), "circuit_breaker_threshold"); err == nil && val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			cbThreshold = n
		}
	}
	if val, err := mgrDB.GetSetting(context.Background(), "circuit_breaker_lockout_minutes"); err == nil && val != "" {
		if m, err := strconv.Atoi(val); err == nil && m > 0 {
			cbLockout = time.Duration(m) * time.Minute
		}
	}

	rateLimiter := auth.NewRateLimiter(maxAttempts, lockoutDuration, cbThreshold, 5*time.Minute, cbLockout)

	// 10. Initialize Telemetry Collector & Poller
	telemetryCollector := telemetry.NewTelemetryCollector(metricsDB)
	{
		rawH, m5H, h1D := 2, 24, 7
		if preset, err := mgrDB.GetSetting(context.Background(), "metrics_retention_preset"); err == nil && preset != "" {
			switch preset {
			case "ultra_compact":
				rawH, m5H, h1D = 1, 12, 3
			case "compact":
				rawH, m5H, h1D = 2, 24, 7
			}
		}
		if val, err := mgrDB.GetSetting(context.Background(), "metrics_retention_raw_hours"); err == nil && val != "" {
			if h, err := strconv.Atoi(val); err == nil && h > 0 {
				rawH = h
			}
		}
		if val, err := mgrDB.GetSetting(context.Background(), "metrics_retention_5m_hours"); err == nil && val != "" {
			if h, err := strconv.Atoi(val); err == nil && h > 0 {
				m5H = h
			}
		}
		if val, err := mgrDB.GetSetting(context.Background(), "metrics_retention_1h_days"); err == nil && val != "" {
			if d, err := strconv.Atoi(val); err == nil && d > 0 {
				h1D = d
			}
		}
		telemetryCollector.SetRetention(rawH, m5H, h1D)
	}
	telemetryCollector.Start(30*time.Second, 5*time.Minute, 1*time.Hour)
	defer telemetryCollector.Stop()

	// Initialize Resource Alert Monitor (monitors usage vs allocated, broadcasts alert every 1m if nearly full)
	resourceMonitor := resourcemonitor.NewResourceMonitor(serverEngine, playerMgr, mgrDB, webhookDispatcher)
	resourceMonitor.SetMailer(mailerInstance)

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
				if st, err := serverEngine.GetServerStatus(context.Background(), &s); err == nil && st != s.Status {
					oldStatus := s.Status
					s.Status = st
					_ = mgrDB.UpdateServerStatus(context.Background(), s.ID, st, s.ContainerID)

					if st == models.ServerStatusCrashed && oldStatus == models.ServerStatusRunning {
						if webhookURL, _ := mgrDB.GetSetting(context.Background(), "discord_webhook_url"); webhookURL != "" {
							_ = webhookDispatcher.NotifyServerCrashed(context.Background(), webhookURL, s.Name, s.ID, "Server process exited unexpectedly.")
						}
						appURL, _ := mgrDB.GetSetting(context.Background(), mail.SettingAppURL)
						mailerInstance.SendSystemAlert(context.Background(), "crash", "SERVER_CRASH",
							fmt.Sprintf("Server Crashed: %s", s.Name),
							fmt.Sprintf("Server '%s' (%s) exited unexpectedly.", s.Name, s.ID),
							map[string]string{
								"Server Name": s.Name,
								"Server ID":   s.ID,
								"Status":      "Crashed",
							},
							appURL+"/servers/"+s.ID,
						)
					}
				}
				if s.Status == models.ServerStatusRunning {
					playerCount := len(playerMgr.GetOnlinePlayers(s.ID))
					if playerCount == 0 && s.Port > 0 {
						if pong, err := raknet.PingServer("127.0.0.1", s.Port, 300*time.Millisecond); err == nil && pong.OnlinePlayers > 0 {
							playerCount = pong.OnlinePlayers
						}
					}
					if stats, err := serverEngine.GetContainerStats(context.Background(), &s); err == nil && stats != nil {
						telemetryCollector.Ingest(s.ID, stats.CPUPercent, stats.RAMBytes, playerCount)

						// Check resource usage vs allocated and automatically broadcast every 1m if nearly full
						_, _, _ = resourceMonitor.CheckServer(context.Background(), &s, stats.CPUPercent, stats.RAMBytes)
					}

					// Check idle timeout (0 players) and auto-stop if configured
					_, _ = resourceMonitor.CheckIdle(context.Background(), &s, playerCount, time.Now().UTC())
				} else {
					_, _ = resourceMonitor.CheckIdle(context.Background(), &s, 0, time.Now().UTC())
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
	if runtime.GOOS == "windows" {
		winFw := firewall.NewWindowsFirewallDriver()
		if err := winFw.Validate(ctx); err == nil {
			fwDriver = winFw
			log.Println("[Firewall] Initialized Windows Advanced Firewall (netsh) driver")
		} else {
			log.Printf("[Firewall] Windows firewall unreachable/non-elevated (%v). Operating in simulated Mock driver mode.", err)
			fwDriver = firewall.NewMockFirewallDriver()
		}
	} else {
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
				if iptErr := ipt.Validate(ctx); iptErr == nil {
					fwDriver = ipt
					log.Printf("[Firewall] UFW unavailable (%v); fallen back to IPTables driver", err)
				} else {
					log.Printf("[Firewall] UFW unavailable (%v) and IPTables failed (%v)", err, iptErr)
				}
			}
		}

		if fwDriver == nil {
			log.Println("[Firewall] No supported host firewall detected. Operating in simulated Mock driver mode.")
			fwDriver = firewall.NewMockFirewallDriver()
		}
	}

	// Sync permanent allow rules to firewall
	if err := firewall.SyncPermanentAllowRules(ctx, mgrDB, fwDriver); err != nil {
		log.Printf("[Firewall] Warning: failed to sync permanent allow rules: %v", err)
	}

	leaseAuditor := firewall.NewLeaseAuditor(mgrDB, fwDriver, 15*time.Second)
	leaseAuditor.Start(ctx)
	defer leaseAuditor.Stop()

	// 13. Initialize Task Scheduler (cron automation)
	taskScheduler := scheduler.NewTaskScheduler(mgrDB, serverEngine, cfg.DataDir)
	_ = taskScheduler.Start(ctx)
	defer taskScheduler.Stop()

	// 14. Initialize Dynamic DNS (DDNS) Manager
	ddnsConfig := api.LoadDDNSConfig(ctx, mgrDB)
	ddnsNotifier := &ddnsNotificationBridge{db: mgrDB, dispatcher: webhookDispatcher}
	ddnsSyncer := &ddnsAddressSyncer{db: mgrDB}
	ddnsManager := ddns.NewManager(ddnsConfig, nil, ddnsNotifier, ddnsSyncer)
	ddnsManager.Start()
	defer ddnsManager.Stop()

	// 15. Build Router
	router := api.NewRouter(api.RouterOptions{
		DB:                 mgrDB,
		IPResolver:         resolver,
		RateLimiter:        rateLimiter,
		Engine:             serverEngine,
		Firewall:           fwDriver,
		Scheduler:          taskScheduler,
		PortAllocator:      portAlloc,
		PlayerManager:      playerMgr,
		DataDir:            cfg.DataDir,
		JWTSecret:          []byte(jwtSecretStr),
		Pepper:             pepperStr,
		WebFS:              web.DistFS(),
		TelemetryCollector: telemetryCollector,
		MetricsDB:          metricsDB,
		ResourceMonitor:    resourceMonitor,
		BDSCache:           bdsCache,
		BDSResolver:        bdsResolver,
		DDNSManager:        ddnsManager,
		Mailer:             mailerInstance,
	})

	// 16. Start HTTP Server
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

	log.Printf("[Web] Management Dashboard available at: http://localhost:%d\n", cfg.Port)

	if openBrowser && !isWindowsService() {
		go func() {
			time.Sleep(500 * time.Millisecond)
			dashURL := fmt.Sprintf("http://localhost:%d", cfg.Port)
			if runtime.GOOS == "windows" {
				_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", dashURL).Start()
			} else {
				_ = exec.Command("xdg-open", dashURL).Start()
			}
		}()
	}

	// 16. Service & Graceful Shutdown
	rootCtx, rootCancel := context.WithCancel(context.Background())
	if isWindowsService() {
		go func() {
			if err := runWindowsService(rootCtx, rootCancel); err != nil {
				log.Printf("[Service] Windows service execution error: %v", err)
			}
		}()
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		log.Println("[SHUTDOWN] OS interrupt signal received.")
	case <-rootCtx.Done():
		log.Println("[SHUTDOWN] Service stop requested by SCM.")
	}

	log.Println("[SHUTDOWN] Terminating Bedrock Server Manager gracefully...")

	// Concurrently stop all running native BDS servers with 15s deadline (Decision 33)
	if nativeEng != nil {
		nativeEng.StopAllGracefully(15)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[WARN] Server forced to shutdown: %v", err)
	}

	log.Println("[SHUTDOWN] Exited cleanly.")
}

type ddnsNotificationBridge struct {
	db         *database.ManagerDB
	dispatcher *webhook.Dispatcher
}

func (b *ddnsNotificationBridge) getWebhookURL(ctx context.Context) string {
	u, _ := b.db.GetSetting(ctx, "discord_webhook_url")
	return u
}

func (b *ddnsNotificationBridge) OnIPChanged(ctx context.Context, domain, provider, ipv4, ipv6 string) {
	url := b.getWebhookURL(ctx)
	if url != "" {
		_ = b.dispatcher.NotifyDDNSSynced(ctx, url, domain, provider, ipv4, ipv6, true)
	}
	_ = b.db.CreateAuditLog(ctx, &models.AuditLog{
		ActorType: "system",
		ActorName: "ddns",
		Action:    "ddns_ip_changed",
		Target:    domain,
		Details:   fmt.Sprintf(`{"provider": "%s", "ipv4": "%s", "ipv6": "%s"}`, provider, ipv4, ipv6),
	})
}

func (b *ddnsNotificationBridge) OnSyncFailed(ctx context.Context, domain, provider string, syncErr error) {
	url := b.getWebhookURL(ctx)
	if url != "" {
		_ = b.dispatcher.NotifyDDNSFailed(ctx, url, domain, provider, syncErr)
	}
	_ = b.db.CreateAuditLog(ctx, &models.AuditLog{
		ActorType: "system",
		ActorName: "ddns",
		Action:    "ddns_sync_failed",
		Target:    domain,
		Details:   fmt.Sprintf(`{"provider": "%s", "error": "%v"}`, provider, syncErr),
	})
}

func (b *ddnsNotificationBridge) OnSyncRecovered(ctx context.Context, domain, provider, ipv4, ipv6 string) {
	url := b.getWebhookURL(ctx)
	if url != "" {
		_ = b.dispatcher.NotifyDDNSSynced(ctx, url, domain, provider, ipv4, ipv6, false)
	}
	_ = b.db.CreateAuditLog(ctx, &models.AuditLog{
		ActorType: "system",
		ActorName: "ddns",
		Action:    "ddns_sync_recovered",
		Target:    domain,
		Details:   fmt.Sprintf(`{"provider": "%s", "ipv4": "%s", "ipv6": "%s"}`, provider, ipv4, ipv6),
	})
}

type ddnsAddressSyncer struct {
	db *database.ManagerDB
}

func (s *ddnsAddressSyncer) UpdateGameServerAddress(ctx context.Context, newAddress string) error {
	log.Printf("[DDNS] Propagating game server address to: %s", newAddress)
	return s.db.SetSetting(ctx, "default_game_server_address", newAddress)
}

