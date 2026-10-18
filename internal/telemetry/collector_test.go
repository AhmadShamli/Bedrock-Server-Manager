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

func TestTelemetryQueryMetricsAndRetention(t *testing.T) {
	metricsDB, err := database.OpenMetricsDB(":memory:")
	if err != nil {
		t.Fatalf("OpenMetricsDB failed: %v", err)
	}
	defer metricsDB.Close()

	col := NewTelemetryCollector(metricsDB)
	col.SetRetention(1, 12, 3)
	rawH, m5H, h1D := col.GetRetention()
	if rawH != 1 || m5H != 12 || h1D != 3 {
		t.Fatalf("unexpected retention values: %d, %d, %d", rawH, m5H, h1D)
	}

	ctx := context.Background()
	now := time.Now().UTC()

	// Ingest samples across time
	col.Ingest("srv-2", 12.0, 1024*1024*400, 2)
	col.Ingest("srv-2", 18.0, 1024*1024*450, 4)
	if err := col.Flush(ctx); err != nil {
		t.Fatalf("Flush failed: %v", err)
	}

	if err := metricsDB.Rollup5Min(ctx, now); err != nil {
		t.Fatalf("Rollup5Min failed: %v", err)
	}
	if err := metricsDB.Rollup1Hour(ctx, now); err != nil {
		t.Fatalf("Rollup1Hour failed: %v", err)
	}

	// Query ranges
	res15m, err := col.QueryMetrics(ctx, "srv-2", "15m", now.Add(-15*time.Minute))
	if err != nil || len(res15m) != 2 {
		t.Errorf("expected 2 samples for 15m, got %d (err: %v)", len(res15m), err)
	}

	res24h, err := col.QueryMetrics(ctx, "srv-2", "24h", now.Add(-24*time.Hour))
	if err != nil || len(res24h) == 0 {
		t.Errorf("expected rollup samples for 24h, got %d (err: %v)", len(res24h), err)
	}

	res7d, err := col.QueryMetrics(ctx, "srv-2", "7d", now.Add(-7*24*time.Hour))
	if err != nil || len(res7d) == 0 {
		t.Errorf("expected rollup samples for 7d, got %d (err: %v)", len(res7d), err)
	}

	// Test PruneAndVacuum
	if err := col.PruneAndVacuum(ctx); err != nil {
		t.Fatalf("PruneAndVacuum failed: %v", err)
	}
}
