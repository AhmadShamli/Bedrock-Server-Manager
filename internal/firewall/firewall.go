package firewall

import (
	"context"
)

// ActiveRule represents a currently active firewall rule.
type ActiveRule struct {
	Backend string `json:"backend"`
	IP      string `json:"ip"`
	Port    int    `json:"port"`
	Comment string `json:"comment,omitempty"`
}

// FirewallDriver defines the common interface for dynamically manipulating host firewall rules.
type FirewallDriver interface {
	// Name returns the driver identifier (e.g. "ufw", "iptables", "custom", "mock").
	Name() string

	// Detect checks whether the firewall tool is available on the system.
	Detect(ctx context.Context) (bool, error)

	// Validate ensures the firewall is properly configured and usable.
	Validate(ctx context.Context) error

	// AllowPort creates an allow rule for incoming UDP traffic from the specified IP to the server port.
	AllowPort(ctx context.Context, ip string, port int, comment string) error

	// RevokePort deletes the allow rule for incoming UDP traffic from the specified IP to the server port.
	RevokePort(ctx context.Context, ip string, port int, comment string) error

	// FlushRules purges all rules created/managed by Bedrock Server Manager.
	FlushRules(ctx context.Context) error

	// ListActiveRules returns all active rules created/managed by this driver.
	ListActiveRules(ctx context.Context) ([]ActiveRule, error)
}
