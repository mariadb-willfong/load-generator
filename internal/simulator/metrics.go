package simulator

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// OperationType represents the type of database operation
type OperationType string

const (
	OpBalanceCheck    OperationType = "balance_check"
	OpHistoryView     OperationType = "history_view"
	OpTransfer        OperationType = "transfer"
	OpWithdrawal      OperationType = "withdrawal"
	OpDeposit         OperationType = "deposit"
	OpBatchPayroll    OperationType = "batch_payroll"
	OpAccountSweep    OperationType = "account_sweep"
	OpLogin           OperationType = "login"
	OpAuditLog        OperationType = "audit_log"
)

// EnhancedMetrics provides comprehensive metrics tracking with percentiles
type EnhancedMetrics struct {
	// Atomic counters for thread-safe updates
	totalOperations atomic.Int64
	totalErrors     atomic.Int64
	totalSessions   atomic.Int64
	readOps         atomic.Int64
	writeOps        atomic.Int64

	// Per-operation type tracking
	opCounts   map[OperationType]*atomic.Int64
	opLatency  map[OperationType]*LatencyTracker
	opMu       sync.RWMutex

	// Error tracking
	errorSim *ErrorSimulator

	// Session type tracking
	sessionCounts map[SessionType]*atomic.Int64
	sessionMu     sync.RWMutex

	// Timing
	startTime time.Time

	// Rolling window for recent TPS calculation
	recentOps     *RollingWindow
	recentErrors  *RollingWindow
}

// LatencyTracker maintains latency samples for percentile calculation
type LatencyTracker struct {
	mu       sync.Mutex
	samples  []time.Duration
	maxSize  int
	totalNs  int64
	count    int64
}

// RollingWindow tracks counts within a sliding time window
type RollingWindow struct {
	mu       sync.Mutex
	buckets  []windowBucket
	duration time.Duration
	bucketMs int64
}

type windowBucket struct {
	timestamp int64 // Unix milliseconds
	count     int64
}

// NewEnhancedMetrics creates a new metrics tracker with error simulation
func NewEnhancedMetrics(errorSim *ErrorSimulator) *EnhancedMetrics {
	m := &EnhancedMetrics{
		opCounts:      make(map[OperationType]*atomic.Int64),
		opLatency:     make(map[OperationType]*LatencyTracker),
		sessionCounts: make(map[SessionType]*atomic.Int64),
		errorSim:      errorSim,
		startTime:     time.Now(),
		recentOps:     NewRollingWindow(time.Minute, 100*time.Millisecond),
		recentErrors:  NewRollingWindow(time.Minute, 100*time.Millisecond),
	}

	// Initialize operation counters
	for _, op := range []OperationType{OpBalanceCheck, OpHistoryView, OpTransfer, OpWithdrawal, OpDeposit, OpBatchPayroll, OpAccountSweep, OpLogin, OpAuditLog} {
		m.opCounts[op] = &atomic.Int64{}
		m.opLatency[op] = NewLatencyTracker(10000) // Keep last 10k samples per operation
	}

	// Initialize session counters
	for _, st := range []SessionType{SessionTypeATM, SessionTypeOnline, SessionTypeBusiness} {
		m.sessionCounts[st] = &atomic.Int64{}
	}

	return m
}

// NewLatencyTracker creates a new latency tracker with reservoir sampling
func NewLatencyTracker(maxSize int) *LatencyTracker {
	return &LatencyTracker{
		samples: make([]time.Duration, 0, maxSize),
		maxSize: maxSize,
	}
}

// Record adds a latency sample
func (lt *LatencyTracker) Record(latency time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	lt.totalNs += latency.Nanoseconds()
	lt.count++

	// Reservoir sampling: keep maxSize samples with uniform probability
	if len(lt.samples) < lt.maxSize {
		lt.samples = append(lt.samples, latency)
	} else {
		// Random replacement for reservoir sampling
		// For simplicity, replace oldest entry (FIFO) when full
		copy(lt.samples, lt.samples[1:])
		lt.samples[len(lt.samples)-1] = latency
	}
}

// Percentile returns the p-th percentile latency
func (lt *LatencyTracker) Percentile(p float64) time.Duration {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	if len(lt.samples) == 0 {
		return 0
	}

	// Sort samples (on copy to avoid modifying original)
	sorted := make([]time.Duration, len(lt.samples))
	copy(sorted, lt.samples)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	idx := int(float64(len(sorted)-1) * p / 100.0)
	return sorted[idx]
}

// Average returns the average latency
func (lt *LatencyTracker) Average() time.Duration {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	if lt.count == 0 {
		return 0
	}
	return time.Duration(lt.totalNs / lt.count)
}

// Count returns the total number of samples recorded
func (lt *LatencyTracker) Count() int64 {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	return lt.count
}

