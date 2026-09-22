package models

import "time"

// User roles
const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
)

// Server status
const (
	ServerStatusStopped  = "stopped"
	ServerStatusStarting = "starting"
	ServerStatusRunning  = "running"
	ServerStatusStopping = "stopping"
	ServerStatusCrashed  = "crashed"
)

// Port gate modes
const (
	PortGateModeGamertag   = "gamertag"
	PortGateModePassphrase = "passphrase"
	PortGateModeCombined   = "combined"
)

// User represents a system administrator or server operator.
type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// UserServerAccess maps operator access to specific servers.
type UserServerAccess struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	ServerID  string    `json:"server_id"`
	CreatedAt time.Time `json:"created_at"`
}

// Server represents a managed Minecraft Bedrock Dedicated Server instance.
type Server struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	Version         string    `json:"version"`
	Port            int       `json:"port"`
	PortV6          int       `json:"portv6"`
	Status          string    `json:"status"`
	Mode            string    `json:"mode"`
	Difficulty      string    `json:"difficulty"`
	AutostartOnBoot bool      `json:"autostart_on_boot"`
	PortGateEnabled bool      `json:"port_gate_enabled"`
	PortGateMode    string    `json:"port_gate_mode"`
	PortGateTimeout int       `json:"port_gate_timeout"` // in seconds
	MemoryLimit     string    `json:"memory_limit"`      // e.g. "2G"
	CPULimit        float64   `json:"cpu_limit"`         // e.g. 2.0
	ContainerID     string    `json:"container_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// PortGateKey represents an access passphrase/key for unlocking the port gate.
type PortGateKey struct {
	ID                   int64      `json:"id"`
	ServerID             *string    `json:"server_id,omitempty"` // nil = Global Key
	Label                string     `json:"label"`
	KeyHash              string     `json:"-"`
	KeyPrefix            string     `json:"key_prefix"` // e.g. first 4 chars for identification
	MaxUses              int        `json:"max_uses"`   // 0 = unlimited
	UsedCount            int        `json:"used_count"`
	LeaseDurationSeconds int        `json:"lease_duration_seconds"` // 0 = default
	ExpiresAt            *time.Time `json:"expires_at,omitempty"`
	IsActive             bool       `json:"is_active"`
	CreatedAt            time.Time  `json:"created_at"`
}

// PortGateLease represents an active or expired dynamic firewall grant for a client IP.
type PortGateLease struct {
	ID               int64     `json:"id"`
	ServerID         string    `json:"server_id"`
	KeyID            *int64    `json:"key_id,omitempty"`
	IPAddress        string    `json:"ip_address"`
	Gamertag         string    `json:"gamertag,omitempty"`
	KnockMethod      string    `json:"knock_method"`
	SessionTokenHash string    `json:"-"`
	GrantedAt        time.Time `json:"granted_at"`
	ExpiresAt        time.Time `json:"expires_at"`
	Comment          string    `json:"comment"`
	Status           string    `json:"status"` // active, expired, revoked
}

// Backup represents a hot or cold server archive.
type Backup struct {
	ID        int64     `json:"id"`
	ServerID  string    `json:"server_id"`
	Filename  string    `json:"filename"`
	SizeBytes int64     `json:"size_bytes"`
	Type      string    `json:"type"` // manual, scheduled, pre-update
	IsLocked  bool      `json:"is_locked"`
	Status    string    `json:"status"` // in_progress, completed, failed
	CreatedAt time.Time `json:"created_at"`
}

// Task represents a scheduled automation (restart, backup, command).
type Task struct {
	ID        int64      `json:"id"`
	ServerID  *string    `json:"server_id,omitempty"`
	Name      string     `json:"name"`
	CronExpr  string     `json:"cron_expr"`
	Action    string     `json:"action"` // restart, backup, command
	Payload   string     `json:"payload"`
	LastRun   *time.Time `json:"last_run,omitempty"`
	NextRun   *time.Time `json:"next_run,omitempty"`
	Enabled   bool       `json:"enabled"`
	CreatedAt time.Time  `json:"created_at"`
}

// AuditLog records administrator and system security events.
type AuditLog struct {
	ID        int64     `json:"id"`
	UserID    *int64    `json:"user_id,omitempty"`
	ActorType string    `json:"actor_type"` // user, system, player
	ActorName string    `json:"actor_name"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Details   string    `json:"details"`
	ClientIP  string    `json:"client_ip"`
	Timestamp time.Time `json:"timestamp"`
}

// MetricRaw represents a high-resolution telemetry sample.
type MetricRaw struct {
	ID          int64     `json:"id"`
	ServerID    string    `json:"server_id"`
	Timestamp   time.Time `json:"timestamp"`
	CPUPercent  float64   `json:"cpu_percent"`
	RAMBytes    int64     `json:"ram_bytes"`
	PlayerCount int       `json:"player_count"`
}

// MetricRollup represents a downsampled summary of telemetry (5m or 1h).
type MetricRollup struct {
	ID          int64     `json:"id"`
	ServerID    string    `json:"server_id"`
	Timestamp   time.Time `json:"timestamp"`
	AvgCPU      float64   `json:"avg_cpu"`
	MaxCPU      float64   `json:"max_cpu"`
	MinCPU      float64   `json:"min_cpu"`
	AvgRAM      int64     `json:"avg_ram"`
	MaxRAM      int64     `json:"max_ram"`
	MinRAM      int64     `json:"min_ram"`
	AvgPlayers  float64   `json:"avg_players"`
	MaxPlayers  int       `json:"max_players"`
	SampleCount int       `json:"sample_count"`
}
