package scheduler

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

func TestTaskSchedulerFlow(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	db, err := database.OpenManagerDB(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("OpenManagerDB failed: %v", err)
	}
	defer db.Close()

	srv := &models.Server{
		ID:     "srv-sched",
		Name:   "Scheduler Server",
		Status: models.ServerStatusStopped,
	}
	_ = db.CreateServer(ctx, srv)

	mockEng := engine.NewMockEngine()
	sched := NewTaskScheduler(db, mockEng, tmpDir)

	task := &models.Task{
		ServerID: &srv.ID,
		Name:     "Daily Backup",
		CronExpr: "0 4 * * *", // 4:00 AM daily
		Action:   "backup",
		Enabled:  true,
	}
	if err := db.CreateTask(ctx, task); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	if err := sched.Start(ctx); err != nil {
		t.Fatalf("scheduler start failed: %v", err)
	}
	defer sched.Stop()

	// Verify next_run was calculated and set in DB
	refreshed, err := db.GetTask(ctx, task.ID)
	if err != nil {
		t.Fatalf("failed to fetch task: %v", err)
	}
	if refreshed.NextRun == nil {
		t.Fatalf("expected next_run to be populated")
	}

	// Test ExecuteNow
	err = sched.ExecuteNow(ctx, task.ID)
	if err != nil {
		t.Fatalf("ExecuteNow failed: %v", err)
	}

	// Give async worker brief moment to complete
	time.Sleep(100 * time.Millisecond)

	// Verify task audit log recorded
	logs, err := db.ListAuditLogs(ctx, 10, 0)
	if err != nil || len(logs) == 0 {
		t.Fatalf("expected audit log for task execution")
	}
}