// NewRollingWindow creates a new rolling window for TPS calculation
func NewRollingWindow(duration time.Duration, bucketSize time.Duration) *RollingWindow {
	numBuckets := int(duration / bucketSize)
	if numBuckets < 10 {
		numBuckets = 10
	}
	return &RollingWindow{
		buckets:  make([]windowBucket, 0, numBuckets),
		duration: duration,
		bucketMs: bucketSize.Milliseconds(),
	}
}

// Add increments the count in the current time bucket
func (rw *RollingWindow) Add(count int64) {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	now := time.Now().UnixMilli()
	bucketTime := (now / rw.bucketMs) * rw.bucketMs

	// Prune old buckets
	cutoff := now - rw.duration.Milliseconds()
	newBuckets := rw.buckets[:0]
	for _, b := range rw.buckets {
		if b.timestamp >= cutoff {
			newBuckets = append(newBuckets, b)
		}
	}
	rw.buckets = newBuckets

	// Add to current bucket or create new one
	if len(rw.buckets) > 0 && rw.buckets[len(rw.buckets)-1].timestamp == bucketTime {
		rw.buckets[len(rw.buckets)-1].count += count
	} else {
		rw.buckets = append(rw.buckets, windowBucket{timestamp: bucketTime, count: count})
	}
}

// Rate returns the rate per second over the window
func (rw *RollingWindow) Rate() float64 {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	now := time.Now().UnixMilli()
	cutoff := now - rw.duration.Milliseconds()

	var total int64
	for _, b := range rw.buckets {
		if b.timestamp >= cutoff {
			total += b.count
		}
	}

	seconds := float64(rw.duration) / float64(time.Second)
	return float64(total) / seconds
}

// RecordOperation records a completed operation with latency
func (m *EnhancedMetrics) RecordOperation(opType OperationType, isWrite bool, latency time.Duration) {
	m.totalOperations.Add(1)
	m.recentOps.Add(1)

	if isWrite {
		m.writeOps.Add(1)
	} else {
		m.readOps.Add(1)
	}

	// Track per-operation metrics
	m.opMu.RLock()
	if counter, exists := m.opCounts[opType]; exists {
		counter.Add(1)
	}
	if tracker, exists := m.opLatency[opType]; exists {
		tracker.Record(latency)
	}
	m.opMu.RUnlock()
}

// RecordError records an error with classification.
// Only infrastructure errors (database, unknown) are counted in the main error metric.
// Simulated errors (auth failures, insufficient funds, etc.) are tracked separately
// since they represent deliberate simulation behavior, not actual problems.
func (m *EnhancedMetrics) RecordError(errType ErrorType) {
	// Always track in the error simulator for detailed breakdown
	if m.errorSim != nil {
		m.errorSim.RecordError(errType)
	}

	// Only count infrastructure errors in the main error counter
	if !IsSimulatedErrorType(errType) {
		m.totalErrors.Add(1)
		m.recentErrors.Add(1)
	}
}

// RecordSessionComplete records a completed session
func (m *EnhancedMetrics) RecordSessionComplete(sessionType SessionType) {
	m.totalSessions.Add(1)

	m.sessionMu.RLock()
	if counter, exists := m.sessionCounts[sessionType]; exists {
		counter.Add(1)
	}
	m.sessionMu.RUnlock()
}

// EnhancedSnapshot contains comprehensive metrics
type EnhancedSnapshot struct {
	// Overall counts
	TotalSessions   int64
	TotalOperations int64
	TotalErrors     int64
	ReadOps         int64
	WriteOps        int64

	// Rates
	TPS       float64
	RecentTPS float64 // TPS over last minute
	ErrorRate float64 // Errors per second

	// Overall latency
	AvgLatency time.Duration
	P50Latency time.Duration
	P95Latency time.Duration
	P99Latency time.Duration

	// Per-operation breakdown
	OperationStats map[OperationType]OperationStat

	// Session breakdown
	SessionStats map[SessionType]int64

	// Error breakdown
	ErrorStats map[ErrorType]int64

	// Timing
	Uptime time.Duration
}

// OperationStat holds stats for a single operation type
type OperationStat struct {
	Count      int64
	AvgLatency time.Duration
	P50Latency time.Duration
	P95Latency time.Duration
	P99Latency time.Duration
}

