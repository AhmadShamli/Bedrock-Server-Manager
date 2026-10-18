package engine

import (
	"context"
	"testing"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

type mockLookup struct {
	servers map[string]*models.Server
}

func (m *mockLookup) GetServer(ctx context.Context, id string) (*models.Server, error) {
	if s, ok := m.servers[id]; ok {
		return s, nil
	}
	return nil, nil
}

func TestEngineRouter(t *testing.T) {
	dockerEng := NewMockEngine()
	nativeEng := NewMockEngine()
	lookup := &mockLookup{
		servers: map[string]*models.Server{
			"dock-1": {ID: "dock-1", Engine: models.EngineDocker},
			"nat-1":  {ID: "nat-1", Engine: models.EngineNative},
		},
	}

	router := NewEngineRouter(dockerEng, nativeEng, lookup)

	ctx := context.Background()

	// 1. Create Docker server
	dockSrv := &models.Server{ID: "dock-1", Engine: models.EngineDocker}
	cid, err := router.CreateServer(ctx, dockSrv, "/tmp")
	if err != nil {
		t.Fatalf("expected create server to succeed, got %v", err)
	}
	if cid == "" {
		t.Fatalf("expected non-empty cid")
	}

	// 2. Create Native server
	natSrv := &models.Server{ID: "nat-1", Engine: models.EngineNative}
	cidNat, err := router.CreateServer(ctx, natSrv, "/tmp")
	if err != nil {
		t.Fatalf("expected create native server to succeed, got %v", err)
	}
	if cidNat == "" {
		t.Fatalf("expected non-empty cidNat")
	}

	// 3. Start Native server
	if err := router.StartServer(ctx, natSrv); err != nil {
		t.Fatalf("failed to start native server: %v", err)
	}
	st, err := router.GetServerStatus(ctx, natSrv)
	if err != nil || st != models.ServerStatusRunning {
		t.Fatalf("expected running status for native server, got %s (err: %v)", st, err)
	}

	// 4. Log routing
	nativeEng.AppendMockLog("nat-1", "Native server started")
	dockerEng.AppendMockLog("dock-1", "Docker server started")

	natLogs := router.GetRecentLogs("nat-1")
	if len(natLogs) == 0 || natLogs[len(natLogs)-1] != "Native server started" {
		t.Fatalf("unexpected native logs: %v", natLogs)
	}

	dockLogs := router.GetRecentLogs("dock-1")
	if len(dockLogs) == 0 || dockLogs[len(dockLogs)-1] != "Docker server started" {
		t.Fatalf("unexpected docker logs: %v", dockLogs)
	}
}
