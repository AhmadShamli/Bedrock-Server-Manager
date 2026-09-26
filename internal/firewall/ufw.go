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

// UFWDriver manages firewall rules using Ubuntu's Uncomplicated Firewall (UFW).
type UFWDriver struct {
	mu       sync.Mutex
	detector *NetNSDetector
}

// NewUFWDriver creates a new UFWDriver instance.
func NewUFWDriver(detector *NetNSDetector) *UFWDriver {
	if detector == nil {
		detector = NewNetNSDetector("auto")
	}
	return &UFWDriver{detector: detector}
}

func (u *UFWDriver) Name() string {
	return "ufw"
}

func (u *UFWDriver) Detect(ctx context.Context) (bool, error) {
	bin := FindExecutable("ufw")
	if _, err := os.Stat(bin); err != nil {
		if _, err := exec.LookPath("ufw"); err != nil {
			return false, fmt.Errorf("ufw executable not found in PATH or standard system directories")
		}
	}

	out, err := u.detector.RunCommand(ctx, "ufw", "status")
	if err != nil {
		return false, fmt.Errorf("ufw status check failed: %w", err)
	}

	outStr := strings.ToLower(string(out))
	if strings.Contains(outStr, "status: active") {
		return true, nil
	}
	if strings.Contains(outStr, "status: inactive") {
		return false, fmt.Errorf("ufw is installed but inactive ('Status: inactive' - run 'sudo ufw enable')")
	}
	if strings.Contains(outStr, "status:") {
		return true, nil
	}
	return false, fmt.Errorf("unexpected ufw output: %s", strings.TrimSpace(string(out)))
}

func (u *UFWDriver) Validate(ctx context.Context) error {
	detected, err := u.Detect(ctx)
	if err != nil {
		return err
	}
	if !detected {
		return fmt.Errorf("ufw is not active or installed")
	}
	return nil
}

func (u *UFWDriver) AllowPort(ctx context.Context, ip string, port int, comment string) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	if comment == "" {
		comment = fmt.Sprintf("bsm_%d", port)
	}

	args := []string{"allow", "proto", "udp", "from", ip, "to", "any", "port", strconv.Itoa(port), "comment", comment}
	_, err := u.detector.RunCommand(ctx, "ufw", args...)
	return err
}

func (u *UFWDriver) RevokePort(ctx context.Context, ip string, port int, comment string) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	args := []string{"delete", "allow", "proto", "udp", "from", ip, "to", "any", "port", strconv.Itoa(port)}
	_, err := u.detector.RunCommand(ctx, "ufw", args...)
	return err
}

func (u *UFWDriver) FlushRules(ctx context.Context) error {
	u.mu.Lock()
	defer u.mu.Unlock()

	rules, err := u.listRulesInternal(ctx)
	if err != nil {
		return err
	}

	for _, r := range rules {
		if strings.HasPrefix(r.Comment, "bsm") {
			args := []string{"delete", "allow", "proto", "udp", "from", r.IP, "to", "any", "port", strconv.Itoa(r.Port)}
			_, _ = u.detector.RunCommand(ctx, "ufw", args...)
		}
	}
	return nil
}

func (u *UFWDriver) ListActiveRules(ctx context.Context) ([]ActiveRule, error) {
	u.mu.Lock()
	defer u.mu.Unlock()

	return u.listRulesInternal(ctx)
}

func (u *UFWDriver) listRulesInternal(ctx context.Context) ([]ActiveRule, error) {
	out, err := u.detector.RunCommand(ctx, "ufw", "status", "numbered")
	if err != nil {
		return nil, err
	}
	return parseUFWStatusLines(string(out)), nil
}

func parseUFWStatusLines(out string) []ActiveRule {
	var rules []ActiveRule
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "ALLOW IN") {
			continue
		}
		idx := strings.Index(line, "]")
		if idx == -1 {
			continue
		}
		rest := strings.TrimSpace(line[idx+1:])
		parts := strings.Fields(rest)
		if len(parts) < 4 {
			continue
		}
		target := parts[0] // e.g. 19132/udp
		fromIP := parts[3]

		comment := ""
		if hashIdx := strings.Index(rest, "#"); hashIdx != -1 {
			comment = strings.TrimSpace(rest[hashIdx+1:])
		}

		targetParts := strings.Split(target, "/")
		if len(targetParts) == 2 && strings.EqualFold(targetParts[1], "udp") {
			port, _ := strconv.Atoi(targetParts[0])
			if port > 0 {
				rules = append(rules, ActiveRule{
					Backend: "ufw",
					IP:      fromIP,
					Port:    port,
					Comment: comment,
				})
			}
		}
	}
	return rules
}
