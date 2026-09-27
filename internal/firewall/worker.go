package firewall

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

// LeaseAuditor periodically audits active port gate leases and revokes expired firewall rules.
type LeaseAuditor struct {
	db       *database.ManagerDB
	driver   FirewallDriver
	interval time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewLeaseAuditor initializes a new background lease auditor.
func NewLeaseAuditor(db *database.ManagerDB, driver FirewallDriver, interval time.Duration) *LeaseAuditor {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	return &LeaseAuditor{
		db:       db,
		driver:   driver,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start launches the background auditing loop.
func (a *LeaseAuditor) Start(ctx context.Context) {
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		ticker := time.NewTicker(a.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-a.stopCh:
				return
			case <-ticker.C:
				if count, err := a.AuditOnce(ctx); err != nil {
					log.Printf("[LeaseAuditor] Error auditing expired leases: %v", err)
				} else if count > 0 {
					log.Printf("[LeaseAuditor] Expired and revoked %d port gate leases", count)
				}
			}
		}
	}()
}

// Stop cleanly stops the background auditor.
func (a *LeaseAuditor) Stop() {
	select {
	case <-a.stopCh:
		return
	default:
		close(a.stopCh)
	}
	a.wg.Wait()
}

// AuditOnce runs a single audit pass, revoking firewall rules for expired leases.
func (a *LeaseAuditor) AuditOnce(ctx context.Context) (int, error) {
	expiredLeases, err := a.db.GetExpiredActiveLeases(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to query expired leases: %w", err)
	}

	revokedCount := 0
	for _, lease := range expiredLeases {
		server, err := a.db.GetServer(ctx, lease.ServerID)
		if err != nil {
			// If server deleted, still mark lease expired
			_ = a.db.SetLeaseStatus(ctx, lease.ID, "expired")
			continue
		}

		comment := fmt.Sprintf("bsm_%s_%d", server.ID, lease.ID)

		// Revoke IPv4/base port
		_ = a.driver.RevokePort(ctx, lease.IPAddress, server.Port, comment)

		// Revoke IPv6 port if defined
		if server.PortV6 > 0 {
			_ = a.driver.RevokePort(ctx, lease.IPAddress, server.PortV6, comment)
		}

		// Update DB status to expired
		if err := a.db.SetLeaseStatus(ctx, lease.ID, "expired"); err == nil {
			revokedCount++
			_ = a.db.CreateAuditLog(ctx, &models.AuditLog{
				ActorType: "system",
				ActorName: "lease_auditor",
				Action:    "knock_lease_expired",
				Target:    server.ID,
				Details:   fmt.Sprintf(`{"lease_id": %d, "ip": "%s", "gamertag": "%s"}`, lease.ID, lease.IPAddress, lease.Gamertag),
				ClientIP:  lease.IPAddress,
			})
		}
	}

	return revokedCount, nil
}

// SyncPermanentAllowRules ensures all permanent allow rules from database are applied to the firewall.
func SyncPermanentAllowRules(ctx context.Context, db *database.ManagerDB, driver FirewallDriver) error {
	if driver == nil || db == nil {
		return nil
	}
	rules, err := db.ListPortGateAllowRules(ctx, nil)
	if err != nil {
		return err
	}
	servers, err := db.ListServers(ctx)
	if err != nil {
		return err
	}

	for _, rule := range rules {
		comment := fmt.Sprintf("bsm_perm_%d", rule.ID)
		for _, s := range servers {
			if rule.ServerID == nil || *rule.ServerID == "" || *rule.ServerID == s.ID {
				_ = driver.AllowPort(ctx, rule.IPOrSubnet, s.Port, comment)
				if s.PortV6 > 0 {
					_ = driver.AllowPort(ctx, rule.IPOrSubnet, s.PortV6, comment)
				}
			}
		}
	}
	return nil
}
