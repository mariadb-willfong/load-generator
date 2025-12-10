package simulator

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/willfong/load-generator/internal/config"
	"github.com/willfong/load-generator/internal/database"
	"github.com/willfong/load-generator/internal/simulator/burst"
	"github.com/willfong/load-generator/internal/utils"
)

// SessionManager coordinates concurrent customer sessions
type SessionManager struct {
	pool      *database.Pool
	queries   *database.Queries
	config    config.SimulateConfig
	rng       *utils.Random
	scheduler *Scheduler // Timezone-aware session scheduler

	// Burst and load control
	burstMgr    *burst.Manager
	loadCtrl    *LoadController

	// Error simulation
	errorSim *ErrorSimulator

	// Active sessions
	sessions sync.Map // sessionID -> *CustomerSession
	wg       sync.WaitGroup

	// Lifecycle control
	ctx    context.Context
	cancel context.CancelFunc

	// Metrics and audit
	metrics     *EnhancedMetrics
	auditWriter *AuditWriter

	// Graceful shutdown
	drainTimeout time.Duration
	stopping     atomic.Bool
}

// NewSessionManager creates a new session manager
func NewSessionManager(pool *database.Pool, cfg config.SimulateConfig, seed int64) *SessionManager {
	ctx, cancel := context.WithCancel(context.Background())

	rng := utils.NewRandom(seed)
	queries := database.NewQueries(pool)

	// Initialize burst manager with configured providers
	burstMgr := burst.NewManager()

	// Register lunch burst provider
	if cfg.EnableLunchBurst {
		lunchCfg := burst.BurstConfig{
			Enabled:    true,
			Multiplier: cfg.LunchBurstMultiplier,
			Duration:   cfg.LunchBurstDuration,
		}
		if lunchCfg.Multiplier == 0 {
			lunchCfg.Multiplier = cfg.BurstMultiplier
		}
		burstMgr.RegisterProvider(burst.NewLunchBurst(lunchCfg))
	}

	// Register payroll burst provider
	if cfg.EnablePayrollBurst {
		payrollCfg := burst.BurstConfig{
			Enabled:    true,
			Multiplier: cfg.PayrollBurstMultiplier,
			Duration:   cfg.PayrollBurstDuration,
		}
		if payrollCfg.Multiplier == 0 {
			payrollCfg.Multiplier = cfg.BurstMultiplier
		}
		burstMgr.RegisterProvider(burst.NewPayrollBurst(payrollCfg))
	}

	// Register random burst provider
	if cfg.EnableRandomBurst {
		randomCfg := burst.BurstConfig{
			Enabled:     true,
			Multiplier:  cfg.RandomBurstMaxMultiplier,
			Duration:    cfg.RandomBurstMaxDuration,
			Probability: cfg.RandomBurstProbability,
		}
		randomBurst := burst.NewRandomBurst(randomCfg, seed)
		randomBurst.SetDurationRange(cfg.RandomBurstMinDuration, cfg.RandomBurstMaxDuration)
		randomBurst.SetMultiplierRange(cfg.RandomBurstMinMultiplier, cfg.RandomBurstMaxMultiplier)
		randomBurst.SetCooldown(cfg.RandomBurstCooldown)
		burstMgr.RegisterProvider(randomBurst)
	}

	// Initialize load controller
	loadCtrl := NewLoadController(cfg)

	// Initialize error simulator
	errorSim := NewErrorSimulator(cfg)

	// Initialize audit writer
	auditWriter := NewAuditWriter(pool, DefaultAuditWriterConfig())

	return &SessionManager{
		pool:         pool,
		queries:      queries,
		config:       cfg,
		rng:          rng,
		scheduler:    NewScheduler(queries, cfg),
		burstMgr:     burstMgr,
		loadCtrl:     loadCtrl,
		errorSim:     errorSim,
		ctx:          ctx,
		cancel:       cancel,
		metrics:      NewEnhancedMetrics(errorSim),
		auditWriter:  auditWriter,
		drainTimeout: 30 * time.Second,
	}
}

