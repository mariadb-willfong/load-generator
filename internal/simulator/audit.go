package simulator

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/willfong/load-generator/internal/database"
	"github.com/willfong/load-generator/internal/models"
)

// AuditWriter provides buffered, async writing of audit logs
type AuditWriter struct {
	pool *database.Pool

	// Buffered channel for incoming audit logs
	buffer chan *models.AuditLog

	// Configuration
	batchSize    int
	flushInterval time.Duration
	workers      int

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Statistics
	stats AuditStats
}

// AuditStats tracks audit writing statistics
type AuditStats struct {
	logsReceived    atomic.Int64
	logsWritten     atomic.Int64
	batchesWritten  atomic.Int64
	writeErrors     atomic.Int64
	droppedLogs     atomic.Int64
	lastFlushTime   atomic.Value // time.Time
	avgBatchSize    atomic.Int64
	totalBatches    atomic.Int64
}

// AuditWriterConfig holds configuration for the audit writer
type AuditWriterConfig struct {
	BufferSize    int           // Size of the audit log buffer
	BatchSize     int           // Max logs per batch insert
	FlushInterval time.Duration // How often to flush incomplete batches
	Workers       int           // Number of concurrent write workers
}

// DefaultAuditWriterConfig returns sensible defaults
func DefaultAuditWriterConfig() AuditWriterConfig {
	return AuditWriterConfig{
		BufferSize:    10000,
		BatchSize:     100,
		FlushInterval: 500 * time.Millisecond,
		Workers:       2,
	}
}

// NewAuditWriter creates a new buffered audit writer
func NewAuditWriter(pool *database.Pool, cfg AuditWriterConfig) *AuditWriter {
	ctx, cancel := context.WithCancel(context.Background())

	aw := &AuditWriter{
		pool:          pool,
		buffer:        make(chan *models.AuditLog, cfg.BufferSize),
		batchSize:     cfg.BatchSize,
		flushInterval: cfg.FlushInterval,
		workers:       cfg.Workers,
		ctx:           ctx,
		cancel:        cancel,
	}

	aw.stats.lastFlushTime.Store(time.Now())

	return aw
}

// Start begins the background write workers
func (aw *AuditWriter) Start() {
	for i := 0; i < aw.workers; i++ {
		aw.wg.Add(1)
		go aw.writeWorker(i)
	}
}

// Write queues an audit log for async writing
// Returns immediately; the log will be written in the background
func (aw *AuditWriter) Write(log *models.AuditLog) {
	aw.stats.logsReceived.Add(1)

	select {
	case aw.buffer <- log:
		// Successfully queued
	default:
		// Buffer full - drop the log and record it
		aw.stats.droppedLogs.Add(1)
	}
}

// writeWorker processes logs from the buffer and writes them in batches
func (aw *AuditWriter) writeWorker(workerID int) {
	defer aw.wg.Done()

	batch := make([]*models.AuditLog, 0, aw.batchSize)
	ticker := time.NewTicker(aw.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case log := <-aw.buffer:
			batch = append(batch, log)
			if len(batch) >= aw.batchSize {
				aw.writeBatch(batch)
				batch = batch[:0]
			}

		case <-ticker.C:
			// Periodic flush of incomplete batches
			if len(batch) > 0 {
				aw.writeBatch(batch)
				batch = batch[:0]
			}

		case <-aw.ctx.Done():
			// Drain remaining buffer on shutdown
			aw.drainBuffer(batch)
			return
		}
	}
}

// drainBuffer writes all remaining logs during shutdown
func (aw *AuditWriter) drainBuffer(currentBatch []*models.AuditLog) {
	// First write any current batch
	if len(currentBatch) > 0 {
		aw.writeBatch(currentBatch)
	}

	// Then drain the channel
	batch := make([]*models.AuditLog, 0, aw.batchSize)
	for {
		select {
		case log := <-aw.buffer:
			batch = append(batch, log)
			if len(batch) >= aw.batchSize {
				aw.writeBatch(batch)
				batch = batch[:0]
			}
		default:
			// Channel empty
			if len(batch) > 0 {
				aw.writeBatch(batch)
			}
			return
		}
	}
}

