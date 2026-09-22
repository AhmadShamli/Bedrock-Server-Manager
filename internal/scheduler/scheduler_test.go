package scheduler

import (
	"context"
	"os"
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
	_ = os.MkdirAll(filepath.Join(tmpDir, "servers", srv.ID, "worlds", "Bedrock level"), 0755)

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

func TestGlobalTaskExecution(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	db, err := database.OpenManagerDB(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("OpenManagerDB failed: %v", err)
	}
	defer db.Close()

	srv1 := &models.Server{
		ID:     "srv-1",
		Name:   "Server One",
		Status: models.ServerStatusRunning,
	}
	srv2 := &models.Server{
		ID:     "srv-2",
		Name:   "Server Two",
		Status: models.ServerStatusStopped,
	}
	_ = db.CreateServer(ctx, srv1)
	_ = db.CreateServer(ctx, srv2)

	_ = os.MkdirAll(filepath.Join(tmpDir, "servers", srv1.ID, "worlds", "Bedrock level"), 0755)
	_ = os.MkdirAll(filepath.Join(tmpDir, "servers", srv2.ID, "worlds", "Bedrock level"), 0755)

	mockEng := engine.NewMockEngine()
	sched := NewTaskScheduler(db, mockEng, tmpDir)
	sched.SetRestartDelays([]time.Duration{0, 0, 0})

	// Global command task (ServerID == nil)
	globalCmdTask := &models.Task{
		ServerID: nil,
		Name:     "Global Broadcast",
		CronExpr: "0 0 * * *",
		Action:   "command",
		Payload:  "say Scheduled Server-Wide Maintenance Alert",
		Enabled:  true,
	}
	if err := db.CreateTask(ctx, globalCmdTask); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Global backup task (ServerID == nil)
	globalBackupTask := &models.Task{
		ServerID: nil,
		Name:     "Global Backup",
		CronExpr: "0 3 * * *",
		Action:   "backup",
		Enabled:  true,
	}
	if err := db.CreateTask(ctx, globalBackupTask); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Global restart task (ServerID == nil)
	globalRestartTask := &models.Task{
		ServerID: nil,
		Name:     "Global Restart",
		CronExpr: "0 5 * * *",
		Action:   "restart",
		Enabled:  true,
	}
	if err := db.CreateTask(ctx, globalRestartTask); err != nil {
		t.Fatalf("CreateTask failed: %v", err)
	}

	// Test execute command task
	if err := sched.runCommandTask(ctx, globalCmdTask); err != nil {
		t.Fatalf("runCommandTask for global task failed: %v", err)
	}

	// Test execute backup task
	if err := sched.runBackupTask(ctx, globalBackupTask); err != nil {
		t.Fatalf("runBackupTask for global task failed: %v", err)
	}

	// Test execute restart task
	if err := sched.runRestartTask(ctx, globalRestartTask); err != nil {
		t.Fatalf("runRestartTask for global task failed: %v", err)
	}
}
