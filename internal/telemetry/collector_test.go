package telemetry

import (
	"context"
	"testing"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
)

func TestTelemetryCollector(t *testing.T) {
	metricsDB, err := database.OpenMetricsDB(":memory:")
	if err != nil {
		t.Fatalf("OpenMetricsDB failed: %v", err)
	}
	defer metricsDB.Close()

	col := NewTelemetryCollector(metricsDB)
	col.Ingest("srv-1", 10.5, 1024*1024*500, 4)
	col.Ingest("srv-1", 15.2, 1024*1024*520, 5)

	ctx := context.Background()
	if err := col.Flush(ctx); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	samples, err := metricsDB.QueryRaw(ctx, "srv-1", time.Now().Add(-1*time.Minute))
	if err != nil {
		t.Fatalf("QueryRaw failed: %v", err)
	}
	if len(samples) != 2 {
		t.Fatalf("expected 2 flushed samples, got %d", len(samples))
	}
	if samples[0].CPUPercent != 10.5 || samples[1].PlayerCount != 5 {
		t.Errorf("unexpected sample values: %+v", samples)
	}

	// Ingest an in-memory sample that is not yet flushed
	col.Ingest("srv-1", 20.0, 1024*1024*600, 6)

	combined, err := col.QueryRaw(ctx, "srv-1", time.Now().Add(-1*time.Minute))
	if err != nil {
		t.Fatalf("col.QueryRaw failed: %v", err)
	}
	if len(combined) != 3 {
		t.Fatalf("expected 3 total samples (2 disk + 1 in-memory), got %d", len(combined))
	}
	if combined[2].CPUPercent != 20.0 || combined[2].PlayerCount != 6 {
		t.Errorf("unexpected combined sample: %+v", combined[2])
	}

	// Verify GetLatest returns the newest ingested sample
	latest := col.GetLatest("srv-1")
	if latest == nil {
		t.Fatalf("expected GetLatest to return sample, got nil")
	}
	if latest.CPUPercent != 20.0 || latest.RAMBytes != 1024*1024*600 || latest.PlayerCount != 6 {
		t.Errorf("unexpected GetLatest sample: %+v", latest)
	}

	if none := col.GetLatest("non-existent-server"); none != nil {
		t.Errorf("expected nil for non-existent server, got %+v", none)
	}
}