// Start launches the simulation with the configured number of concurrent sessions
func (sm *SessionManager) Start() error {
	fmt.Printf("Starting simulation with %d concurrent sessions...\n", sm.config.NumSessions)

	// Start audit writer
	sm.auditWriter.Start()
	fmt.Println("Audit writer started")

	// Initialize scheduler's customer cache for weighted timezone selection
	fmt.Println("Building timezone-aware customer cache...")
	if err := sm.scheduler.RefreshCustomerCache(sm.ctx); err != nil {
		fmt.Printf("Warning: Could not build customer cache (will use random selection): %v\n", err)
	} else {
		cacheStats := sm.scheduler.GetCacheStats()
		fmt.Printf("Cached %d customers across %d timezones\n",
			cacheStats.TotalCustomers, cacheStats.TimezoneCount)
	}

	// Show initial global activity snapshot
	activity := sm.scheduler.GetGlobalActivitySummary()
	fmt.Printf("Current global activity level: %s\n", activity)

	// Display burst configuration
	sm.printBurstConfig()

	// Launch burst manager background monitoring
	go sm.burstMgr.Run(sm.ctx, 30*time.Second, func(event *burst.BurstEvent) {
		fmt.Printf("[BURST] %s triggered: %.1fx multiplier for %s\n",
			event.Type, event.Multiplier, event.RemainingDuration().Round(time.Second))
	})

	// Launch load controller if ramp is enabled
	if sm.config.EnableRamp {
		fmt.Printf("Load ramping enabled: %s ramp-up, %s ramp-down\n",
			sm.config.RampUpDuration, sm.config.RampDownDuration)
		sm.loadCtrl.SetOnPhaseChange(func(phase LoadPhase) {
			fmt.Printf("[LOAD] Phase changed: %s\n", phase)
		})
		go sm.loadCtrl.Run(sm.ctx)
	} else {
		// Start immediately at full load
		sm.loadCtrl.Start()
	}

	// Launch metrics reporter
	go sm.reportMetrics()

	// Launch periodic cache refresh (every 5 minutes)
	go sm.refreshCachePeriodically()

	// Launch session workers
	for i := 0; i < sm.config.NumSessions; i++ {
		sm.wg.Add(1)
		go sm.runSession(i)
	}

	fmt.Println("All sessions started. Press Ctrl+C to stop.")
	return nil
}

// TriggerManualBurst creates an on-demand burst for testing
func (sm *SessionManager) TriggerManualBurst(multiplier float64, duration time.Duration, extraSessions int) *burst.BurstEvent {
	event := sm.burstMgr.TriggerManualBurst(multiplier, duration, extraSessions)
	if event != nil {
		fmt.Printf("[BURST] Manual burst triggered: %.1fx multiplier for %s\n",
			multiplier, duration)
	}
	return event
}

// GetBurstMultiplier returns the current combined burst multiplier
func (sm *SessionManager) GetBurstMultiplier() float64 {
	return sm.burstMgr.GetActiveMultiplier()
}

// GetActiveBursts returns all currently active burst events
func (sm *SessionManager) GetActiveBursts() []*burst.BurstEvent {
	return sm.burstMgr.GetActiveBursts()
}

// GetLoadController returns the load controller for external control
func (sm *SessionManager) GetLoadController() *LoadController {
	return sm.loadCtrl
}

// printBurstConfig displays the configured burst settings
func (sm *SessionManager) printBurstConfig() {
	var enabled []string
	if sm.config.EnableLunchBurst {
		enabled = append(enabled, fmt.Sprintf("lunch(%.1fx)", sm.config.LunchBurstMultiplier))
	}
	if sm.config.EnablePayrollBurst {
		enabled = append(enabled, fmt.Sprintf("payroll(%.1fx)", sm.config.PayrollBurstMultiplier))
	}
	if sm.config.EnableRandomBurst {
		enabled = append(enabled, fmt.Sprintf("random(%.0f%%)", sm.config.RandomBurstProbability*100))
	}

	if len(enabled) > 0 {
		fmt.Printf("Burst scenarios enabled: %v\n", enabled)
	} else {
		fmt.Println("No burst scenarios enabled")
	}
}

