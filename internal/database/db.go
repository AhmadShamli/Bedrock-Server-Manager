package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	_ "modernc.org/sqlite"
)

const TimeLayout = time.RFC3339

func FormatTime(t time.Time) string {
	return t.UTC().Format(TimeLayout)
}

func ParseTime(s string) (time.Time, error) {
	return time.Parse(TimeLayout, s)
}

func ParseNullTime(ns sql.NullString) *time.Time {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	t, err := ParseTime(ns.String)
	if err != nil {
		return nil
	}
	return &t
}

func FormatNullTime(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: FormatTime(*t), Valid: true}
}

// ManagerDB wraps the primary SQLite database.
type ManagerDB struct {
	*sql.DB
}

// MetricsDB wraps the dedicated time-series telemetry SQLite database.
type MetricsDB struct {
	*sql.DB
}

// OpenManagerDB opens the primary metadata database and runs migrations.
func OpenManagerDB(dbPath string) (*ManagerDB, error) {
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory '%s': %w", dir, err)
		}
	}

	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open manager database: %w", err)
	}

	db.SetMaxOpenConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping manager database: %w", err)
	}

	mdb := &ManagerDB{DB: db}
	if err := mdb.Migrate(ctx); err != nil {
		return nil, fmt.Errorf("manager database migration failed: %w", err)
	}

	return mdb, nil
}

// Migrate executes primary schema migrations.
func (db *ManagerDB) Migrate(ctx context.Context) error {
	_, err := db.ExecContext(ctx, ManagerSchemaSQL)
	return err
}

// OpenMetricsDB opens the telemetry time-series database and runs migrations.
func OpenMetricsDB(dbPath string) (*MetricsDB, error) {
	if dbPath != ":memory:" {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create metrics database directory '%s': %w", dir, err)
		}
	}

	dsn := fmt.Sprintf("%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open metrics database: %w", err)
	}

	db.SetMaxOpenConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping metrics database: %w", err)
	}

	metricsDB := &MetricsDB{DB: db}
	if err := metricsDB.Migrate(ctx); err != nil {
		return nil, fmt.Errorf("metrics database migration failed: %w", err)
	}

	return metricsDB, nil
}

// Migrate executes telemetry schema migrations.
func (db *MetricsDB) Migrate(ctx context.Context) error {
	_, err := db.ExecContext(ctx, MetricsSchemaSQL)
	return err
}

// --- Users & Access ---

