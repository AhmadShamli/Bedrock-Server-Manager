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
}