// refreshCachePeriodically refreshes the scheduler's customer cache
func (sm *SessionManager) refreshCachePeriodically() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := sm.scheduler.RefreshCustomerCache(sm.ctx); err != nil {
				// Log but don't fail - cache is still valid from last refresh
				fmt.Printf("Warning: Failed to refresh customer cache: %v\n", err)
			}
		case <-sm.ctx.Done():
			return
		}
	}
}

// Stop gracefully shuts down all sessions
func (sm *SessionManager) Stop() {
	if sm.stopping.Swap(true) {
		return // Already stopping
	}

	fmt.Println("\nInitiating graceful shutdown...")
	startTime := time.Now()

	// Signal all sessions to stop
	sm.cancel()

	// Wait for sessions with timeout
	done := make(chan struct{})
	go func() {
		sm.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		fmt.Printf("All sessions stopped in %s\n", time.Since(startTime).Round(time.Millisecond))
	case <-time.After(sm.drainTimeout):
		fmt.Printf("Warning: Timeout waiting for sessions after %s\n", sm.drainTimeout)
	}

	// Stop audit writer (drains remaining logs)
	fmt.Println("Draining audit log buffer...")
	if err := sm.auditWriter.Stop(); err != nil {
		fmt.Printf("Warning: Audit writer shutdown error: %v\n", err)
	} else {
		fmt.Println("Audit writer stopped")
	}

	fmt.Println("Shutdown complete.")
	sm.printFinalStats()
}

// Wait blocks until all sessions complete (when Stop is called)
func (sm *SessionManager) Wait() {
	sm.wg.Wait()
}

// runSession runs a single customer session loop
func (sm *SessionManager) runSession(workerID int) {
	defer sm.wg.Done()

	// Create a child RNG for this worker (deterministic)
	workerRng := sm.rng.Fork()

	for {
		select {
		case <-sm.ctx.Done():
			return
		default:
			// Create a new session
			session, err := sm.createSession(workerID, workerRng)
			if err != nil {
				// Session creation errors are always infrastructure failures
				// (database issues, connection problems) - halt immediately
				fmt.Fprintf(os.Stderr, "\nFatal: session creation failed: %v\n", err)
				os.Exit(1)
			}

			// Run the session workflow
			sm.executeSession(session)
		}
	}
}

// createSession initializes a new customer session
func (sm *SessionManager) createSession(workerID int, rng *utils.Random) (*CustomerSession, error) {
	ctx, cancel := context.WithTimeout(sm.ctx, 30*time.Second)
	defer cancel()

	// Use scheduler for timezone-weighted customer selection
	// This implements "follow the sun" by favoring customers in active timezones
	customer, err := sm.scheduler.SelectCustomer(ctx, rng)
	if err != nil {
		return nil, fmt.Errorf("failed to get customer: %w", err)
	}

	// Use scheduler to get recommended session type (time-aware)
	// Session type varies based on time of day (more ATM at lunch, etc.)
	sessionType := sm.scheduler.GetRecommendedSessionType(customer, rng)

	// Get customer's accounts
	accounts, err := sm.queries.GetCustomerAccounts(ctx, customer.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts: %w", err)
	}
	if len(accounts) == 0 {
		return nil, fmt.Errorf("customer has no active accounts")
	}

	// Generate session ID
	sessionID := fmt.Sprintf("SIM-%d-%d-%d", workerID, time.Now().UnixNano(), customer.ID)

	session := &CustomerSession{
		ID:          sessionID,
		WorkerID:    workerID,
		Customer:    customer,
		Accounts:    accounts,
		Type:        sessionType,
		State:       StateInitialized,
		StartTime:   time.Now(),
		rng:         rng,
		queries:     sm.queries,
		config:      sm.config,
		metrics:     sm.metrics,
		errorSim:    sm.errorSim,
		auditWriter: sm.auditWriter,
		ctx:         sm.ctx,
	}

	// Store session
	sm.sessions.Store(sessionID, session)

	return session, nil
}

