package scheduler

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/backup"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/engine"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/robfig/cron/v3"
)

// TaskScheduler coordinates cron-based automated server routines.
type TaskScheduler struct {
	mu            sync.Mutex
	db            *database.ManagerDB
	eng           engine.ServerEngine
	dataDir       string
	cron          *cron.Cron
	entryIDs      map[int64]cron.EntryID
	runningMu     sync.Mutex
	restartDelays []time.Duration
}

// NewTaskScheduler initializes a new TaskScheduler instance.
func NewTaskScheduler(db *database.ManagerDB, eng engine.ServerEngine, dataDir string) *TaskScheduler {
	return &TaskScheduler{
		db:            db,
		eng:           eng,
		dataDir:       dataDir,
		cron:          cron.New(cron.WithParser(cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor))),
		entryIDs:      make(map[int64]cron.EntryID),
		restartDelays: []time.Duration{15 * time.Second, 10 * time.Second, 5 * time.Second},
	}
}

// SetRestartDelays configures custom countdown intervals for server restart routines (e.g. for testing).
func (s *TaskScheduler) SetRestartDelays(delays []time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restartDelays = delays
}

// Start begins the cron scheduler and loads active tasks from the database.
func (s *TaskScheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cron.Start()
	log.Println("[Scheduler] Cron scheduler started.")
	return s.reloadLocked(ctx)
}

// Stop terminates scheduled jobs.
func (s *TaskScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cron != nil {
		s.cron.Stop()
		log.Println("[Scheduler] Cron scheduler stopped.")
	}
}

// Reload queries active tasks from the database and updates cron jobs.
func (s *TaskScheduler) Reload(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reloadLocked(ctx)
}

func (s *TaskScheduler) reloadLocked(ctx context.Context) error {
	// Remove existing entries
	for _, entryID := range s.entryIDs {
		s.cron.Remove(entryID)
	}
	s.entryIDs = make(map[int64]cron.EntryID)

	tasks, err := s.db.ListTasks(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to query tasks for scheduler: %w", err)
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

	for _, t := range tasks {
		if !t.Enabled {
			continue
		}

		sched, err := parser.Parse(t.CronExpr)
		if err != nil {
			log.Printf("[Scheduler] Skipping task #%d (%s): invalid cron '%s': %v", t.ID, t.Name, t.CronExpr, err)
			continue
		}

		next := sched.Next(time.Now())
		_ = s.db.UpdateTaskRun(ctx, t.ID, time.Time{}, &next)

		taskCopy := t
		entryID := s.cron.Schedule(sched, cron.FuncJob(func() {
			s.executeTask(context.Background(), &taskCopy)
		}))
		s.entryIDs[t.ID] = entryID
		log.Printf("[Scheduler] Scheduled task #%d '%s' [%s] next run at %s", t.ID, t.Name, t.CronExpr, next.Format("2006-01-02 15:04:05"))
	}

	return nil
}

// ExecuteNow runs a specific task immediately outside its regular schedule.
func (s *TaskScheduler) ExecuteNow(ctx context.Context, taskID int64) error {
	task, err := s.db.GetTask(ctx, taskID)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}
	go s.executeTask(context.Background(), task)
	return nil
}

func (s *TaskScheduler) executeTask(ctx context.Context, t *models.Task) {
	s.runningMu.Lock()
	defer s.runningMu.Unlock()

	log.Printf("[Scheduler] Executing task #%d: '%s' (action: %s)", t.ID, t.Name, t.Action)
	now := time.Now().UTC()

	var execErr error
	switch t.Action {
	case "backup":
		execErr = s.runBackupTask(ctx, t)
	case "restart":
		execErr = s.runRestartTask(ctx, t)
	case "command":
		execErr = s.runCommandTask(ctx, t)
	default:
		execErr = fmt.Errorf("unknown action: %s", t.Action)
	}

	if execErr != nil {
		log.Printf("[Scheduler] Task #%d '%s' failed: %v", t.ID, t.Name, execErr)
	} else {
		log.Printf("[Scheduler] Task #%d '%s' completed successfully", t.ID, t.Name)
	}

	// Calculate next run
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	var nextRun *time.Time
	if sched, err := parser.Parse(t.CronExpr); err == nil {
		next := sched.Next(now)
		nextRun = &next
	}
	_ = s.db.UpdateTaskRun(ctx, t.ID, now, nextRun)

	_ = s.db.CreateAuditLog(ctx, &models.AuditLog{
		ActorType: "scheduler",
		ActorName: "cron",
		Action:    "task_executed",
		Target:    fmt.Sprintf("task_%d", t.ID),
		Details:   fmt.Sprintf(`{"name": "%s", "action": "%s", "error": "%v"}`, t.Name, t.Action, execErr),
	})
}

