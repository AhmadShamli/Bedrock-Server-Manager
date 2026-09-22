package telemetry

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/database"
	"github.com/AhmadShamli/Bedrock-Server-Manager/internal/models"
)

// TelemetryCollector buffers high-frequency telemetry samples in memory
// and flushes them to MetricsDB in periodic batch transactions.
// It also manages downsampling rollups and retention pruning.
type TelemetryCollector struct {
	mu           sync.Mutex
	metricsDB    *database.MetricsDB
	buffer       []models.MetricRaw
	flushTicker  *time.Ticker
	rollupTicker *time.Ticker
	pruneTicker  *time.Ticker
	stopChan     chan struct{}

	RawRetainHours int
	Rollup5mDays   int
	Rollup1hDays   int
}

// NewTelemetryCollector creates a new TelemetryCollector.
func NewTelemetryCollector(db *database.MetricsDB) *TelemetryCollector {
	return &TelemetryCollector{
		metricsDB:      db,
		buffer:         make([]models.MetricRaw, 0, 200),
		stopChan:       make(chan struct{}),
		RawRetainHours: 6,
		Rollup5mDays:   7,
		Rollup1hDays:   30,
	}
}

// Ingest pushes a sample into the in-memory buffer.
func (tc *TelemetryCollector) Ingest(serverID string, cpuPercent float64, ramBytes int64, playerCount int) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.buffer = append(tc.buffer, models.MetricRaw{
		ServerID:    serverID,
		Timestamp:   time.Now().UTC(),
		CPUPercent:  cpuPercent,
		RAMBytes:    ramBytes,
		PlayerCount: playerCount,
	})
}

// Flush writes buffered samples to disk in a single transaction.
func (tc *TelemetryCollector) Flush(ctx context.Context) error {
	tc.mu.Lock()
	if len(tc.buffer) == 0 {
		tc.mu.Unlock()
		return nil
	}
	toFlush := tc.buffer
	tc.buffer = make([]models.MetricRaw, 0, 200)
	tc.mu.Unlock()

	return tc.metricsDB.InsertRawBatch(ctx, toFlush)
}

// Start launches the background flush, rollup, and prune loops.
func (tc *TelemetryCollector) Start(flushInterval, rollupInterval, pruneInterval time.Duration) {
	tc.flushTicker = time.NewTicker(flushInterval)
	tc.rollupTicker = time.NewTicker(rollupInterval)
	tc.pruneTicker = time.NewTicker(pruneInterval)

	go func() {
		for {
			select {
			case <-tc.stopChan:
				return
			case <-tc.flushTicker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				if err := tc.Flush(ctx); err != nil {
					log.Printf("[Telemetry] Error flushing buffer: %v", err)
				}
				cancel()
			case <-tc.rollupTicker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				now := time.Now().UTC()
				if err := tc.metricsDB.Rollup5Min(ctx, now); err != nil {
					log.Printf("[Telemetry] Error rolling up 5m: %v", err)
				}
				if err := tc.metricsDB.Rollup1Hour(ctx, now); err != nil {
					log.Printf("[Telemetry] Error rolling up 1h: %v", err)
				}
				cancel()
			case <-tc.pruneTicker.C:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				if err := tc.metricsDB.PruneOldMetrics(ctx, tc.RawRetainHours, tc.Rollup5mDays, tc.Rollup1hDays); err != nil {
					log.Printf("[Telemetry] Error pruning old metrics: %v", err)
				}
				cancel()
			}
		}
	}()
}

// Stop flushes remaining metrics and halts background loops.
func (tc *TelemetryCollector) Stop() {
	close(tc.stopChan)
	if tc.flushTicker != nil {
		tc.flushTicker.Stop()
	}
	if tc.rollupTicker != nil {
		tc.rollupTicker.Stop()
	}
	if tc.pruneTicker != nil {
		tc.pruneTicker.Stop()
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = tc.Flush(ctx)
}