func (db *ManagerDB) CountUsers(ctx context.Context) (int, error) {
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

func (db *ManagerDB) CreateUser(ctx context.Context, username, passwordHash, role string) (*models.User, error) {
	now := time.Now().UTC()
	nowStr := FormatTime(now)
	res, err := db.ExecContext(ctx,
		"INSERT INTO users (username, password_hash, role, created_at) VALUES (?, ?, ?, ?)",
		username, passwordHash, role, nowStr,
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &models.User{
		ID:           id,
		Username:     username,
		PasswordHash: passwordHash,
		Role:         role,
		CreatedAt:    now,
	}, nil
}

func (db *ManagerDB) GetUserByUsername(ctx context.Context, username string) (*models.User, error) {
	var u models.User
	var createdAtStr string
	err := db.QueryRowContext(ctx,
		"SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?",
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &createdAtStr)
	if err != nil {
		return nil, err
	}
	t, err := ParseTime(createdAtStr)
	if err != nil {
		return nil, err
	}
	u.CreatedAt = t
	return &u, nil
}

func (db *ManagerDB) GetUserByID(ctx context.Context, id int64) (*models.User, error) {
	var u models.User
	var createdAtStr string
	err := db.QueryRowContext(ctx,
		"SELECT id, username, password_hash, role, created_at FROM users WHERE id = ?",
		id,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &createdAtStr)
	if err != nil {
		return nil, err
	}
	t, err := ParseTime(createdAtStr)
	if err != nil {
		return nil, err
	}
	u.CreatedAt = t
	return &u, nil
}

func (db *ManagerDB) ListUsers(ctx context.Context) ([]models.User, error) {
	rows, err := db.QueryContext(ctx, "SELECT id, username, password_hash, role, created_at FROM users ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]models.User, 0)
	for rows.Next() {
		var u models.User
		var createdAtStr string
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &createdAtStr); err != nil {
			return nil, err
		}
		if t, err := ParseTime(createdAtStr); err == nil {
			u.CreatedAt = t
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (db *ManagerDB) UpdateUserPassword(ctx context.Context, id int64, newHash string) error {
	_, err := db.ExecContext(ctx, "UPDATE users SET password_hash = ? WHERE id = ?", newHash, id)
	return err
}

func (db *ManagerDB) DeleteUser(ctx context.Context, id int64) error {
	_, err := db.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)
	return err
}

func (db *ManagerDB) GrantServerAccess(ctx context.Context, userID int64, serverID string) error {
	nowStr := FormatTime(time.Now().UTC())
	_, err := db.ExecContext(ctx,
		"INSERT OR IGNORE INTO user_server_access (user_id, server_id, created_at) VALUES (?, ?, ?)",
		userID, serverID, nowStr,
	)
	return err
}

func (db *ManagerDB) RevokeServerAccess(ctx context.Context, userID int64, serverID string) error {
	_, err := db.ExecContext(ctx, "DELETE FROM user_server_access WHERE user_id = ? AND server_id = ?", userID, serverID)
	return err
}

func (db *ManagerDB) GetUserServerAccess(ctx context.Context, userID int64) ([]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT server_id FROM user_server_access WHERE user_id = ?", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	serverIDs := make([]string, 0)
	for rows.Next() {
		var sID string
		if err := rows.Scan(&sID); err != nil {
			return nil, err
		}
		serverIDs = append(serverIDs, sID)
	}
	return serverIDs, rows.Err()
}

func (db *ManagerDB) CheckUserServerAccess(ctx context.Context, userID int64, serverID string) (bool, error) {
	var exists int
	err := db.QueryRowContext(ctx,
		"SELECT 1 FROM user_server_access WHERE user_id = ? AND server_id = ?",
		userID, serverID,
	).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// --- Servers ---

func (db *ManagerDB) CreateServer(ctx context.Context, s *models.Server) error {
	now := time.Now().UTC()
	nowStr := FormatTime(now)
	s.CreatedAt = now
	s.UpdatedAt = now

	autostartInt := 0
	if s.AutostartOnBoot {
		autostartInt = 1
	}
	portGateInt := 0
	if s.PortGateEnabled {
		portGateInt = 1
	}

	_, err := db.ExecContext(ctx, `
		INSERT INTO servers (
			id, name, version, port, portv6, status, mode, difficulty,
			autostart_on_boot, port_gate_enabled, port_gate_mode, port_gate_timeout,
			memory_limit, cpu_limit, container_id, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		s.ID, s.Name, s.Version, s.Port, s.PortV6, s.Status, s.Mode, s.Difficulty,
		autostartInt, portGateInt, s.PortGateMode, s.PortGateTimeout,
		s.MemoryLimit, s.CPULimit, s.ContainerID, nowStr, nowStr,
	)
	return err
}

func (db *ManagerDB) GetServer(ctx context.Context, id string) (*models.Server, error) {
	var s models.Server
	var autostartInt, portGateInt int
	var createdAtStr, updatedAtStr string

	err := db.QueryRowContext(ctx, `
		SELECT id, name, version, port, portv6, status, mode, difficulty,
		       autostart_on_boot, port_gate_enabled, port_gate_mode, port_gate_timeout,
		       memory_limit, cpu_limit, container_id, created_at, updated_at
		FROM servers WHERE id = ?`, id,
	).Scan(
		&s.ID, &s.Name, &s.Version, &s.Port, &s.PortV6, &s.Status, &s.Mode, &s.Difficulty,
		&autostartInt, &portGateInt, &s.PortGateMode, &s.PortGateTimeout,
		&s.MemoryLimit, &s.CPULimit, &s.ContainerID, &createdAtStr, &updatedAtStr,
	)
	if err != nil {
		return nil, err
	}
	s.AutostartOnBoot = autostartInt == 1
	s.PortGateEnabled = portGateInt == 1
	if t, err := ParseTime(createdAtStr); err == nil {
		s.CreatedAt = t
	}
	if t, err := ParseTime(updatedAtStr); err == nil {
		s.UpdatedAt = t
	}
	return &s, nil
}

func (db *ManagerDB) ListServers(ctx context.Context) ([]models.Server, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, name, version, port, portv6, status, mode, difficulty,
		       autostart_on_boot, port_gate_enabled, port_gate_mode, port_gate_timeout,
		       memory_limit, cpu_limit, container_id, created_at, updated_at
		FROM servers ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	servers := make([]models.Server, 0)
	for rows.Next() {
		var s models.Server
		var autostartInt, portGateInt int
		var createdAtStr, updatedAtStr string

		if err := rows.Scan(
			&s.ID, &s.Name, &s.Version, &s.Port, &s.PortV6, &s.Status, &s.Mode, &s.Difficulty,
			&autostartInt, &portGateInt, &s.PortGateMode, &s.PortGateTimeout,
			&s.MemoryLimit, &s.CPULimit, &s.ContainerID, &createdAtStr, &updatedAtStr,
		); err != nil {
			return nil, err
		}
		s.AutostartOnBoot = autostartInt == 1
		s.PortGateEnabled = portGateInt == 1
		if t, err := ParseTime(createdAtStr); err == nil {
			s.CreatedAt = t
		}
		if t, err := ParseTime(updatedAtStr); err == nil {
			s.UpdatedAt = t
		}
		servers = append(servers, s)
	}
	return servers, rows.Err()
}

func (db *ManagerDB) UpdateServerStatus(ctx context.Context, id, status, containerID string) error {
	nowStr := FormatTime(time.Now().UTC())
	_, err := db.ExecContext(ctx, `
		UPDATE servers 
		SET status = ?, container_id = ?, updated_at = ?
		WHERE id = ?`,
		status, containerID, nowStr, id,
	)
	return err
}

func (db *ManagerDB) UpdateServer(ctx context.Context, s *models.Server) error {
	nowStr := FormatTime(time.Now().UTC())
	autostartInt := 0
	if s.AutostartOnBoot {
		autostartInt = 1
	}
	portGateInt := 0
	if s.PortGateEnabled {
		portGateInt = 1
	}

	_, err := db.ExecContext(ctx, `
		UPDATE servers SET
			name = ?, version = ?, port = ?, portv6 = ?, mode = ?, difficulty = ?,
			autostart_on_boot = ?, port_gate_enabled = ?, port_gate_mode = ?, port_gate_timeout = ?,
			memory_limit = ?, cpu_limit = ?, updated_at = ?
		WHERE id = ?`,
		s.Name, s.Version, s.Port, s.PortV6, s.Mode, s.Difficulty,
		autostartInt, portGateInt, s.PortGateMode, s.PortGateTimeout,
		s.MemoryLimit, s.CPULimit, nowStr, s.ID,
	)
	return err
}

func (db *ManagerDB) DeleteServer(ctx context.Context, id string) error {
	_, err := db.ExecContext(ctx, "DELETE FROM servers WHERE id = ?", id)
	return err
}

// --- Port Gate Keys ---

func (db *ManagerDB) CreatePortGateKey(ctx context.Context, k *models.PortGateKey) error {
	now := time.Now().UTC()
	nowStr := FormatTime(now)
	k.CreatedAt = now

	isActiveInt := 0
	if k.IsActive {
		isActiveInt = 1
	}

	res, err := db.ExecContext(ctx, `
		INSERT INTO port_gate_keys (
			server_id, label, key_hash, key_prefix, max_uses, used_count,
			lease_duration_seconds, expires_at, is_active, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		k.ServerID, k.Label, k.KeyHash, k.KeyPrefix, k.MaxUses, k.UsedCount,
		k.LeaseDurationSeconds, FormatNullTime(k.ExpiresAt), isActiveInt, nowStr,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	k.ID = id
	return nil
}

func (db *ManagerDB) ListPortGateKeys(ctx context.Context, serverID *string) ([]models.PortGateKey, error) {
	var query string
	var args []interface{}

	if serverID != nil {
		query = `
			SELECT id, server_id, label, key_hash, key_prefix, max_uses, used_count,
			       lease_duration_seconds, expires_at, is_active, created_at
			FROM port_gate_keys WHERE server_id = ? OR server_id IS NULL ORDER BY id DESC`
		args = append(args, *serverID)
	} else {
		query = `
			SELECT id, server_id, label, key_hash, key_prefix, max_uses, used_count,
			       lease_duration_seconds, expires_at, is_active, created_at
			FROM port_gate_keys ORDER BY id DESC`
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	keys := make([]models.PortGateKey, 0)
	for rows.Next() {
		var k models.PortGateKey
		var sID sql.NullString
		var expiresAtStr sql.NullString
		var createdAtStr string
		var isActiveInt int

		if err := rows.Scan(
			&k.ID, &sID, &k.Label, &k.KeyHash, &k.KeyPrefix, &k.MaxUses, &k.UsedCount,
			&k.LeaseDurationSeconds, &expiresAtStr, &isActiveInt, &createdAtStr,
		); err != nil {
			return nil, err
		}

		if sID.Valid {
			k.ServerID = &sID.String
		}
		k.ExpiresAt = ParseNullTime(expiresAtStr)
		k.IsActive = isActiveInt == 1
		if t, err := ParseTime(createdAtStr); err == nil {
			k.CreatedAt = t
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (db *ManagerDB) IncrementKeyUsage(ctx context.Context, keyID int64) error {
	_, err := db.ExecContext(ctx, "UPDATE port_gate_keys SET used_count = used_count + 1 WHERE id = ?", keyID)
	return err
}

func (db *ManagerDB) DeletePortGateKey(ctx context.Context, id int64) error {
	_, err := db.ExecContext(ctx, "DELETE FROM port_gate_keys WHERE id = ?", id)
	return err
}

// --- Port Gate Leases ---

func (db *ManagerDB) CreatePortGateLease(ctx context.Context, l *models.PortGateLease) error {
	grantedAtStr := FormatTime(l.GrantedAt)
	expiresAtStr := FormatTime(l.ExpiresAt)

	res, err := db.ExecContext(ctx, `
		INSERT INTO port_gate_leases (
			server_id, key_id, ip_address, gamertag, knock_method,
			session_token_hash, granted_at, expires_at, comment, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		l.ServerID, l.KeyID, l.IPAddress, l.Gamertag, l.KnockMethod,
		l.SessionTokenHash, grantedAtStr, expiresAtStr, l.Comment, l.Status,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	l.ID = id
	return nil
}

func (db *ManagerDB) GetActiveLeaseByIP(ctx context.Context, serverID, ip string) (*models.PortGateLease, error) {
	nowStr := FormatTime(time.Now().UTC())
	var l models.PortGateLease
	var keyID sql.NullInt64
	var grantedAtStr, expiresAtStr string

	err := db.QueryRowContext(ctx, `
		SELECT id, server_id, key_id, ip_address, gamertag, knock_method,
		       session_token_hash, granted_at, expires_at, comment, status
		FROM port_gate_leases
		WHERE server_id = ? AND ip_address = ? AND status = 'active' AND expires_at > ?
		ORDER BY id DESC LIMIT 1`,
		serverID, ip, nowStr,
	).Scan(
		&l.ID, &l.ServerID, &keyID, &l.IPAddress, &l.Gamertag, &l.KnockMethod,
		&l.SessionTokenHash, &grantedAtStr, &expiresAtStr, &l.Comment, &l.Status,
	)
	if err != nil {
		return nil, err
	}
	if keyID.Valid {
		l.KeyID = &keyID.Int64
	}
	if t, err := ParseTime(grantedAtStr); err == nil {
		l.GrantedAt = t
	}
	if t, err := ParseTime(expiresAtStr); err == nil {
		l.ExpiresAt = t
	}
	return &l, nil
}

func (db *ManagerDB) GetActiveLeaseBySessionToken(ctx context.Context, serverID, tokenHash string) (*models.PortGateLease, error) {
	nowStr := FormatTime(time.Now().UTC())
	var l models.PortGateLease
	var keyID sql.NullInt64
	var grantedAtStr, expiresAtStr string

	err := db.QueryRowContext(ctx, `
		SELECT id, server_id, key_id, ip_address, gamertag, knock_method,
		       session_token_hash, granted_at, expires_at, comment, status
		FROM port_gate_leases
		WHERE server_id = ? AND session_token_hash = ? AND status = 'active' AND expires_at > ?
		ORDER BY id DESC LIMIT 1`,
		serverID, tokenHash, nowStr,
	).Scan(
		&l.ID, &l.ServerID, &keyID, &l.IPAddress, &l.Gamertag, &l.KnockMethod,
		&l.SessionTokenHash, &grantedAtStr, &expiresAtStr, &l.Comment, &l.Status,
	)
	if err != nil {
		return nil, err
	}
	if keyID.Valid {
		l.KeyID = &keyID.Int64
	}
	if t, err := ParseTime(grantedAtStr); err == nil {
		l.GrantedAt = t
	}
	if t, err := ParseTime(expiresAtStr); err == nil {
		l.ExpiresAt = t
	}
	return &l, nil
}

func (db *ManagerDB) ListActiveLeases(ctx context.Context, serverID string) ([]models.PortGateLease, error) {
	nowStr := FormatTime(time.Now().UTC())
	rows, err := db.QueryContext(ctx, `
		SELECT id, server_id, key_id, ip_address, gamertag, knock_method,
		       session_token_hash, granted_at, expires_at, comment, status
		FROM port_gate_leases
		WHERE server_id = ? AND status = 'active' AND expires_at > ?
		ORDER BY id DESC`,
		serverID, nowStr,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	leases := make([]models.PortGateLease, 0)
	for rows.Next() {
		var l models.PortGateLease
		var keyID sql.NullInt64
		var grantedAtStr, expiresAtStr string

		if err := rows.Scan(
			&l.ID, &l.ServerID, &keyID, &l.IPAddress, &l.Gamertag, &l.KnockMethod,
			&l.SessionTokenHash, &grantedAtStr, &expiresAtStr, &l.Comment, &l.Status,
		); err != nil {
			return nil, err
		}
		if keyID.Valid {
			l.KeyID = &keyID.Int64
		}
		if t, err := ParseTime(grantedAtStr); err == nil {
			l.GrantedAt = t
		}
		if t, err := ParseTime(expiresAtStr); err == nil {
			l.ExpiresAt = t
		}
		leases = append(leases, l)
	}
	return leases, rows.Err()
}

func (db *ManagerDB) UpdateLeaseIP(ctx context.Context, leaseID int64, newIP string) error {
	_, err := db.ExecContext(ctx, "UPDATE port_gate_leases SET ip_address = ? WHERE id = ?", newIP, leaseID)
	return err
}

func (db *ManagerDB) RevokeLease(ctx context.Context, leaseID int64) error {
	_, err := db.ExecContext(ctx, "UPDATE port_gate_leases SET status = 'revoked' WHERE id = ?", leaseID)
	return err
}

func (db *ManagerDB) GetLease(ctx context.Context, id int64) (*models.PortGateLease, error) {
	var l models.PortGateLease
	var keyID sql.NullInt64
	var grantedAtStr, expiresAtStr string

	err := db.QueryRowContext(ctx, `
		SELECT id, server_id, key_id, ip_address, gamertag, knock_method,
		       session_token_hash, granted_at, expires_at, comment, status
		FROM port_gate_leases
		WHERE id = ?`, id,
	).Scan(
		&l.ID, &l.ServerID, &keyID, &l.IPAddress, &l.Gamertag, &l.KnockMethod,
		&l.SessionTokenHash, &grantedAtStr, &expiresAtStr, &l.Comment, &l.Status,
	)
	if err != nil {
		return nil, err
	}
	if keyID.Valid {
		l.KeyID = &keyID.Int64
	}
	if t, err := ParseTime(grantedAtStr); err == nil {
		l.GrantedAt = t
	}
	if t, err := ParseTime(expiresAtStr); err == nil {
		l.ExpiresAt = t
	}
	return &l, nil
}

func (db *ManagerDB) SetLeaseStatus(ctx context.Context, leaseID int64, status string) error {
	_, err := db.ExecContext(ctx, "UPDATE port_gate_leases SET status = ? WHERE id = ?", status, leaseID)
	return err
}

func (db *ManagerDB) GetExpiredActiveLeases(ctx context.Context) ([]models.PortGateLease, error) {
	nowStr := FormatTime(time.Now().UTC())
	rows, err := db.QueryContext(ctx, `
		SELECT id, server_id, key_id, ip_address, gamertag, knock_method,
		       session_token_hash, granted_at, expires_at, comment, status
		FROM port_gate_leases
		WHERE status = 'active' AND expires_at <= ?
		ORDER BY id ASC`, nowStr,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	leases := make([]models.PortGateLease, 0)
	for rows.Next() {
		var l models.PortGateLease
		var keyID sql.NullInt64
		var grantedAtStr, expiresAtStr string

		if err := rows.Scan(
			&l.ID, &l.ServerID, &keyID, &l.IPAddress, &l.Gamertag, &l.KnockMethod,
			&l.SessionTokenHash, &grantedAtStr, &expiresAtStr, &l.Comment, &l.Status,
		); err != nil {
			return nil, err
		}
		if keyID.Valid {
			l.KeyID = &keyID.Int64
		}
		if t, err := ParseTime(grantedAtStr); err == nil {
			l.GrantedAt = t
		}
		if t, err := ParseTime(expiresAtStr); err == nil {
			l.ExpiresAt = t
		}
		leases = append(leases, l)
	}
	return leases, rows.Err()
}

func (db *ManagerDB) ExpireOldLeases(ctx context.Context) (int64, error) {
	nowStr := FormatTime(time.Now().UTC())
	res, err := db.ExecContext(ctx, "UPDATE port_gate_leases SET status = 'expired' WHERE status = 'active' AND expires_at <= ?", nowStr)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// --- Audit Logs ---

func (db *ManagerDB) CreateAuditLog(ctx context.Context, log *models.AuditLog) error {
	now := time.Now().UTC()
	nowStr := FormatTime(now)
	log.Timestamp = now

	res, err := db.ExecContext(ctx, `
		INSERT INTO audit_logs (user_id, actor_type, actor_name, action, target, details, client_ip, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		log.UserID, log.ActorType, log.ActorName, log.Action, log.Target, log.Details, log.ClientIP, nowStr,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	log.ID = id
	return nil
}

func (db *ManagerDB) ListAuditLogs(ctx context.Context, limit, offset int) ([]models.AuditLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := db.QueryContext(ctx, `
		SELECT id, user_id, actor_type, actor_name, action, target, details, client_ip, timestamp
		FROM audit_logs ORDER BY id DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := make([]models.AuditLog, 0)
	for rows.Next() {
		var l models.AuditLog
		var uID sql.NullInt64
		var tsStr string

		if err := rows.Scan(
			&l.ID, &uID, &l.ActorType, &l.ActorName, &l.Action, &l.Target, &l.Details, &l.ClientIP, &tsStr,
		); err != nil {
			return nil, err
		}
		if uID.Valid {
			l.UserID = &uID.Int64
		}
		if t, err := ParseTime(tsStr); err == nil {
			l.Timestamp = t
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

// --- System Settings ---

func (db *ManagerDB) GetSetting(ctx context.Context, key string) (string, error) {
	var val string
	err := db.QueryRowContext(ctx, "SELECT value FROM system_settings WHERE key = ?", key).Scan(&val)
	if err != nil {
		return "", err
	}
	return val, nil
}

func (db *ManagerDB) SetSetting(ctx context.Context, key, val string) error {
	_, err := db.ExecContext(ctx, "INSERT INTO system_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", key, val)
	return err
}

func (db *ManagerDB) GetAllSettings(ctx context.Context) (map[string]string, error) {
	rows, err := db.QueryContext(ctx, "SELECT key, value FROM system_settings")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	settings := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		settings[k] = v
	}
	return settings, rows.Err()
}

// --- Backups & Tasks stubs for Phase 1 ---

func (db *ManagerDB) CreateBackup(ctx context.Context, b *models.Backup) error {
	now := time.Now().UTC()
	b.CreatedAt = now
	isLockedInt := 0
	if b.IsLocked {
		isLockedInt = 1
	}

	res, err := db.ExecContext(ctx, `
		INSERT INTO backups (server_id, filename, size_bytes, type, is_locked, status, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		b.ServerID, b.Filename, b.SizeBytes, b.Type, isLockedInt, b.Status, FormatTime(now),
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	b.ID = id
	return nil
}

func (db *ManagerDB) ListBackups(ctx context.Context, serverID string) ([]models.Backup, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, server_id, filename, size_bytes, type, is_locked, status, created_at
		FROM backups WHERE server_id = ? ORDER BY id DESC`, serverID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	backups := make([]models.Backup, 0)
	for rows.Next() {
		var b models.Backup
		var isLockedInt int
		var createdAtStr string

		if err := rows.Scan(
			&b.ID, &b.ServerID, &b.Filename, &b.SizeBytes, &b.Type, &isLockedInt, &b.Status, &createdAtStr,
		); err != nil {
			return nil, err
		}
		b.IsLocked = isLockedInt == 1
		if t, err := ParseTime(createdAtStr); err == nil {
			b.CreatedAt = t
		}
		backups = append(backups, b)
	}
	return backups, rows.Err()
}

func (db *ManagerDB) GetBackup(ctx context.Context, id int64) (*models.Backup, error) {
	var b models.Backup
	var isLockedInt int
	var createdAtStr string

	err := db.QueryRowContext(ctx, `
		SELECT id, server_id, filename, size_bytes, type, is_locked, status, created_at
		FROM backups WHERE id = ?`, id,
	).Scan(
		&b.ID, &b.ServerID, &b.Filename, &b.SizeBytes, &b.Type, &isLockedInt, &b.Status, &createdAtStr,
	)
	if err != nil {
		return nil, err
	}
	b.IsLocked = isLockedInt == 1
	if t, err := ParseTime(createdAtStr); err == nil {
		b.CreatedAt = t
	}
	return &b, nil
}

func (db *ManagerDB) DeleteBackup(ctx context.Context, id int64) error {
	_, err := db.ExecContext(ctx, "DELETE FROM backups WHERE id = ?", id)
	return err
}

func (db *ManagerDB) ToggleBackupLock(ctx context.Context, id int64, isLocked bool) error {
	lockedInt := 0
	if isLocked {
		lockedInt = 1
	}
	_, err := db.ExecContext(ctx, "UPDATE backups SET is_locked = ? WHERE id = ?", lockedInt, id)
	return err
}

func (db *ManagerDB) GetUnpinnedBackups(ctx context.Context, serverID string) ([]models.Backup, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, server_id, filename, size_bytes, type, is_locked, status, created_at
		FROM backups WHERE server_id = ? AND is_locked = 0 ORDER BY id ASC`, serverID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	backups := make([]models.Backup, 0)
	for rows.Next() {
		var b models.Backup
		var isLockedInt int
		var createdAtStr string

		if err := rows.Scan(
			&b.ID, &b.ServerID, &b.Filename, &b.SizeBytes, &b.Type, &isLockedInt, &b.Status, &createdAtStr,
		); err != nil {
			return nil, err
		}
		b.IsLocked = isLockedInt == 1
		if t, err := ParseTime(createdAtStr); err == nil {
			b.CreatedAt = t
		}
		backups = append(backups, b)
	}
	return backups, rows.Err()
}

// --- Task CRUD ---

func (db *ManagerDB) CreateTask(ctx context.Context, t *models.Task) error {
	now := time.Now().UTC()
	t.CreatedAt = now
	enabledInt := 0
	if t.Enabled {
		enabledInt = 1
	}

	res, err := db.ExecContext(ctx, `
		INSERT INTO tasks (server_id, name, cron_expr, action, payload, last_run, next_run, enabled, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ServerID, t.Name, t.CronExpr, t.Action, t.Payload,
		FormatNullTime(t.LastRun), FormatNullTime(t.NextRun), enabledInt, FormatTime(now),
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	t.ID = id
	return nil
}

func (db *ManagerDB) ListTasks(ctx context.Context, serverID *string) ([]models.Task, error) {
	var rows *sql.Rows
	var err error
	if serverID != nil {
		rows, err = db.QueryContext(ctx, `
			SELECT id, server_id, name, cron_expr, action, payload, last_run, next_run, enabled, created_at
			FROM tasks WHERE server_id = ? OR server_id IS NULL ORDER BY id DESC`, *serverID,
		)
	} else {
		rows, err = db.QueryContext(ctx, `
			SELECT id, server_id, name, cron_expr, action, payload, last_run, next_run, enabled, created_at
			FROM tasks ORDER BY id DESC`,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]models.Task, 0)
	for rows.Next() {
		var t models.Task
		var srvID sql.NullString
		var lastRunStr, nextRunStr, createdAtStr sql.NullString
		var enabledInt int

		if err := rows.Scan(
			&t.ID, &srvID, &t.Name, &t.CronExpr, &t.Action, &t.Payload,
			&lastRunStr, &nextRunStr, &enabledInt, &createdAtStr,
		); err != nil {
			return nil, err
		}
		if srvID.Valid {
			t.ServerID = &srvID.String
		}
		if lastRunStr.Valid {
			if tm, err := ParseTime(lastRunStr.String); err == nil {
				t.LastRun = &tm
			}
		}
		if nextRunStr.Valid {
			if tm, err := ParseTime(nextRunStr.String); err == nil {
				t.NextRun = &tm
			}
		}
		t.Enabled = enabledInt == 1
		if createdAtStr.Valid {
			if tm, err := ParseTime(createdAtStr.String); err == nil {
				t.CreatedAt = tm
			}
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (db *ManagerDB) GetTask(ctx context.Context, id int64) (*models.Task, error) {
	var t models.Task
	var srvID sql.NullString
	var lastRunStr, nextRunStr, createdAtStr sql.NullString
	var enabledInt int

	err := db.QueryRowContext(ctx, `
		SELECT id, server_id, name, cron_expr, action, payload, last_run, next_run, enabled, created_at
		FROM tasks WHERE id = ?`, id,
	).Scan(
		&t.ID, &srvID, &t.Name, &t.CronExpr, &t.Action, &t.Payload,
		&lastRunStr, &nextRunStr, &enabledInt, &createdAtStr,
	)
	if err != nil {
		return nil, err
	}
	if srvID.Valid {
		t.ServerID = &srvID.String
	}
	if lastRunStr.Valid {
		if tm, err := ParseTime(lastRunStr.String); err == nil {
			t.LastRun = &tm
		}
	}
	if nextRunStr.Valid {
		if tm, err := ParseTime(nextRunStr.String); err == nil {
			t.NextRun = &tm
		}
	}
	t.Enabled = enabledInt == 1
	if createdAtStr.Valid {
		if tm, err := ParseTime(createdAtStr.String); err == nil {
			t.CreatedAt = tm
		}
	}
	return &t, nil
}

func (db *ManagerDB) UpdateTask(ctx context.Context, t *models.Task) error {
	enabledInt := 0
	if t.Enabled {
		enabledInt = 1
	}
	_, err := db.ExecContext(ctx, `
		UPDATE tasks
		SET server_id = ?, name = ?, cron_expr = ?, action = ?, payload = ?, enabled = ?
		WHERE id = ?`,
		t.ServerID, t.Name, t.CronExpr, t.Action, t.Payload, enabledInt, t.ID,
	)
	return err
}

func (db *ManagerDB) DeleteTask(ctx context.Context, id int64) error {
	_, err := db.ExecContext(ctx, "DELETE FROM tasks WHERE id = ?", id)
	return err
}

func (db *ManagerDB) ToggleTask(ctx context.Context, id int64, enabled bool) error {
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}
	_, err := db.ExecContext(ctx, "UPDATE tasks SET enabled = ? WHERE id = ?", enabledInt, id)
	return err
}

func (db *ManagerDB) UpdateTaskRun(ctx context.Context, id int64, lastRun time.Time, nextRun *time.Time) error {
	_, err := db.ExecContext(ctx, `
		UPDATE tasks
		SET last_run = ?, next_run = ?
		WHERE id = ?`,
		FormatTime(lastRun), FormatNullTime(nextRun), id,
	)
	return err
}

// --- Global Player Access Operations ---

func (db *ManagerDB) ListGlobalPlayers(ctx context.Context) ([]*models.GlobalPlayer, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, name, xuid, is_allowlisted, permission, ignores_player_limit, created_at, updated_at
		FROM global_players
		ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	players := make([]*models.GlobalPlayer, 0)
	for rows.Next() {
		var (
			p               models.GlobalPlayer
			allowlistedInt  int
			ignoresLimitInt int
			createdAtStr    string
			updatedAtStr    string
		)
		if err := rows.Scan(&p.ID, &p.Name, &p.XUID, &allowlistedInt, &p.Permission, &ignoresLimitInt, &createdAtStr, &updatedAtStr); err != nil {
			return nil, err
		}
		p.IsAllowlisted = allowlistedInt == 1
		p.IgnoresPlayerLimit = ignoresLimitInt == 1
		p.CreatedAt, _ = ParseTime(createdAtStr)
		p.UpdatedAt, _ = ParseTime(updatedAtStr)
		players = append(players, &p)
	}
	return players, rows.Err()
}

func (db *ManagerDB) GetGlobalPlayer(ctx context.Context, id int64) (*models.GlobalPlayer, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, name, xuid, is_allowlisted, permission, ignores_player_limit, created_at, updated_at
		FROM global_players
		WHERE id = ?`, id)

	var (
		p               models.GlobalPlayer
		allowlistedInt  int
		ignoresLimitInt int
		createdAtStr    string
		updatedAtStr    string
	)
	if err := row.Scan(&p.ID, &p.Name, &p.XUID, &allowlistedInt, &p.Permission, &ignoresLimitInt, &createdAtStr, &updatedAtStr); err != nil {
		return nil, err
	}
	p.IsAllowlisted = allowlistedInt == 1
	p.IgnoresPlayerLimit = ignoresLimitInt == 1
	p.CreatedAt, _ = ParseTime(createdAtStr)
	p.UpdatedAt, _ = ParseTime(updatedAtStr)
	return &p, nil
}

func (db *ManagerDB) GetGlobalPlayerByName(ctx context.Context, name string) (*models.GlobalPlayer, error) {
	row := db.QueryRowContext(ctx, `
		SELECT id, name, xuid, is_allowlisted, permission, ignores_player_limit, created_at, updated_at
		FROM global_players
		WHERE LOWER(name) = LOWER(?)`, name)

	var (
		p               models.GlobalPlayer
		allowlistedInt  int
		ignoresLimitInt int
		createdAtStr    string
		updatedAtStr    string
	)
	if err := row.Scan(&p.ID, &p.Name, &p.XUID, &allowlistedInt, &p.Permission, &ignoresLimitInt, &createdAtStr, &updatedAtStr); err != nil {
		return nil, err
	}
	p.IsAllowlisted = allowlistedInt == 1
	p.IgnoresPlayerLimit = ignoresLimitInt == 1
	p.CreatedAt, _ = ParseTime(createdAtStr)
	p.UpdatedAt, _ = ParseTime(updatedAtStr)
	return &p, nil
}

func (db *ManagerDB) UpsertGlobalPlayer(ctx context.Context, gp *models.GlobalPlayer) (*models.GlobalPlayer, error) {
	now := time.Now().UTC()
	if gp.CreatedAt.IsZero() {
		gp.CreatedAt = now
	}
	gp.UpdatedAt = now

	allowlistedInt := 0
	if gp.IsAllowlisted {
		allowlistedInt = 1
	}
	ignoresLimitInt := 0
	if gp.IgnoresPlayerLimit {
		ignoresLimitInt = 1
	}
	if gp.Permission == "" {
		gp.Permission = "member"
	}

	query := `
		INSERT INTO global_players (name, xuid, is_allowlisted, permission, ignores_player_limit, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(name) DO UPDATE SET
			xuid = CASE WHEN excluded.xuid != '' THEN excluded.xuid ELSE global_players.xuid END,
			is_allowlisted = excluded.is_allowlisted,
			permission = excluded.permission,
			ignores_player_limit = excluded.ignores_player_limit,
			updated_at = excluded.updated_at
		RETURNING id, created_at, updated_at`

	var createdAtStr, updatedAtStr string
	err := db.QueryRowContext(ctx, query,
		gp.Name,
		gp.XUID,
		allowlistedInt,
		gp.Permission,
		ignoresLimitInt,
		FormatTime(gp.CreatedAt),
		FormatTime(gp.UpdatedAt),
	).Scan(&gp.ID, &createdAtStr, &updatedAtStr)
	if err != nil {
		return nil, err
	}

	gp.CreatedAt, _ = ParseTime(createdAtStr)
	gp.UpdatedAt, _ = ParseTime(updatedAtStr)
	return gp, nil
}

func (db *ManagerDB) UpdateGlobalPlayer(ctx context.Context, gp *models.GlobalPlayer) (*models.GlobalPlayer, error) {
	now := time.Now().UTC()
	gp.UpdatedAt = now

	allowlistedInt := 0
	if gp.IsAllowlisted {
		allowlistedInt = 1
	}
	ignoresLimitInt := 0
	if gp.IgnoresPlayerLimit {
		ignoresLimitInt = 1
	}
	if gp.Permission == "" {
		gp.Permission = "member"
	}

	_, err := db.ExecContext(ctx, `
		UPDATE global_players
		SET name = ?, xuid = ?, is_allowlisted = ?, permission = ?, ignores_player_limit = ?, updated_at = ?
		WHERE id = ?`,
		gp.Name, gp.XUID, allowlistedInt, gp.Permission, ignoresLimitInt, FormatTime(gp.UpdatedAt), gp.ID)
	if err != nil {
		return nil, err
	}
	return gp, nil
}

func (db *ManagerDB) DeleteGlobalPlayer(ctx context.Context, id int64) error {
	_, err := db.ExecContext(ctx, "DELETE FROM global_players WHERE id = ?", id)
	return err
}

func (db *ManagerDB) DeleteGlobalPlayerByName(ctx context.Context, nameOrXuid string) error {
	_, err := db.ExecContext(ctx, "DELETE FROM global_players WHERE LOWER(name) = LOWER(?) OR (xuid != '' AND xuid = ?)", nameOrXuid, nameOrXuid)
	return err
}

// --- MetricsDB Operations ---

func (db *MetricsDB) InsertRawBatch(ctx context.Context, samples []models.MetricRaw) error {
	if len(samples) == 0 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO metrics_raw (server_id, timestamp, cpu_percent, ram_bytes, player_count)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, s := range samples {
		if _, err := stmt.ExecContext(ctx, s.ServerID, FormatTime(s.Timestamp), s.CPUPercent, s.RAMBytes, s.PlayerCount); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (db *MetricsDB) QueryRaw(ctx context.Context, serverID string, since time.Time) ([]models.MetricRaw, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, server_id, timestamp, cpu_percent, ram_bytes, player_count
		FROM metrics_raw WHERE server_id = ? AND timestamp >= ?
		ORDER BY timestamp ASC`, serverID, FormatTime(since),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []models.MetricRaw
	for rows.Next() {
		var m models.MetricRaw
		var tsStr string
		if err := rows.Scan(&m.ID, &m.ServerID, &tsStr, &m.CPUPercent, &m.RAMBytes, &m.PlayerCount); err != nil {
			return nil, err
		}
		if t, err := ParseTime(tsStr); err == nil {
			m.Timestamp = t
		}
		list = append(list, m)
	}
	return list, rows.Err()
}

func (db *MetricsDB) Rollup5Min(ctx context.Context, before time.Time) error {
	beforeStr := FormatTime(before)
	_, err := db.ExecContext(ctx, `
		INSERT INTO metrics_5m (server_id, timestamp, avg_cpu, max_cpu, min_cpu, avg_ram, max_ram, min_ram, avg_players, max_players, sample_count)
		SELECT 
			server_id,
			strftime('%Y-%m-%dT%H:%M:00Z', timestamp) as ts_bucket,
			AVG(cpu_percent),
			MAX(cpu_percent),
			MIN(cpu_percent),
			CAST(AVG(ram_bytes) AS INTEGER),
			MAX(ram_bytes),
			MIN(ram_bytes),
			AVG(player_count),
			MAX(player_count),
			COUNT(*)
		FROM metrics_raw
		WHERE timestamp <= ?
		GROUP BY server_id, strftime('%Y-%m-%dT%H:', timestamp), (strftime('%M', timestamp) / 5)
	`, beforeStr)
	return err
}

func (db *MetricsDB) Rollup1Hour(ctx context.Context, before time.Time) error {
	beforeStr := FormatTime(before)
	_, err := db.ExecContext(ctx, `
		INSERT INTO metrics_1h (server_id, timestamp, avg_cpu, max_cpu, min_cpu, avg_ram, max_ram, min_ram, avg_players, max_players, sample_count)
		SELECT 
			server_id,
			strftime('%Y-%m-%dT%H:00:00Z', timestamp) as ts_bucket,
			AVG(avg_cpu),
			MAX(max_cpu),
			MIN(min_cpu),
			CAST(AVG(avg_ram) AS INTEGER),
			MAX(max_ram),
			MIN(min_ram),
			AVG(avg_players),
			MAX(max_players),
			SUM(sample_count)
		FROM metrics_5m
		WHERE timestamp <= ?
		GROUP BY server_id, strftime('%Y-%m-%dT%H:00:00Z', timestamp)
	`, beforeStr)
	return err
}

func (db *MetricsDB) PruneOldMetrics(ctx context.Context, rawRetainHours, rollup5mDays, rollup1hDays int) error {
	now := time.Now().UTC()
	rawCutoff := FormatTime(now.Add(-time.Duration(rawRetainHours) * time.Hour))
	m5Cutoff := FormatTime(now.AddDate(0, 0, -rollup5mDays))
	h1Cutoff := FormatTime(now.AddDate(0, 0, -rollup1hDays))

	if _, err := db.ExecContext(ctx, "DELETE FROM metrics_raw WHERE timestamp < ?", rawCutoff); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "DELETE FROM metrics_5m WHERE timestamp < ?", m5Cutoff); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, "DELETE FROM metrics_1h WHERE timestamp < ?", h1Cutoff); err != nil {
		return err
	}
	return nil
}