// executeSession runs the session through its workflow
func (sm *SessionManager) executeSession(session *CustomerSession) {
	defer func() {
		sm.sessions.Delete(session.ID)
		session.State = StateEnded
		sm.metrics.RecordSessionComplete(session.Type)
	}()

	// Use scheduler to check if customer should be active right now
	// This applies timezone, intraday patterns, and customer segment factors
	if !sm.scheduler.ShouldExecuteSession(session.Customer, session.rng) {
		// Customer not active at this time - apply appropriate pacing delay
		pacing := sm.scheduler.GetSessionPacing(session.Customer)
		select {
		case <-time.After(pacing):
		case <-sm.ctx.Done():
		}
		return
	}

	// Authenticate (login/PIN verification)
	if !session.Authenticate() {
		return
	}

	// Execute session-specific workflow
	switch session.Type {
	case SessionTypeATM:
		session.RunATMWorkflow()
	case SessionTypeOnline:
		session.RunOnlineWorkflow()
	case SessionTypeBusiness:
		session.RunBusinessWorkflow()
	}
}

// thinkTime waits for a realistic delay between actions
func (sm *SessionManager) thinkTime(rng *utils.Random) {
	minMs := sm.config.MinThinkTime.Milliseconds()
	maxMs := sm.config.MaxThinkTime.Milliseconds()

	delayMs := minMs + rng.Int64N(maxMs-minMs+1)
	delay := time.Duration(delayMs) * time.Millisecond

	select {
	case <-time.After(delay):
	case <-sm.ctx.Done():
	}
}

// reportMetrics periodically logs metrics
func (sm *SessionManager) reportMetrics() {
	ticker := time.NewTicker(sm.config.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			stats := sm.metrics.Snapshot()
			globalActivity := sm.scheduler.GetGlobalActivitySummary()

			// Check for active bursts
			activeBursts := sm.burstMgr.GetActiveBursts()
			burstInfo := ""
			if len(activeBursts) > 0 {
				burstInfo = fmt.Sprintf(" | BURST: %s", activeBursts[0].Type)
				if len(activeBursts) > 1 {
					burstInfo += fmt.Sprintf("+%d", len(activeBursts)-1)
				}
			}

			// Load control status
			loadStatus := ""
			if sm.config.EnableRamp {
				loadStatus = fmt.Sprintf(" | Load: %s", sm.loadCtrl.StatusString())
			}

			fmt.Printf("[%s] Activity: %-12s | Sessions: %d | TPS: %.1f (recent: %.1f) | Errors: %d | Latency: avg=%s p95=%s%s%s\n",
				time.Now().Format("15:04:05"),
				globalActivity,
				sm.countActiveSessions(),
				stats.TPS,
				stats.RecentTPS,
				stats.TotalErrors,
				stats.AvgLatency.Round(time.Microsecond),
				stats.P95Latency.Round(time.Microsecond),
				burstInfo,
				loadStatus,
			)
		case <-sm.ctx.Done():
			return
		}
	}
}

