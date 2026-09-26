package config

import (
	"bufio"
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
	ConfigFile        string
}

// LoadEnvFile parses a key-value .env file and sets environment variables if they are not already set.
func LoadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}

		// Only set if not already set in OS environment
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, val)
		}
	}
	return scanner.Err()
}

// LoadConfig reads configuration from environment variables and/or a configuration file with sensible defaults.
// If explicitConfigFile is provided or BSM_CONFIG_FILE is set, it attempts to load that file.
// Otherwise, it checks standard locations: /etc/bedrock-server-manager/bsm.env, ./bsm.env, ./.env.
func LoadConfig(explicitConfigFile ...string) *Config {
	var loadedFile string

	var candidateFiles []string
	for _, p := range explicitConfigFile {
		if p != "" {
			candidateFiles = append(candidateFiles, p)
		}
	}
	if envFile := os.Getenv("BSM_CONFIG_FILE"); envFile != "" {
		candidateFiles = append(candidateFiles, envFile)
	}
	candidateFiles = append(candidateFiles,
		"/etc/bedrock-server-manager/bsm.env",
		"./bsm.env",
		"./.env",
	)

	for _, file := range candidateFiles {
		if fi, err := os.Stat(file); err == nil && !fi.IsDir() {
			if err := LoadEnvFile(file); err == nil {
				loadedFile = file
				break
			}
		}
	}

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
	dataDir = filepath.Clean(dataDir)

	managerDBPath := os.Getenv("MANAGER_DB_PATH")
	if managerDBPath == "" {
		managerDBPath = filepath.Join(dataDir, "manager.db")
	}

	metricsDBPath := os.Getenv("METRICS_DB_PATH")
	if metricsDBPath == "" {
		metricsDBPath = filepath.Join(dataDir, "metrics.db")
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
		ManagerDBPath:     managerDBPath,
		MetricsDBPath:     metricsDBPath,
		JWTSecret:         os.Getenv("JWT_SECRET"),
		Pepper:            os.Getenv("PEPPER"),
		ProxyMode:         proxyMode,
		TrustedProxies:    trustedProxies,
		FirewallDriver:    firewallDriver,
		HeartbeatInterval: hbInterval,
		DockerHost:        os.Getenv("DOCKER_HOST"),
		ConfigFile:        loadedFile,
	}
}