func (s *TaskScheduler) resolveTargetServers(ctx context.Context, t *models.Task) ([]*models.Server, error) {
	if t.ServerID != nil {
		srv, err := s.db.GetServer(ctx, *t.ServerID)
		if err != nil {
			return nil, fmt.Errorf("server not found: %w", err)
		}
		return []*models.Server{srv}, nil
	}

	servers, err := s.db.ListServers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list servers: %w", err)
	}
	result := make([]*models.Server, len(servers))
	for i := range servers {
		result[i] = &servers[i]
	}
	return result, nil
}

func (s *TaskScheduler) runBackupTask(ctx context.Context, t *models.Task) error {
	targets, err := s.resolveTargetServers(ctx, t)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		return nil
	}

	var errs []string
	for _, srv := range targets {
		serverDir := filepath.Join(s.dataDir, "servers", srv.ID)
		backupDir := filepath.Join(s.dataDir, "backups", srv.ID)
		if _, err := backup.CreateHotBackup(ctx, srv, serverDir, backupDir, s.eng, "scheduled", false, s.db); err != nil {
			errs = append(errs, fmt.Sprintf("server %s: %v", srv.ID, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("backup failed: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (s *TaskScheduler) runRestartTask(ctx context.Context, t *models.Task) error {
	targets, err := s.resolveTargetServers(ctx, t)
	if err != nil {
		return err
	}
	if len(targets) == 0 || s.eng == nil {
		return nil
	}

	var running []*models.Server
	for _, srv := range targets {
		if srv.Status == models.ServerStatusRunning {
			running = append(running, srv)
		}
	}

	s.mu.Lock()
	delays := s.restartDelays
	s.mu.Unlock()

	if len(running) > 0 && len(delays) == 3 {
		// Broadcast countdown warnings to in-game players across all target running servers
		for _, srv := range running {
			_ = s.eng.SendConsoleCommand(ctx, srv, "say [ALERT] Server scheduled restart in 30 seconds!")
		}
		if sleepWithContext(ctx, delays[0]) {
			for _, srv := range running {
				_ = s.eng.SendConsoleCommand(ctx, srv, "say [ALERT] Server scheduled restart in 15 seconds!")
			}
			if sleepWithContext(ctx, delays[1]) {
				for _, srv := range running {
					_ = s.eng.SendConsoleCommand(ctx, srv, "say [ALERT] Server scheduled restart in 5 seconds!")
				}
				_ = sleepWithContext(ctx, delays[2])
			}
		}
	}

	var errs []string
	for _, srv := range targets {
		if srv.Status == models.ServerStatusRunning {
			if err := s.eng.RestartServer(ctx, srv); err != nil {
				errs = append(errs, fmt.Sprintf("server %s restart failed: %v", srv.ID, err))
			}
		} else if srv.Status == models.ServerStatusStopped {
			if err := s.eng.StartServer(ctx, srv); err != nil {
				errs = append(errs, fmt.Sprintf("server %s start failed: %v", srv.ID, err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("restart errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

func (s *TaskScheduler) runCommandTask(ctx context.Context, t *models.Task) error {
	targets, err := s.resolveTargetServers(ctx, t)
	if err != nil {
		return err
	}
	if len(targets) == 0 || s.eng == nil {
		return nil
	}

	var errs []string
	for _, srv := range targets {
		if srv.Status == models.ServerStatusRunning {
			if err := s.eng.SendConsoleCommand(ctx, srv, t.Payload); err != nil {
				errs = append(errs, fmt.Sprintf("server %s command failed: %v", srv.ID, err))
			}
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("command execution errors: %s", strings.Join(errs, "; "))
	}
	return nil
}

func sleepWithContext(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
