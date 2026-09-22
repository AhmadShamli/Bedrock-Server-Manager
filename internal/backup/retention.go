package backup

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

// RetentionPolicy controls automated pruning of unpinned backups.
type RetentionPolicy struct {
	MaxCount     int           `json:"max_count"`      // Keep at most N unpinned backups (default: 10)
	MaxAge       time.Duration `json:"max_age"`        // Max age for unpinned backups (default: 14 days)
	MaxDiskBytes int64         `json:"max_disk_bytes"` // Max total disk bytes for unpinned backups (default: 5GB)
}

// DefaultRetentionPolicy returns standard sensible defaults.
func DefaultRetentionPolicy() RetentionPolicy {
	return RetentionPolicy{
		MaxCount:     10,
		MaxAge:       14 * 24 * time.Hour,
		MaxDiskBytes: 5 * 1024 * 1024 * 1024, // 5 GB
	}
}

// EnforceRetention prunes expired or excess unpinned backups according to policy.
func EnforceRetention(ctx context.Context, serverID string, policy RetentionPolicy, backupDir string, mgrDB *database.ManagerDB) ([]int64, error) {
	unpinned, err := mgrDB.GetUnpinnedBackups(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to list unpinned backups: %w", err)
	}

	if len(unpinned) == 0 {
		return nil, nil
	}

	now := time.Now().UTC()
	var toDelete []models.Backup
	var remaining []models.Backup

	// 1. Age-based pruning
	for _, b := range unpinned {
		if policy.MaxAge > 0 && now.Sub(b.CreatedAt) > policy.MaxAge {
			toDelete = append(toDelete, b)
		} else {
			remaining = append(remaining, b)
		}
	}

	// 2. Count-based pruning (retain the newest MaxCount)
	if policy.MaxCount > 0 && len(remaining) > policy.MaxCount {
		excess := len(remaining) - policy.MaxCount
		toDelete = append(toDelete, remaining[:excess]...)
		remaining = remaining[excess:]
	}

	// 3. Disk quota pruning (oldest first until total is within MaxDiskBytes)
	if policy.MaxDiskBytes > 0 {
		var totalBytes int64
		for _, b := range remaining {
			totalBytes += b.SizeBytes
		}

		for len(remaining) > 0 && totalBytes > policy.MaxDiskBytes {
			pruned := remaining[0]
			toDelete = append(toDelete, pruned)
			totalBytes -= pruned.SizeBytes
			remaining = remaining[1:]
		}
	}

	var deletedIDs []int64
	for _, b := range toDelete {
		filePath := filepath.Join(backupDir, b.Filename)
		_ = os.Remove(filePath)
		if err := mgrDB.DeleteBackup(ctx, b.ID); err == nil {
			deletedIDs = append(deletedIDs, b.ID)
			log.Printf("[Retention] Pruned unpinned backup #%d (%s, %d bytes)", b.ID, b.Filename, b.SizeBytes)
			_ = mgrDB.CreateAuditLog(ctx, &models.AuditLog{
				ActorType: "system",
				ActorName: "backup_retention",
				Action:    "backup_pruned",
				Target:    serverID,
				Details:   fmt.Sprintf(`{"backup_id": %d, "filename": "%s"}`, b.ID, b.Filename),
			})
		}
	}

	return deletedIDs, nil
}
