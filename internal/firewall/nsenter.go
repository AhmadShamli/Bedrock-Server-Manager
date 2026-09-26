package firewall

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// FindExecutable searches for an executable in PATH and standard system sbin directories.
func FindExecutable(bin string) string {
	if strings.Contains(bin, "/") {
		return bin
	}
	if p, err := exec.LookPath(bin); err == nil {
		return p
	}
	for _, dir := range []string{"/usr/sbin", "/sbin", "/usr/local/sbin", "/usr/bin", "/bin"} {
		fullPath := filepath.Join(dir, bin)
		if fi, err := os.Stat(fullPath); err == nil && !fi.IsDir() {
			return fullPath
		}
	}
	return bin
}

// NetNSDetector detects whether the current process is in a containerized network namespace
// and automatically wraps firewall CLI calls with nsenter if needed.
type NetNSDetector struct {
	override string // "auto", "true", "false"
}

// NewNetNSDetector creates a new NetNSDetector.
func NewNetNSDetector(override string) *NetNSDetector {
	return &NetNSDetector{override: strings.ToLower(override)}
}

// ShouldUseNsenter determines whether commands targeting host network namespace need nsenter.
func (d *NetNSDetector) ShouldUseNsenter() bool {
	if d.override == "true" {
		return true
	}
	if d.override == "false" {
		return false
	}

	// Auto-detection: compare inode of current net ns to PID 1 net ns
	selfStat, err1 := os.Stat("/proc/self/ns/net")
	initStat, err2 := os.Stat("/proc/1/ns/net")
	if err1 != nil || err2 != nil {
		return false
	}

	selfSys, ok1 := selfStat.Sys().(*syscall.Stat_t)
	initSys, ok2 := initStat.Sys().(*syscall.Stat_t)
	if !ok1 || !ok2 {
		return false
	}

	// If inodes differ, we are running in an isolated container network namespace (e.g. docker bridge)
	return selfSys.Ino != initSys.Ino
}

// WrapCommand prepends nsenter --net=/proc/1/ns/net if nsenter is needed,
// and elevates commands with sudo -n if running as a non-root user (e.g. systemd bedrock user).
func (d *NetNSDetector) WrapCommand(ctx context.Context, cmd string, args ...string) *exec.Cmd {
	resolvedCmd := FindExecutable(cmd)
	baseName := filepath.Base(resolvedCmd)
	isNonRoot := os.Getuid() != 0

	hasSudo := false
	if _, err := exec.LookPath("sudo"); err == nil {
		hasSudo = true
	} else if _, err := os.Stat("/usr/bin/sudo"); err == nil {
		hasSudo = true
	}

	if d.ShouldUseNsenter() {
		nsenterBin := FindExecutable("nsenter")
		nsArgs := append([]string{"--net=/proc/1/ns/net", resolvedCmd}, args...)
		if isNonRoot && hasSudo {
			sudoArgs := append([]string{"-n", nsenterBin}, nsArgs...)
			return exec.CommandContext(ctx, "sudo", sudoArgs...)
		}
		return exec.CommandContext(ctx, nsenterBin, nsArgs...)
	}

	// For host-level calls as non-root user:
	// UFW's Python script explicitly validates os.getuid() == 0 and fails otherwise.
	// Passwordless sudo is granted by /etc/sudoers.d/bedrock-server-manager.
	if isNonRoot && hasSudo && baseName == "ufw" {
		sudoArgs := append([]string{"-n", resolvedCmd}, args...)
		return exec.CommandContext(ctx, "sudo", sudoArgs...)
	}

	return exec.CommandContext(ctx, resolvedCmd, args...)
}

// RunCommand executes a command with nsenter wrapping if applicable.
func (d *NetNSDetector) RunCommand(ctx context.Context, cmd string, args ...string) ([]byte, error) {
	c := d.WrapCommand(ctx, cmd, args...)
	out, err := c.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("command '%s %s' failed: %w (output: %s)", cmd, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return out, nil
}
