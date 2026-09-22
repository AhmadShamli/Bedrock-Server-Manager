package config

import (
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds the runtime configuration for the manager.
type Config struct {
	Port              int
	DataDir           string
	ManagerDBPath     string
	MetricsDBPath     string
	JWTSecret         string
	Pepper            string
	ProxyMode         string
	TrustedProxies    []netip.Prefix
	FirewallDriver    string
	HeartbeatInterval time.Duration
	DockerHost        string
}

// LoadConfig reads configuration from environment variables with sensible defaults.
func LoadConfig() *Config {
	port := 8080
	if p := os.Getenv("PORT"); p != "" {
		if val, err := strconv.Atoi(p); err == nil && val > 0 && val <= 65535 {
			port = val
		}
	}

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}

	proxyMode := strings.ToLower(os.Getenv("PROXY_MODE"))
	if proxyMode == "" {
		proxyMode = "direct"
	}

	var trustedProxies []netip.Prefix
	if tp := os.Getenv("TRUSTED_PROXIES"); tp != "" {
		for _, raw := range strings.Split(tp, ",") {
			raw = strings.TrimSpace(raw)
			if prefix, err := netip.ParsePrefix(raw); err == nil {
				trustedProxies = append(trustedProxies, prefix)
			}
		}
	}

	firewallDriver := strings.ToLower(os.Getenv("FIREWALL_DRIVER"))
	if firewallDriver == "" {
		firewallDriver = "ufw"
	}

	hbInterval := 10 * time.Second
	if hb := os.Getenv("HEARTBEAT_INTERVAL_SECONDS"); hb != "" {
		if val, err := strconv.Atoi(hb); err == nil && val > 0 {
			hbInterval = time.Duration(val) * time.Second
		}
	}

	return &Config{
		Port:              port,
		DataDir:           dataDir,
		ManagerDBPath:     filepath.Join(dataDir, "manager.db"),
		MetricsDBPath:     filepath.Join(dataDir, "metrics.db"),
		JWTSecret:         os.Getenv("JWT_SECRET"),
		Pepper:            os.Getenv("PEPPER"),
		ProxyMode:         proxyMode,
		TrustedProxies:    trustedProxies,
		FirewallDriver:    firewallDriver,
		HeartbeatInterval: hbInterval,
		DockerHost:        os.Getenv("DOCKER_HOST"),
	}
}