// countActiveSessions counts currently running sessions
func (sm *SessionManager) countActiveSessions() int {
	count := 0
	sm.sessions.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// printFinalStats outputs final simulation statistics
func (sm *SessionManager) printFinalStats() {
	stats := sm.metrics.Snapshot()
	schedStats := sm.scheduler.GetStats()
	burstStats := sm.burstMgr.GetStats()
	auditStats := sm.auditWriter.GetStats()

	fmt.Println("\n=== Simulation Complete ===")
	fmt.Printf("Uptime:             %s\n", stats.Uptime.Round(time.Second))
	fmt.Printf("Total Sessions:     %d\n", stats.TotalSessions)
	fmt.Printf("Total Operations:   %d\n", stats.TotalOperations)
	fmt.Printf("Total Errors:       %d (%.2f/sec)\n", stats.TotalErrors, stats.ErrorRate)
	fmt.Printf("Average TPS:        %.2f\n", stats.TPS)
	fmt.Printf("Read Operations:    %d\n", stats.ReadOps)
	fmt.Printf("Write Operations:   %d\n", stats.WriteOps)

	fmt.Println("\n--- Latency Statistics ---")
	fmt.Printf("Average:            %s\n", stats.AvgLatency.Round(time.Microsecond))
	fmt.Printf("P50:                %s\n", stats.P50Latency.Round(time.Microsecond))
	fmt.Printf("P95:                %s\n", stats.P95Latency.Round(time.Microsecond))
	fmt.Printf("P99:                %s\n", stats.P99Latency.Round(time.Microsecond))

	// Per-operation stats
	fmt.Println("\n--- Operation Statistics ---")
	for opType, stat := range stats.OperationStats {
		if stat.Count > 0 {
			fmt.Printf("  %-18s: count=%-8d avg=%-10s p95=%s\n",
				opType, stat.Count, stat.AvgLatency.Round(time.Microsecond), stat.P95Latency.Round(time.Microsecond))
		}
	}

	// Session type stats
	fmt.Println("\n--- Session Types ---")
	for st, count := range stats.SessionStats {
		if count > 0 {
			fmt.Printf("  %-15s: %d\n", st.String(), count)
		}
	}

	// Error breakdown
	if len(stats.ErrorStats) > 0 {
		fmt.Println("\n--- Error Breakdown ---")
		for errType, count := range stats.ErrorStats {
			if count > 0 {
				fmt.Printf("  %-15s: %d\n", errType, count)
			}
		}

		// Retry stats
		retryStats := sm.errorSim.GetRetryStats()
		if retryStats.TotalRetries > 0 {
			fmt.Printf("\nRetries:            %d total, %d successful, %d exhausted\n",
				retryStats.TotalRetries, retryStats.SuccessfulRetries, retryStats.ExhaustedRetries)
		}
	}

	fmt.Println("\n--- Scheduler Statistics ---")
	fmt.Printf("Sessions Scheduled: %d\n", schedStats.TotalScheduled)
	fmt.Printf("Bursts Triggered:   %d\n", schedStats.BurstTriggered)
	fmt.Printf("Skipped (inactive): %d\n", schedStats.SkippedInactive)

	fmt.Println("\n--- Burst Statistics ---")
	fmt.Printf("Total Bursts:       %d\n", burstStats.TotalBurstsTriggered)
	if burstStats.TotalBurstsTriggered > 0 {
		for burstType, count := range burstStats.BurstsByType {
			fmt.Printf("  %-15s: %d\n", burstType, count)
		}
		fmt.Printf("Extra Sessions:     %d\n", burstStats.TotalExtraSessions)
		if !burstStats.LastBurstTime.IsZero() {
			fmt.Printf("Last Burst:         %s\n", burstStats.LastBurstTime.Format("2006-01-02 15:04:05"))
		}
	}

	// Load control stats if enabled
	if sm.config.EnableRamp {
		loadStats := sm.loadCtrl.GetStats()
		fmt.Println("\n--- Load Control Statistics ---")
		if !loadStats.RampUpStartTime.IsZero() {
			fmt.Printf("Ramp-up duration:   %s\n", loadStats.RampUpEndTime.Sub(loadStats.RampUpStartTime).Round(time.Second))
		}
		if !loadStats.SteadyStateStart.IsZero() {
			fmt.Printf("Steady state time:  %s\n", loadStats.TimeInSteadyState.Round(time.Second))
		}
		fmt.Printf("Max load reached:   %d sessions\n", loadStats.MaxLoadReached)
	}

	// Audit writer stats
	fmt.Println("\n--- Audit Trail Statistics ---")
	fmt.Printf("Logs Received:      %d\n", auditStats.LogsReceived)
	fmt.Printf("Logs Written:       %d\n", auditStats.LogsWritten)
	fmt.Printf("Batches Written:    %d (avg size: %d)\n", auditStats.BatchesWritten, auditStats.AvgBatchSize)
	if auditStats.WriteErrors > 0 {
		fmt.Printf("Write Errors:       %d\n", auditStats.WriteErrors)
	}
	if auditStats.DroppedLogs > 0 {
		fmt.Printf("Dropped Logs:       %d\n", auditStats.DroppedLogs)
	}

	// Show top 5 timezones by activity
	if len(schedStats.ScheduledByTimezone) > 0 {
		fmt.Println("\nTop Timezones by Session Count:")
		// Simple display (full sorting would require more code)
		count := 0
		for tz, sessions := range schedStats.ScheduledByTimezone {
			if count >= 5 {
				break
			}
			fmt.Printf("  %-25s: %d sessions\n", tz, sessions)
			count++
		}
	}
}

// Legacy Metrics struct removed - now using EnhancedMetrics from metrics.go
