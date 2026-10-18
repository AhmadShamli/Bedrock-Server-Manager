package telemetry

import (
	"context"
	"log"
	"math"
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
	latest       map[string]models.MetricRaw
	flushTicker  *time.Ticker
	rollupTicker *time.Ticker
	pruneTicker  *time.Ticker
	stopChan     chan struct{}
	stopOnce     sync.Once

	RawRetainHours int
	Rollup5mHours  int
	Rollup1hDays   int
}

// NewTelemetryCollector creates a new TelemetryCollector.
func NewTelemetryCollector(db *database.MetricsDB) *TelemetryCollector {
	return &TelemetryCollector{
		metricsDB:      db,
		buffer:         make([]models.MetricRaw, 0, 200),
		latest:         make(map[string]models.MetricRaw),
		stopChan:       make(chan struct{}),
		RawRetainHours: 2,
		Rollup5mHours:  24,
		Rollup1hDays:   7,
	}
}

// Ingest pushes a sample into the in-memory buffer and updates the latest sample map.
func (tc *TelemetryCollector) Ingest(serverID string, cpuPercent float64, ramBytes int64, playerCount int) {
	if math.IsNaN(cpuPercent) || math.IsInf(cpuPercent, 0) || cpuPercent < 0 || cpuPercent > 10000 {
		cpuPercent = 0
	}
	if ramBytes < 0 {
		ramBytes = 0
	}

	tc.mu.Lock()
	defer tc.mu.Unlock()

	sample := models.MetricRaw{
		ServerID:    serverID,
		Timestamp:   time.Now().UTC(),
		CPUPercent:  cpuPercent,
		RAMBytes:    ramBytes,
		PlayerCount: playerCount,
	}

	tc.buffer = append(tc.buffer, sample)
	if tc.latest == nil {
		tc.latest = make(map[string]models.MetricRaw)
	}
	tc.latest[serverID] = sample
}

// GetLatest retrieves the most recent in-memory sample for a server, or nil if not found.
func (tc *TelemetryCollector) GetLatest(serverID string) *models.MetricRaw {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if tc.latest == nil {
		return nil
	}
	sample, ok := tc.latest[serverID]
	if !ok {
		return nil
	}
	res := sample
	return &res
}

// QueryRaw retrieves metrics for a server since a given timestamp, combining database records and in-memory buffer.
func (tc *TelemetryCollector) QueryRaw(ctx context.Context, serverID string, since time.Time) ([]models.MetricRaw, error) {
	var dbRecords []models.MetricRaw
	var err error
	if tc.metricsDB != nil {
		dbRecords, err = tc.metricsDB.QueryRaw(ctx, serverID, since)
		if err != nil {
			return nil, err
		}
	}

	tc.mu.Lock()
	var inMemory []models.MetricRaw
	for _, m := range tc.buffer {
		if m.ServerID == serverID && !m.Timestamp.Before(since) {
			inMemory = append(inMemory, m)
		}
	}
	tc.mu.Unlock()

	if len(inMemory) == 0 {
		return dbRecords, nil
	}

	return append(dbRecords, inMemory...), nil
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

	if err := tc.metricsDB.InsertRawBatch(ctx, toFlush); err != nil {
		tc.mu.Lock()
		if len(tc.buffer)+len(toFlush) < 5000 {
			tc.buffer = append(toFlush, tc.buffer...)
		}
		tc.mu.Unlock()
		return err
	}
	return nil
}

// SetRetention updates in-memory retention thresholds.
func (tc *TelemetryCollector) SetRetention(rawHours, rollup5mHours, rollup1hDays int) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	if rawHours > 0 {
		tc.RawRetainHours = rawHours
	}
	if rollup5mHours > 0 {
		tc.Rollup5mHours = rollup5mHours
	}
	if rollup1hDays > 0 {
		tc.Rollup1hDays = rollup1hDays
	}
}

// GetRetention returns current retention thresholds.
func (tc *TelemetryCollector) GetRetention() (int, int, int) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	return tc.RawRetainHours, tc.Rollup5mHours, tc.Rollup1hDays
}

