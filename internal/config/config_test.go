package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigDefaults(t *testing.T) {
	// Clear any environment overrides
	os.Unsetenv("DATA_DIR")
	os.Unsetenv("PORT")
	os.Unsetenv("BSM_CONFIG_FILE")

	cfg := LoadConfig()
	if cfg.Port != 8080 {
		t.Fatalf("expected port 8080, got %d", cfg.Port)
	}
	if cfg.DataDir != "data" {
		t.Fatalf("expected dataDir 'data', got '%s'", cfg.DataDir)
	}
	if cfg.ManagerDBPath != filepath.Join("data", "manager.db") {
		t.Fatalf("unexpected ManagerDBPath: %s", cfg.ManagerDBPath)
	}
	if cfg.MetricsDBPath != filepath.Join("data", "metrics.db") {
		t.Fatalf("unexpected MetricsDBPath: %s", cfg.MetricsDBPath)
	}
}

func TestLoadConfigFromCustomFile(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "test.env")
	customDataDir := filepath.Join(tmpDir, "custom-bedrock-data")

	envContent := `
# Custom test configuration
PORT=9090
DATA_DIR=` + customDataDir + `
PROXY_MODE=cloudflare
FIREWALL_DRIVER=iptables
`
	if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
		t.Fatalf("failed to write test env: %v", err)
	}

	// Clear env variables so file values are used
	os.Unsetenv("DATA_DIR")
	os.Unsetenv("PORT")
	os.Unsetenv("PROXY_MODE")
	os.Unsetenv("FIREWALL_DRIVER")

	cfg := LoadConfig(envPath)
	if cfg.ConfigFile != envPath {
		t.Fatalf("expected ConfigFile '%s', got '%s'", envPath, cfg.ConfigFile)
	}
	if cfg.Port != 9090 {
		t.Fatalf("expected port 9090, got %d", cfg.Port)
	}
	if cfg.DataDir != customDataDir {
		t.Fatalf("expected dataDir '%s', got '%s'", customDataDir, cfg.DataDir)
	}
	if cfg.ManagerDBPath != filepath.Join(customDataDir, "manager.db") {
		t.Fatalf("expected ManagerDBPath in customDataDir, got '%s'", cfg.ManagerDBPath)
	}
	if cfg.MetricsDBPath != filepath.Join(customDataDir, "metrics.db") {
		t.Fatalf("expected MetricsDBPath in customDataDir, got '%s'", cfg.MetricsDBPath)
	}
	if cfg.ProxyMode != "cloudflare" {
		t.Fatalf("expected proxyMode 'cloudflare', got '%s'", cfg.ProxyMode)
	}
	if cfg.FirewallDriver != "iptables" {
		t.Fatalf("expected firewallDriver 'iptables', got '%s'", cfg.FirewallDriver)
	}
}

func TestEnvironmentPrecedenceOverFile(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "precedence.env")

	envContent := `DATA_DIR=/from/file/data`
	_ = os.WriteFile(envPath, []byte(envContent), 0600)

	// Set OS environment variable
	overrideDir := filepath.Join(tmpDir, "from-env-override")
	os.Setenv("DATA_DIR", overrideDir)
	defer os.Unsetenv("DATA_DIR")

	cfg := LoadConfig(envPath)
	if cfg.DataDir != overrideDir {
		t.Fatalf("expected env override '%s', got '%s'", overrideDir, cfg.DataDir)
	}
}
