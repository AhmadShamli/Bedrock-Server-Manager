package firewall

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// CustomScriptDriver allows operators to integrate arbitrary custom firewall scripts or hooks.
type CustomScriptDriver struct {
	mu         sync.Mutex
	scriptPath string
}

// NewCustomScriptDriver creates a new CustomScriptDriver with the specified script or executable path.
func NewCustomScriptDriver(scriptPath string) *CustomScriptDriver {
	return &CustomScriptDriver{scriptPath: scriptPath}
}

func (c *CustomScriptDriver) Name() string {
	return "custom"
}

func (c *CustomScriptDriver) Detect(ctx context.Context) (bool, error) {
	if c.scriptPath == "" {
		return false, nil
	}
	info, err := os.Stat(c.scriptPath)
	if err != nil {
		return false, nil
	}
	return !info.IsDir(), nil
}

func (c *CustomScriptDriver) Validate(ctx context.Context) error {
	detected, err := c.Detect(ctx)
	if err != nil || !detected {
		return fmt.Errorf("custom script '%s' does not exist or is not accessible", c.scriptPath)
	}
	return nil
}

func (c *CustomScriptDriver) execute(ctx context.Context, action, ip string, port int, comment string) error {
	cmd := exec.CommandContext(ctx, c.scriptPath)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("BSM_ACTION=%s", action),
		fmt.Sprintf("BSM_IP=%s", ip),
		fmt.Sprintf("BSM_PORT=%s", strconv.Itoa(port)),
		fmt.Sprintf("BSM_COMMENT=%s", comment),
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("custom firewall script failed (%w): %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func (c *CustomScriptDriver) AllowPort(ctx context.Context, ip string, port int, comment string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.execute(ctx, "allow", ip, port, comment)
}

func (c *CustomScriptDriver) RevokePort(ctx context.Context, ip string, port int, comment string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.execute(ctx, "revoke", ip, port, comment)
}

func (c *CustomScriptDriver) FlushRules(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.execute(ctx, "flush", "", 0, "")
}

func (c *CustomScriptDriver) ListActiveRules(ctx context.Context) ([]ActiveRule, error) {
	return []ActiveRule{}, nil
}
