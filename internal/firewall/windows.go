package firewall

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
)

const (
	RulePrefix = "BSM_PORT_GATE_"
)

// WindowsFirewallDriver manages dynamic UDP port gate rules via netsh advfirewall.
type WindowsFirewallDriver struct {
	mu        sync.Mutex
	activeIPs map[int]map[string]bool // port -> IP set
}

// NewWindowsFirewallDriver creates a new Windows firewall driver.
func NewWindowsFirewallDriver() *WindowsFirewallDriver {
	return &WindowsFirewallDriver{
		activeIPs: make(map[int]map[string]bool),
	}
}

func (w *WindowsFirewallDriver) Name() string {
	return "windows"
}

func (w *WindowsFirewallDriver) Detect(ctx context.Context) (bool, error) {
	if runtime.GOOS != "windows" {
		return false, fmt.Errorf("windows firewall driver is only supported on Windows")
	}

	cmd := exec.CommandContext(ctx, "netsh", "advfirewall", "show", "currentprofile")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("netsh execution failed (requires Administrator privileges): %w", err)
	}

	// Warn if default inbound action is not Block
	outStr := string(out)
	if strings.Contains(strings.ToLower(outStr), "inbound") && strings.Contains(strings.ToLower(outStr), "allow") {
		log.Printf("[WARN] [WindowsFirewall] Inbound traffic policy is set to Allow in the active firewall profile. Bedrock port gate requires default inbound Block to be effective.")
	}

	return true, nil
}

func (w *WindowsFirewallDriver) Validate(ctx context.Context) error {
	detected, err := w.Detect(ctx)
	if err != nil {
		return err
	}
	if !detected {
		return fmt.Errorf("windows firewall is not accessible")
	}
	return nil
}

func ruleName(port int) string {
	return fmt.Sprintf("%s%d", RulePrefix, port)
}

func (w *WindowsFirewallDriver) AllowPort(ctx context.Context, ip string, port int, comment string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	ipSet, exists := w.activeIPs[port]
	if !exists {
		ipSet = make(map[string]bool)
		w.activeIPs[port] = ipSet
	}
	ipSet[ip] = true

	return w.syncPortRule(ctx, port)
}

func (w *WindowsFirewallDriver) RevokePort(ctx context.Context, ip string, port int, comment string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	ipSet, exists := w.activeIPs[port]
	if !exists {
		return nil
	}
	delete(ipSet, ip)

	return w.syncPortRule(ctx, port)
}

func (w *WindowsFirewallDriver) syncPortRule(ctx context.Context, port int) error {
	rName := ruleName(port)
	ipSet := w.activeIPs[port]

	if len(ipSet) == 0 {
		// Zero Lease State Machine (Decision 37):
		// When 0 leases exist, set enable=no on the rule so inbound UDP is blocked by default policy
		_ = w.ensureRuleExists(ctx, port)
		cmd := exec.CommandContext(ctx, "netsh", "advfirewall", "firewall", "set", "rule", fmt.Sprintf("name=%s", rName), "new", "enable=no")
		_ = cmd.Run()
		return nil
	}

	// Collect and sort IPs
	ips := make([]string, 0, len(ipSet))
	for ip := range ipSet {
		ips = append(ips, ip)
	}
	sort.Strings(ips)
	ipList := strings.Join(ips, ",")

	_ = w.ensureRuleExists(ctx, port)
	cmd := exec.CommandContext(ctx, "netsh", "advfirewall", "firewall", "set", "rule", fmt.Sprintf("name=%s", rName), "new", fmt.Sprintf("remoteip=%s", ipList), "enable=yes")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update firewall rule %s: %w (output: %s)", rName, err, string(out))
	}

	return nil
}

func (w *WindowsFirewallDriver) ensureRuleExists(ctx context.Context, port int) error {
	rName := ruleName(port)

	// Check if rule already exists
	checkCmd := exec.CommandContext(ctx, "netsh", "advfirewall", "firewall", "show", "rule", fmt.Sprintf("name=%s", rName))
	if err := checkCmd.Run(); err == nil {
		return nil
	}

	// Add base rule initially disabled
	addCmd := exec.CommandContext(ctx, "netsh", "advfirewall", "firewall", "add", "rule",
		fmt.Sprintf("name=%s", rName),
		"dir=in",
		"action=allow",
		"protocol=UDP",
		fmt.Sprintf("localport=%d", port),
		"enable=no",
	)
	_ = addCmd.Run()
	return nil
}

func (w *WindowsFirewallDriver) FlushRules(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Clean all rules matching BSM_PORT_GATE_
	for port := range w.activeIPs {
		rName := ruleName(port)
		delCmd := exec.CommandContext(ctx, "netsh", "advfirewall", "firewall", "delete", "rule", fmt.Sprintf("name=%s", rName))
		_ = delCmd.Run()
	}

	w.activeIPs = make(map[int]map[string]bool)
	return nil
}

func (w *WindowsFirewallDriver) ListActiveRules(ctx context.Context) ([]ActiveRule, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	var rules []ActiveRule
	for port, ipSet := range w.activeIPs {
		for ip := range ipSet {
			rules = append(rules, ActiveRule{
				Backend: "windows",
				IP:      ip,
				Port:    port,
				Comment: ruleName(port),
			})
		}
	}
	return rules, nil
}

// CleanInstanceFirewallRules removes BSM firewall rules for specified ports (e.g. on server deletion).
func (w *WindowsFirewallDriver) CleanInstanceFirewallRules(ctx context.Context, ports ...int) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for _, port := range ports {
		if port <= 0 {
			continue
		}
		delete(w.activeIPs, port)
		rName := ruleName(port)
		cmd := exec.CommandContext(ctx, "netsh", "advfirewall", "firewall", "delete", "rule", fmt.Sprintf("name=%s", rName))
		_ = cmd.Run()
	}
}
