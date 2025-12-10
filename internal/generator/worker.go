package generator

import (
	"runtime"
	"sort"
	"time"

	"github.com/willfong/load-generator/internal/utils"
)

// WorkerConfig contains configuration for a single generation worker
type WorkerConfig struct {
	WorkerID    int                // 0-indexed worker ID
	WorkerCount int                // Total number of workers
	Accounts    []GeneratedAccount // Accounts assigned to this worker
	StartID     int64              // First transaction ID for this worker
	EndID       int64              // Last transaction ID (exclusive, for validation)
	RNG         *utils.Random      // Forked RNG for this worker
	OutputDir   string
	Compress    bool
	Progress    chan<- int64 // Channel to report progress (count increments)
}

// WorkerResult contains results from a completed worker
type WorkerResult struct {
	WorkerID         int
	TransactionCount int64
	AuditLogCount    int64
	Duration         time.Duration
	Error            error
	ShardFile        string // Path to the shard file created
}

// IDRange represents a pre-allocated range of IDs for a worker
type IDRange struct {
	Start int64 // First ID (inclusive)
	End   int64 // Last ID (exclusive)
}

// GetWorkerCount returns the number of workers to use.
// If configured workers is 0, auto-detects using runtime.NumCPU().
func GetWorkerCount(configured int) int {
	if configured > 0 {
		return configured
	}
	cpus := runtime.NumCPU()
	if cpus < 1 {
		return 1
	}
	return cpus
}

// PartitionAccountsByCustomer groups accounts by customer, then distributes
// customer groups across workers for balanced load. This ensures all accounts
// for a single customer stay with the same worker, which is important for
// internal transfers and balance tracking.
func PartitionAccountsByCustomer(accounts []GeneratedAccount, workerCount int) [][]GeneratedAccount {
	if workerCount <= 0 {
		workerCount = 1
	}
	if len(accounts) == 0 {
		return make([][]GeneratedAccount, workerCount)
	}

	// Step 1: Group accounts by customer ID
	customerAccounts := make(map[int64][]GeneratedAccount)
	for _, acc := range accounts {
		custID := acc.Account.CustomerID
		customerAccounts[custID] = append(customerAccounts[custID], acc)
	}

	// Step 2: Get sorted customer IDs for deterministic assignment
	customerIDs := make([]int64, 0, len(customerAccounts))
	for id := range customerAccounts {
		customerIDs = append(customerIDs, id)
	}
	sort.Slice(customerIDs, func(i, j int) bool {
		return customerIDs[i] < customerIDs[j]
	})

	// Step 3: Assign customers to workers (round-robin for balance)
	workerAccounts := make([][]GeneratedAccount, workerCount)
	for i := range workerAccounts {
		workerAccounts[i] = make([]GeneratedAccount, 0)
	}

	for i, custID := range customerIDs {
		workerIdx := i % workerCount
		workerAccounts[workerIdx] = append(workerAccounts[workerIdx], customerAccounts[custID]...)
	}

	return workerAccounts
}

// EstimateTransactionCount estimates the total number of transactions that will
// be generated based on account count, years of history, and transactions per month.
// Includes a buffer for counterparty transactions (internal transfers).
func EstimateTransactionCount(accountCount int, yearsOfHistory int, txnsPerCustomerPerMonth int) int64 {
	months := yearsOfHistory * 12
	// Each account generates approximately txnsPerCustomerPerMonth transactions
	// Add 50% buffer for counterparty transactions from internal transfers
	baseCount := int64(accountCount) * int64(txnsPerCustomerPerMonth) * int64(months)
	return int64(float64(baseCount) * 1.5)
}

// CalculateIDRanges pre-allocates non-overlapping ID ranges for each worker.
// Each worker gets a contiguous block of IDs to use, ensuring no coordination
// is needed during generation.
func CalculateIDRanges(estimatedTotal int64, workerCount int) []IDRange {
	if workerCount <= 0 {
		workerCount = 1
	}
	if estimatedTotal <= 0 {
		estimatedTotal = 1000000 // Default estimate
	}

	// Add 20% buffer for safety (in case estimation is low)
	bufferedTotal := int64(float64(estimatedTotal) * 1.2)
	rangeSize := bufferedTotal / int64(workerCount)

	// Ensure minimum range size
	if rangeSize < 10000 {
		rangeSize = 10000
	}

	ranges := make([]IDRange, workerCount)
	for i := 0; i < workerCount; i++ {
		ranges[i] = IDRange{
			Start: 1 + int64(i)*rangeSize,
			End:   1 + int64(i+1)*rangeSize,
		}
	}

	// Last worker gets extra buffer for any overflow
	ranges[workerCount-1].End = bufferedTotal + rangeSize

	return ranges
}

// EstimateAuditLogCount estimates the total number of audit log entries
// based on transaction count. Audit logs include login events, balance checks,
// and transaction-related events.
func EstimateAuditLogCount(transactionCount int64, customerCount int, yearsOfHistory int) int64 {
	months := yearsOfHistory * 12
	// Estimate: ~3 sessions per customer per month, ~4 events per session
	sessionEvents := int64(customerCount) * int64(months) * 3 * 4
	// Plus transaction-related audit events (roughly 1 per transaction)
	return sessionEvents + transactionCount
}
