package firewall

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

const ChainName = "BSM_PORT_GATE"

// IPTablesDriver manages host firewall rules using iptables and ip6tables in a dedicated chain.
type IPTablesDriver struct {
	mu       sync.Mutex
	detector *NetNSDetector
}

// NewIPTablesDriver creates a new IPTablesDriver instance.
func NewIPTablesDriver(detector *NetNSDetector) *IPTablesDriver {
	if detector == nil {
		detector = NewNetNSDetector("auto")
	}
	return &IPTablesDriver{detector: detector}
}

func (a *IPTablesDriver) Name() string {
	return "iptables"
}

func (a *IPTablesDriver) Detect(ctx context.Context) (bool, error) {
	_, err := exec.LookPath("iptables")
	return err == nil, nil
}

func (a *IPTablesDriver) Validate(ctx context.Context) error {
	detected, _ := a.Detect(ctx)
	if !detected {
		return fmt.Errorf("iptables not found in system PATH")
	}
	return a.initChains(ctx)
}

func (a *IPTablesDriver) initChains(ctx context.Context) error {
	for _, bin := range []string{"iptables", "ip6tables"} {
		if _, err := exec.LookPath(bin); err != nil {
			continue
		}

		// Create chain if it does not exist
		_, _ = a.detector.RunCommand(ctx, bin, "-N", ChainName)

		// Ensure jump from INPUT chain
		if _, err := a.detector.RunCommand(ctx, bin, "-C", "INPUT", "-j", ChainName); err != nil {
			if _, err := a.detector.RunCommand(ctx, bin, "-I", "INPUT", "1", "-j", ChainName); err != nil {
				return fmt.Errorf("failed to attach %s jump to INPUT: %w", bin, err)
			}
		}

		// If Docker is running, also attach to DOCKER-USER chain to filter forwarded traffic
		if _, err := a.detector.RunCommand(ctx, bin, "-L", "DOCKER-USER", "-n"); err == nil {
			if _, err := a.detector.RunCommand(ctx, bin, "-C", "DOCKER-USER", "-j", ChainName); err != nil {
				_, _ = a.detector.RunCommand(ctx, bin, "-I", "DOCKER-USER", "1", "-j", ChainName)
			}
		}
	}
	return nil
}

func (a *IPTablesDriver) getBin(ip string) string {
	if strings.Contains(ip, ":") {
		return "ip6tables"
	}
	return "iptables"
}

func (a *IPTablesDriver) AllowPort(ctx context.Context, ip string, port int, comment string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	bin := a.getBin(ip)
	if comment == "" {
		comment = fmt.Sprintf("bsm_%d", port)
	}

	args := []string{
		"-A", ChainName,
		"-s", ip,
		"-p", "udp",
		"--dport", strconv.Itoa(port),
		"-m", "comment", "--comment", comment,
		"-j", "ACCEPT",
	}

	_, err := a.detector.RunCommand(ctx, bin, args...)
	return err
}

func (a *IPTablesDriver) RevokePort(ctx context.Context, ip string, port int, comment string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	bin := a.getBin(ip)

	// Attempt deletion with comment first
	if comment != "" {
		argsWithComment := []string{
			"-D", ChainName,
			"-s", ip,
			"-p", "udp",
			"--dport", strconv.Itoa(port),
			"-m", "comment", "--comment", comment,
			"-j", "ACCEPT",
		}
		if _, err := a.detector.RunCommand(ctx, bin, argsWithComment...); err == nil {
			return nil
		}
	}

	// Fallback to deletion without comment
	argsPlain := []string{
		"-D", ChainName,
		"-s", ip,
		"-p", "udp",
		"--dport", strconv.Itoa(port),
		"-j", "ACCEPT",
	}
	_, err := a.detector.RunCommand(ctx, bin, argsPlain...)
	return err
}

func (a *IPTablesDriver) FlushRules(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, bin := range []string{"iptables", "ip6tables"} {
		if _, err := exec.LookPath(bin); err == nil {
			_, _ = a.detector.RunCommand(ctx, bin, "-F", ChainName)
		}
	}
	return nil
}

func (a *IPTablesDriver) ListActiveRules(ctx context.Context) ([]ActiveRule, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	var rules []ActiveRule
	for _, bin := range []string{"iptables", "ip6tables"} {
		if _, err := exec.LookPath(bin); err != nil {
			continue
		}
		out, err := a.detector.RunCommand(ctx, bin, "-S", ChainName)
		if err != nil {
			continue
		}
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			if !strings.HasPrefix(line, "-A "+ChainName) {
				continue
			}
			var r ActiveRule
			r.Backend = bin
			tokens := strings.Fields(line)
			for i, tok := range tokens {
				if tok == "-s" && i+1 < len(tokens) {
					r.IP = strings.TrimSuffix(strings.TrimSuffix(tokens[i+1], "/32"), "/128")
				}
				if tok == "--dport" && i+1 < len(tokens) {
					p, _ := strconv.Atoi(tokens[i+1])
					r.Port = p
				}
				if tok == "--comment" && i+1 < len(tokens) {
					r.Comment = tokens[i+1]
				}
			}
			if r.IP != "" && r.Port > 0 {
				rules = append(rules, r)
			}
		}
	}
	return rules, nil
}