// Snapshot returns current metrics
func (m *EnhancedMetrics) Snapshot() EnhancedSnapshot {
	elapsed := time.Since(m.startTime).Seconds()
	if elapsed < 1 {
		elapsed = 1
	}

	ops := m.totalOperations.Load()
	errors := m.totalErrors.Load()

	// Calculate overall latency from all operation latencies
	var totalNs int64
	var totalCount int64
	allSamples := make([]time.Duration, 0, 10000)

	m.opMu.RLock()
	opStats := make(map[OperationType]OperationStat)
	for opType, tracker := range m.opLatency {
		stat := OperationStat{
			Count:      m.opCounts[opType].Load(),
			AvgLatency: tracker.Average(),
			P50Latency: tracker.Percentile(50),
			P95Latency: tracker.Percentile(95),
			P99Latency: tracker.Percentile(99),
		}
		opStats[opType] = stat

		// Collect samples for overall percentiles
		tracker.mu.Lock()
		allSamples = append(allSamples, tracker.samples...)
		totalNs += tracker.totalNs
		totalCount += tracker.count
		tracker.mu.Unlock()
	}
	m.opMu.RUnlock()

	// Calculate overall percentiles
	var avgLatency, p50, p95, p99 time.Duration
	if totalCount > 0 {
		avgLatency = time.Duration(totalNs / totalCount)
	}
	if len(allSamples) > 0 {
		sort.Slice(allSamples, func(i, j int) bool { return allSamples[i] < allSamples[j] })
		p50 = allSamples[len(allSamples)*50/100]
		p95 = allSamples[len(allSamples)*95/100]
		p99 = allSamples[len(allSamples)*99/100]
	}

	// Session stats
	m.sessionMu.RLock()
	sessionStats := make(map[SessionType]int64)
	for st, counter := range m.sessionCounts {
		sessionStats[st] = counter.Load()
	}
	m.sessionMu.RUnlock()

	// Error stats
	var errorStats map[ErrorType]int64
	if m.errorSim != nil {
		errorStats = m.errorSim.GetAllErrorCounts()
	}

	return EnhancedSnapshot{
		TotalSessions:   m.totalSessions.Load(),
		TotalOperations: ops,
		TotalErrors:     errors,
		ReadOps:         m.readOps.Load(),
		WriteOps:        m.writeOps.Load(),
		TPS:             float64(ops) / elapsed,
		RecentTPS:       m.recentOps.Rate(),
		ErrorRate:       float64(errors) / elapsed,
		AvgLatency:      avgLatency,
		P50Latency:      p50,
		P95Latency:      p95,
		P99Latency:      p99,
		OperationStats:  opStats,
		SessionStats:    sessionStats,
		ErrorStats:      errorStats,
		Uptime:          time.Since(m.startTime),
	}
}

// FormatMetricsLine returns a formatted one-line metrics summary
func (m *EnhancedMetrics) FormatMetricsLine() string {
	snap := m.Snapshot()
	return fmt.Sprintf("TPS: %.1f (recent: %.1f) | Ops: %d | Errors: %d (%.2f/s) | Latency: avg=%s p95=%s p99=%s",
		snap.TPS,
		snap.RecentTPS,
		snap.TotalOperations,
		snap.TotalErrors,
		snap.ErrorRate,
		snap.AvgLatency.Round(time.Microsecond),
		snap.P95Latency.Round(time.Microsecond),
		snap.P99Latency.Round(time.Microsecond),
	)
}

// PrintDetailedStats outputs comprehensive statistics
func (m *EnhancedMetrics) PrintDetailedStats() {
	snap := m.Snapshot()

	fmt.Println("\n=== Detailed Metrics ===")
	fmt.Printf("Uptime: %s\n\n", snap.Uptime.Round(time.Second))

	fmt.Println("--- Operations ---")
	fmt.Printf("Total Operations: %d (Read: %d, Write: %d)\n", snap.TotalOperations, snap.ReadOps, snap.WriteOps)
	fmt.Printf("Overall TPS: %.2f (Recent: %.2f)\n", snap.TPS, snap.RecentTPS)
	fmt.Printf("Total Sessions: %d\n", snap.TotalSessions)

	fmt.Println("\n--- Latency ---")
	fmt.Printf("Average: %s\n", snap.AvgLatency.Round(time.Microsecond))
	fmt.Printf("P50:     %s\n", snap.P50Latency.Round(time.Microsecond))
	fmt.Printf("P95:     %s\n", snap.P95Latency.Round(time.Microsecond))
	fmt.Printf("P99:     %s\n", snap.P99Latency.Round(time.Microsecond))

	fmt.Println("\n--- Per-Operation Stats ---")
	for opType, stat := range snap.OperationStats {
		if stat.Count > 0 {
			fmt.Printf("  %-15s: count=%d avg=%s p95=%s\n",
				opType, stat.Count, stat.AvgLatency.Round(time.Microsecond), stat.P95Latency.Round(time.Microsecond))
		}
	}

	fmt.Println("\n--- Session Types ---")
	for st, count := range snap.SessionStats {
		if count > 0 {
			fmt.Printf("  %-15s: %d\n", st.String(), count)
		}
	}

	if len(snap.ErrorStats) > 0 {
		fmt.Println("\n--- Errors ---")
		fmt.Printf("Total Errors: %d (%.2f/sec)\n", snap.TotalErrors, snap.ErrorRate)
		for errType, count := range snap.ErrorStats {
			if count > 0 {
				fmt.Printf("  %-15s: %d\n", errType, count)
			}
		}
	}
}
