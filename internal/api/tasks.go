package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/scheduler"
	"github.com/go-chi/chi/v5"
	"github.com/robfig/cron/v3"
)

type TaskHandler struct {
	db    *database.ManagerDB
	sched *scheduler.TaskScheduler
}

func NewTaskHandler(db *database.ManagerDB, sched *scheduler.TaskScheduler) *TaskHandler {
	return &TaskHandler{
		db:    db,
		sched: sched,
	}
}

func (h *TaskHandler) List(w http.ResponseWriter, r *http.Request) {
	var srvID *string
	if q := r.URL.Query().Get("server_id"); q != "" {
		srvID = &q
	}

	tasks, err := h.db.ListTasks(r.Context(), srvID)
	if err != nil {
		http.Error(w, `{"error": "Failed to list tasks"}`, http.StatusInternalServerError)
		return
	}
	if tasks == nil {
		tasks = []models.Task{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tasks)
}

func (h *TaskHandler) Create(w http.ResponseWriter, r *http.Request) {
	var t models.Task
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
		return
	}

	if t.Name == "" || t.CronExpr == "" || t.Action == "" {
		http.Error(w, `{"error": "Name, cron_expr, and action are required"}`, http.StatusBadRequest)
		return
	}

	// Validate cron expression
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	if _, err := parser.Parse(t.CronExpr); err != nil {
		http.Error(w, `{"error": "Invalid cron expression"}`, http.StatusBadRequest)
		return
	}

	t.Enabled = true
	if err := h.db.CreateTask(r.Context(), &t); err != nil {
		http.Error(w, `{"error": "Failed to create task"}`, http.StatusInternalServerError)
		return
	}

	if h.sched != nil {
		_ = h.sched.Reload(r.Context())
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(t)
}

func (h *TaskHandler) Update(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid task ID"}`, http.StatusBadRequest)
		return
	}

	var t models.Task
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		http.Error(w, `{"error": "Invalid request"}`, http.StatusBadRequest)
		return
	}

	t.ID = id
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	if _, err := parser.Parse(t.CronExpr); err != nil {
		http.Error(w, `{"error": "Invalid cron expression"}`, http.StatusBadRequest)
		return
	}

	if err := h.db.UpdateTask(r.Context(), &t); err != nil {
		http.Error(w, `{"error": "Failed to update task"}`, http.StatusInternalServerError)
		return
	}

	if h.sched != nil {
		_ = h.sched.Reload(r.Context())
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(t)
}

func (h *TaskHandler) Delete(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid task ID"}`, http.StatusBadRequest)
		return
	}

	if err := h.db.DeleteTask(r.Context(), id); err != nil {
		http.Error(w, `{"error": "Failed to delete task"}`, http.StatusInternalServerError)
		return
	}

	if h.sched != nil {
		_ = h.sched.Reload(r.Context())
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}

func (h *TaskHandler) Toggle(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid task ID"}`, http.StatusBadRequest)
		return
	}

	task, err := h.db.GetTask(r.Context(), id)
	if err != nil || task == nil {
		http.Error(w, `{"error": "Task not found"}`, http.StatusNotFound)
		return
	}

	newState := !task.Enabled
	if err := h.db.ToggleTask(r.Context(), id, newState); err != nil {
		http.Error(w, `{"error": "Failed to toggle task"}`, http.StatusInternalServerError)
		return
	}

	if h.sched != nil {
		_ = h.sched.Reload(r.Context())
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"id":      id,
		"enabled": newState,
	})
}

func (h *TaskHandler) RunNow(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, `{"error": "Invalid task ID"}`, http.StatusBadRequest)
		return
	}

	if h.sched != nil {
		if err := h.sched.ExecuteNow(r.Context(), id); err != nil {
			http.Error(w, `{"error": "Failed to trigger task execution"}`, http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]bool{"success": true})
}