// writeBatch performs a bulk insert of audit logs
func (aw *AuditWriter) writeBatch(batch []*models.AuditLog) {
	if len(batch) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := aw.batchInsert(ctx, batch)
	if err != nil {
		aw.stats.writeErrors.Add(1)
		// Optionally log the error, but don't block
		// Individual log insert fallback could be added here
		return
	}

	aw.stats.logsWritten.Add(int64(len(batch)))
	aw.stats.batchesWritten.Add(1)
	aw.stats.lastFlushTime.Store(time.Now())

	// Update average batch size
	totalBatches := aw.stats.totalBatches.Add(1)
	currentAvg := aw.stats.avgBatchSize.Load()
	newAvg := currentAvg + (int64(len(batch))-currentAvg)/totalBatches
	aw.stats.avgBatchSize.Store(newAvg)
}

// batchInsert performs a multi-row insert for efficiency
func (aw *AuditWriter) batchInsert(ctx context.Context, logs []*models.AuditLog) error {
	if len(logs) == 0 {
		return nil
	}

	// Build the INSERT statement with multiple VALUES
	query := `INSERT INTO audit_logs (
		timestamp, customer_id, session_id, action, outcome,
		channel, branch_id, atm_id, ip_address, user_agent,
		account_id, transaction_id, beneficiary_id,
		description, failure_reason, metadata, risk_score, request_id
	) VALUES `

	// Prepare placeholders and args
	valueStrings := make([]string, 0, len(logs))
	valueArgs := make([]interface{}, 0, len(logs)*18)

	for _, log := range logs {
		valueStrings = append(valueStrings, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")
		valueArgs = append(valueArgs,
			log.Timestamp, log.CustomerID, log.SessionID, log.Action, log.Outcome,
			log.Channel, log.BranchID, log.ATMID, log.IPAddress, log.UserAgent,
			log.AccountID, log.TransactionID, log.BeneficiaryID,
			log.Description, log.FailureReason, log.Metadata, log.RiskScore, log.RequestID,
		)
	}

	// Join all value strings
	for i, vs := range valueStrings {
		if i > 0 {
			query += ", "
		}
		query += vs
	}

	_, err := aw.pool.ExecContext(ctx, query, valueArgs...)
	return err
}

// Stop gracefully shuts down the audit writer
func (aw *AuditWriter) Stop() error {
	// Signal workers to stop
	aw.cancel()

	// Wait for workers with timeout
	done := make(chan struct{})
	go func() {
		aw.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("audit writer shutdown timed out")
	}
}

// GetStats returns current audit writing statistics
func (aw *AuditWriter) GetStats() AuditStatsSnapshot {
	lastFlush, _ := aw.stats.lastFlushTime.Load().(time.Time)
	return AuditStatsSnapshot{
		LogsReceived:   aw.stats.logsReceived.Load(),
		LogsWritten:    aw.stats.logsWritten.Load(),
		BatchesWritten: aw.stats.batchesWritten.Load(),
		WriteErrors:    aw.stats.writeErrors.Load(),
		DroppedLogs:    aw.stats.droppedLogs.Load(),
		BufferSize:     len(aw.buffer),
		BufferCapacity: cap(aw.buffer),
		LastFlushTime:  lastFlush,
		AvgBatchSize:   aw.stats.avgBatchSize.Load(),
	}
}

// AuditStatsSnapshot is a point-in-time view of audit stats
type AuditStatsSnapshot struct {
	LogsReceived   int64
	LogsWritten    int64
	BatchesWritten int64
	WriteErrors    int64
	DroppedLogs    int64
	BufferSize     int
	BufferCapacity int
	LastFlushTime  time.Time
	AvgBatchSize   int64
}

// Pending returns the number of logs waiting to be written
func (s AuditStatsSnapshot) Pending() int64 {
	return s.LogsReceived - s.LogsWritten - s.DroppedLogs
}

// WriteRate returns logs written per second (requires start time)
func (s AuditStatsSnapshot) WriteRate(startTime time.Time) float64 {
	elapsed := time.Since(startTime).Seconds()
	if elapsed < 1 {
		return 0
	}
	return float64(s.LogsWritten) / elapsed
}

// PrintStats displays audit writer statistics
func (aw *AuditWriter) PrintStats(startTime time.Time) {
	stats := aw.GetStats()
	fmt.Println("\n--- Audit Writer Stats ---")
	fmt.Printf("Logs Received:  %d\n", stats.LogsReceived)
	fmt.Printf("Logs Written:   %d\n", stats.LogsWritten)
	fmt.Printf("Batches:        %d (avg size: %d)\n", stats.BatchesWritten, stats.AvgBatchSize)
	fmt.Printf("Write Rate:     %.1f logs/sec\n", stats.WriteRate(startTime))
	fmt.Printf("Errors:         %d\n", stats.WriteErrors)
	fmt.Printf("Dropped:        %d\n", stats.DroppedLogs)
	fmt.Printf("Buffer:         %d/%d\n", stats.BufferSize, stats.BufferCapacity)
	if !stats.LastFlushTime.IsZero() {
		fmt.Printf("Last Flush:     %s ago\n", time.Since(stats.LastFlushTime).Round(time.Second))
	}
}

// AuditAction creates and queues an audit log
func (aw *AuditWriter) AuditAction(
	customer *models.Customer,
	sessionID string,
	action models.AuditAction,
	outcome models.AuditOutcome,
	channel models.AuditChannel,
	opts ...AuditOption,
) {
	log := &models.AuditLog{
		Timestamp:  time.Now(),
		SessionID:  sessionID,
		Action:     action,
		Outcome:    outcome,
		Channel:    channel,
	}

	if customer != nil {
		log.CustomerID = &customer.ID
	}

	// Apply options
	for _, opt := range opts {
		opt(log)
	}

	aw.Write(log)
}

// AuditOption is a functional option for audit log creation
type AuditOption func(*models.AuditLog)

// WithAccount sets the account ID on the audit log
func WithAccount(accountID int64) AuditOption {
	return func(log *models.AuditLog) {
		log.AccountID = &accountID
	}
}

// WithTransaction sets the transaction ID on the audit log
func WithTransaction(txnID int64) AuditOption {
	return func(log *models.AuditLog) {
		log.TransactionID = &txnID
	}
}

// WithATM sets the ATM ID on the audit log
func WithATM(atmID int64) AuditOption {
	return func(log *models.AuditLog) {
		log.ATMID = &atmID
	}
}

// WithBranch sets the branch ID on the audit log
func WithBranch(branchID int64) AuditOption {
	return func(log *models.AuditLog) {
		log.BranchID = &branchID
	}
}

// WithFailureReason sets the failure reason on the audit log
func WithFailureReason(reason string) AuditOption {
	return func(log *models.AuditLog) {
		log.FailureReason = reason
	}
}

// WithDescription sets the description on the audit log
func WithDescription(desc string) AuditOption {
	return func(log *models.AuditLog) {
		log.Description = desc
	}
}

// WithIP sets the IP address on the audit log
func WithIP(ip string) AuditOption {
	return func(log *models.AuditLog) {
		log.IPAddress = ip
	}
}

// WithUserAgent sets the user agent on the audit log
func WithUserAgent(ua string) AuditOption {
	return func(log *models.AuditLog) {
		log.UserAgent = ua
	}
}

// WithRiskScore sets the risk score on the audit log
func WithRiskScore(score float64) AuditOption {
	return func(log *models.AuditLog) {
		log.RiskScore = &score
	}
}

// WithBeneficiary sets the beneficiary ID on the audit log
func WithBeneficiary(beneficiaryID int64) AuditOption {
	return func(log *models.AuditLog) {
		log.BeneficiaryID = &beneficiaryID
	}
}