// PruneAndVacuum flushes pending samples, prunes old data per current retention, and runs SQLite VACUUM.
func (tc *TelemetryCollector) PruneAndVacuum(ctx context.Context) error {
	tc.mu.Lock()
	rawHours := tc.RawRetainHours
	m5Hours := tc.Rollup5mHours
	h1Days := tc.Rollup1hDays
	tc.mu.Unlock()

	_ = tc.Flush(ctx)

	if tc.metricsDB != nil {
		if err := tc.metricsDB.PruneOldMetrics(ctx, rawHours, m5Hours, h1Days); err != nil {
			return err
		}
		return tc.metricsDB.Vacuum(ctx)
	}
	return nil
}

// QueryMetrics returns the appropriate telemetry series based on time range:
// - "15m", "1h": Raw telemetry samples + unflushed in-memory buffer
// - "6h", "24h": 5-minute rollup points + recent raw/buffer tail
// - "3d", "7d": 1-hour rollup points + recent 5m/raw/buffer tail
func (tc *TelemetryCollector) QueryMetrics(ctx context.Context, serverID string, timeRange string, since time.Time) ([]models.MetricRaw, error) {
	switch timeRange {
	case "15m", "1h":
		return tc.QueryRaw(ctx, serverID, since)

	case "6h", "24h":
		var m5 []models.MetricRaw
		var err error
		if tc.metricsDB != nil {
			m5, err = tc.metricsDB.QueryRollup5m(ctx, serverID, since)
			if err != nil {
				return nil, err
			}
		}

		rawSince := since
		if len(m5) > 0 {
			rawSince = m5[len(m5)-1].Timestamp
		}

		rawTail, _ := tc.QueryRaw(ctx, serverID, rawSince)
		if len(m5) == 0 {
			return rawTail, nil
		}

		lastTs := m5[len(m5)-1].Timestamp
		for _, r := range rawTail {
			if r.Timestamp.After(lastTs) {
				m5 = append(m5, r)
			}
		}
		return m5, nil

	case "3d", "7d":
		var h1 []models.MetricRaw
		var err error
		if tc.metricsDB != nil {
			h1, err = tc.metricsDB.QueryRollup1h(ctx, serverID, since)
			if err != nil {
				return nil, err
			}
		}

		m5Since := since
		if len(h1) > 0 {
			m5Since = h1[len(h1)-1].Timestamp
		}

		var m5Tail []models.MetricRaw
		if tc.metricsDB != nil {
			m5Tail, _ = tc.metricsDB.QueryRollup5m(ctx, serverID, m5Since)
		}

		rawSince := m5Since
		if len(m5Tail) > 0 {
			rawSince = m5Tail[len(m5Tail)-1].Timestamp
		}
		rawTail, _ := tc.QueryRaw(ctx, serverID, rawSince)

		combined := h1
		lastH1Ts := time.Time{}
		if len(combined) > 0 {
			lastH1Ts = combined[len(combined)-1].Timestamp
		}
		for _, m := range m5Tail {
			if m.Timestamp.After(lastH1Ts) {
				combined = append(combined, m)
			}
		}

		lastCombinedTs := time.Time{}
		if len(combined) > 0 {
			lastCombinedTs = combined[len(combined)-1].Timestamp
		}
		for _, r := range rawTail {
			if r.Timestamp.After(lastCombinedTs) {
				combined = append(combined, r)
			}
		}
		return combined, nil

	default:
		return tc.QueryRaw(ctx, serverID, since)
	}
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
				tc.mu.Lock()
				rawHours := tc.RawRetainHours
				m5Hours := tc.Rollup5mHours
				h1Days := tc.Rollup1hDays
				tc.mu.Unlock()
				if err := tc.metricsDB.PruneOldMetrics(ctx, rawHours, m5Hours, h1Days); err != nil {
					log.Printf("[Telemetry] Error pruning old metrics: %v", err)
				}
				cancel()
			}
		}
	}()
}

// Stop flushes remaining metrics and halts background loops.
func (tc *TelemetryCollector) Stop() {
	tc.stopOnce.Do(func() {
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
	})
}
