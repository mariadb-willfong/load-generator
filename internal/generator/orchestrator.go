package generator

import (
	"fmt"
	"sync"
	"time"

	"github.com/willfong/load-generator/internal/data"
	"github.com/willfong/load-generator/internal/utils"
)

// Orchestrator coordinates all entity generators for bulk data generation.
type Orchestrator struct {
	rng     *utils.Random
	refData *data.ReferenceData
	config  OrchestratorConfig
	verbose bool
	showProgress bool

	// Stored data from entity generation (used for transaction generation)
	branches   []GeneratedBranch
	atms       []GeneratedATM
	customers  []GeneratedCustomer
	businesses []GeneratedBusiness
	accounts   []GeneratedAccount
}

// OrchestratorConfig holds settings for the orchestrator
type OrchestratorConfig struct {
	NumCustomers  int
	NumBusinesses int
	NumBranches   int
	NumATMs       int
	YearsOfHistory int
	OutputDir     string
	Seed          int64

	// Transaction generation settings
	TransactionsPerCustomerPerMonth int
	PayrollDay                      int     // Day of month for payroll (1-31)
	ParetoRatio                     float64 // 0.2 = 20% accounts generate 80% volume
	DeclinedTransactionRate         float64 // 0.0-1.0
	InsufficientFundsRate           float64 // 0.0-1.0

	// Audit log generation settings
	FailedLoginRate                float64 // Rate of failed login attempts (0.0-1.0)
	SessionsPerCustomerPerMonth    int     // Average login sessions per customer per month
	BalanceChecksPerSession        int     // Average balance inquiries per session

	// Performance settings
	Parallel bool // Enable parallel CSV writing for independent tables
	Workers  int  // Number of parallel workers (0 = auto-detect CPUs)

	// Output settings
	Compress bool // Enable xz compression (creates .csv.xz files)
}

// GenerationResult holds statistics from the generation run
type GenerationResult struct {
	BranchCount      int
	ATMCount         int
	CustomerCount    int
	BusinessCount    int
	AccountCount     int
	BeneficiaryCount int
	TransactionCount int
	AuditLogCount    int
	Duration         time.Duration
}

// OrchestratorOptions holds optional settings for the orchestrator
type OrchestratorOptions struct {
	Verbose      bool
	ShowProgress bool
}

// NewOrchestrator creates a new orchestrator
func NewOrchestrator(config OrchestratorConfig, opts OrchestratorOptions) (*Orchestrator, error) {
	// Load reference data
	refData, err := data.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load reference data: %w", err)
	}

	// Create RNG with seed
	rng := utils.NewRandom(config.Seed)

	return &Orchestrator{
		rng:          rng,
		refData:      refData,
		config:       config,
		verbose:      opts.Verbose,
		showProgress: opts.ShowProgress,
	}, nil
}

// GenerateEntities generates all static entities (no transactions)
func (o *Orchestrator) GenerateEntities() (*GenerationResult, error) {
	startTime := time.Now()
	result := &GenerationResult{}

	// 1. Generate branches
	o.log("Generating %d branches...", o.config.NumBranches)
	branchGen := NewBranchGenerator(o.rng.Fork(), o.refData, BranchGeneratorConfig{
		NumBranches: o.config.NumBranches,
		NumATMs:     o.config.NumATMs,
		BaseDate:    time.Now(),
		YearsBack:   o.config.YearsOfHistory,
	})

	branches := branchGen.GenerateBranches()
	o.branches = branches
	result.BranchCount = len(branches)
	o.log("  Generated %d branches", result.BranchCount)

	// Write branches CSV
	if o.showProgress {
		if err := WriteBranchesCSVWithProgress(branches, o.config.OutputDir, o.config.Compress); err != nil {
			return nil, fmt.Errorf("failed to write branches CSV: %w", err)
		}
	} else {
		if err := WriteBranchesCSV(branches, o.config.OutputDir, o.config.Compress); err != nil {
			return nil, fmt.Errorf("failed to write branches CSV: %w", err)
		}
		o.log("  Wrote branches.csv")
	}

	// 2. Generate ATMs
	o.log("Generating %d ATMs...", o.config.NumATMs)
	atms := branchGen.GenerateATMs(branches)
	o.atms = atms
	result.ATMCount = len(atms)
	o.log("  Generated %d ATMs", result.ATMCount)

	// Write ATMs CSV
	if o.showProgress {
		if err := WriteATMsCSVWithProgress(atms, o.config.OutputDir, o.config.Compress); err != nil {
			return nil, fmt.Errorf("failed to write ATMs CSV: %w", err)
		}
	} else {
		if err := WriteATMsCSV(atms, o.config.OutputDir, o.config.Compress); err != nil {
			return nil, fmt.Errorf("failed to write ATMs CSV: %w", err)
		}
		o.log("  Wrote atms.csv")
	}

	// 3. Generate retail customers
	o.log("Generating %d customers...", o.config.NumCustomers)
	customerGen := NewCustomerGenerator(o.rng.Fork(), o.refData, CustomerGeneratorConfig{
		NumCustomers: o.config.NumCustomers,
		Branches:     branches,
		BaseDate:     time.Now(),
		ParetoRatio:  0.2,
	})

	customers := customerGen.GenerateCustomers()
	o.customers = customers
	result.CustomerCount = len(customers)
	o.log("  Generated %d customers", result.CustomerCount)

	// Write customers CSV
	if o.showProgress {
		if err := WriteCustomersCSVWithProgress(customers, o.config.OutputDir, o.config.Compress); err != nil {
			return nil, fmt.Errorf("failed to write customers CSV: %w", err)
		}
	} else {
		if err := WriteCustomersCSV(customers, o.config.OutputDir, o.config.Compress); err != nil {
			return nil, fmt.Errorf("failed to write customers CSV: %w", err)
		}
		o.log("  Wrote customers.csv")
	}

	// 4. Generate businesses
	o.log("Generating %d businesses...", o.config.NumBusinesses)
	businessStartID := int64(o.config.NumCustomers + 1)
	businessGen := NewBusinessGenerator(o.rng.Fork(), o.refData, BusinessGeneratorConfig{
		NumBusinesses: o.config.NumBusinesses,
		StartID:       businessStartID,
		Branches:      branches,
	})

	businesses := businessGen.GenerateBusinesses()
	o.businesses = businesses
	result.BusinessCount = len(businesses)
	o.log("  Generated %d businesses", result.BusinessCount)

	// Write businesses CSV
	if o.showProgress {
		if err := WriteBusinessesCSVWithProgress(businesses, o.config.OutputDir, o.config.Compress); err != nil {
			return nil, fmt.Errorf("failed to write businesses CSV: %w", err)
		}
	} else {
		if err := WriteBusinessesCSV(businesses, o.config.OutputDir, o.config.Compress); err != nil {
			return nil, fmt.Errorf("failed to write businesses CSV: %w", err)
		}
		o.log("  Wrote businesses.csv")
	}

	// 5. Generate accounts for customers
	o.log("Generating accounts for customers...")
	accountGen := NewAccountGenerator(o.rng.Fork(), o.refData, AccountGeneratorConfig{
		Branches: branches,
	})

	customerAccounts, nextAccountID := accountGen.GenerateAccountsForCustomers(customers, 1)
	o.log("  Generated %d customer accounts", len(customerAccounts))

	// Generate accounts for businesses
	o.log("Generating accounts for businesses...")
	businessAccounts, _ := accountGen.GenerateAccountsForBusinesses(businesses, nextAccountID)
	o.log("  Generated %d business accounts", len(businessAccounts))

	// Combine all accounts
	allAccounts := append(customerAccounts, businessAccounts...)
	o.accounts = allAccounts
	result.AccountCount = len(allAccounts)

	// Write accounts CSV
	if o.showProgress {
		if err := WriteAccountsCSVWithProgress(allAccounts, o.config.OutputDir, o.config.Compress); err != nil {
			return nil, fmt.Errorf("failed to write accounts CSV: %w", err)
		}
	} else {
		if err := WriteAccountsCSV(allAccounts, o.config.OutputDir, o.config.Compress); err != nil {
			return nil, fmt.Errorf("failed to write accounts CSV: %w", err)
		}
		o.log("  Wrote accounts.csv")
	}

	// 6. Generate beneficiaries
	o.log("Generating beneficiaries...")
	beneficiaryGen := NewBeneficiaryGenerator(o.rng.Fork(), o.refData, BeneficiaryGeneratorConfig{
		AvgBeneficiariesPerCustomer: 5,
		Businesses:                  businesses,
	})

	beneficiaries, _ := beneficiaryGen.GenerateBeneficiariesForCustomers(customers, 1)
	result.BeneficiaryCount = len(beneficiaries)
	o.log("  Generated %d beneficiaries", result.BeneficiaryCount)

	// Write beneficiaries CSV
	if o.showProgress {
		if err := WriteBeneficiariesCSVWithProgress(beneficiaries, o.config.OutputDir, o.config.Compress); err != nil {
			return nil, fmt.Errorf("failed to write beneficiaries CSV: %w", err)
		}
	} else {
		if err := WriteBeneficiariesCSV(beneficiaries, o.config.OutputDir, o.config.Compress); err != nil {
			return nil, fmt.Errorf("failed to write beneficiaries CSV: %w", err)
		}
		o.log("  Wrote beneficiaries.csv")
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// GenerateTransactions generates historical transactions using parallel streaming.
// Must be called after GenerateEntities.
func (o *Orchestrator) GenerateTransactions() (*GenerationResult, error) {
	if len(o.accounts) == 0 {
		return nil, fmt.Errorf("no accounts found - call GenerateEntities first")
	}

	startTime := time.Now()
	result := &GenerationResult{}

	// Calculate date range for transaction history
	endDate := time.Now()
	startDate := endDate.AddDate(-o.config.YearsOfHistory, 0, 0)

	// Determine worker count
	workerCount := GetWorkerCount(o.config.Workers)

	fmt.Printf("Generating transactions for %d years using %d workers...\n",
		o.config.YearsOfHistory, workerCount)

	// Set defaults if not configured
	txnsPerMonth := o.config.TransactionsPerCustomerPerMonth
	if txnsPerMonth <= 0 {
		txnsPerMonth = 15
	}
	paretoRatio := o.config.ParetoRatio
	if paretoRatio <= 0 {
		paretoRatio = 0.2
	}

	// Partition accounts by customer across workers
	workerAccounts := PartitionAccountsByCustomer(o.accounts, workerCount)

	// Estimate total transactions for progress reporting and ID allocation
	estimatedTotal := EstimateTransactionCount(len(o.accounts), o.config.YearsOfHistory, txnsPerMonth)
	idRanges := CalculateIDRanges(estimatedTotal, workerCount)

	// Fork RNGs for each worker
	workerRNGs := o.rng.ForkN(workerCount)

	// Create progress reporter
	var progress *AggregatedProgressReporter
	if o.showProgress {
		progress = NewAggregatedProgressReporter(AggregatedProgressConfig{
			Total:       estimatedTotal,
			Label:       "  Transactions",
			WorkerCount: workerCount,
		})
		progress.Start()
	}

	// Launch workers
	var wg sync.WaitGroup
	results := make([]WorkerResult, workerCount)
	errChan := make(chan error, workerCount)

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			var progressChan chan<- workerProgress
			if progress != nil {
				progressChan = progress.GetProgressChan()
			}

			gen, err := NewStreamingTransactionGenerator(workerRNGs[workerID], o.refData, StreamingTransactionConfig{
				StartDate:                       startDate,
				EndDate:                         endDate,
				TransactionsPerCustomerPerMonth: txnsPerMonth,
				ParetoRatio:                     paretoRatio,
				PayrollDay:                      o.config.PayrollDay,
				DeclinedTransactionRate:         o.config.DeclinedTransactionRate,
				InsufficientFundsRate:           o.config.InsufficientFundsRate,
				Branches:                        o.branches,
				ATMs:                            o.atms,
				AllAccounts:                     o.accounts,
				Businesses:                      o.businesses,
				WorkerID:                        workerID,
				WorkerCount:                     workerCount,
				StartID:                         idRanges[workerID].Start,
				EndID:                           idRanges[workerID].End,
				OutputDir:                       o.config.OutputDir,
				Compress:                        o.config.Compress,
				ProgressChan:                    progressChan,
			})
			if err != nil {
				errChan <- fmt.Errorf("worker %d: failed to create generator: %w", workerID, err)
				return
			}

			workerStart := time.Now()
			count, err := gen.GenerateAndStream(workerAccounts[workerID])
			if err != nil {
				errChan <- fmt.Errorf("worker %d: %w", workerID, err)
				return
			}

			results[workerID] = WorkerResult{
				WorkerID:         workerID,
				TransactionCount: count,
				Duration:         time.Since(workerStart),
				ShardFile:        gen.ShardFile(),
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if progress != nil {
			progress.Finish()
		}
		return nil, err
	}

	// Finish progress
	if progress != nil {
		progress.Finish()
	}

	// Sum up results
	for _, r := range results {
		result.TransactionCount += int(r.TransactionCount)
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// GenerateAuditLogs generates audit trail entries using parallel streaming.
// Must be called after GenerateEntities.
func (o *Orchestrator) GenerateAuditLogs() (*GenerationResult, error) {
	if len(o.customers) == 0 {
		return nil, fmt.Errorf("no customers found - call GenerateEntities first")
	}

	startTime := time.Now()
	result := &GenerationResult{}

	// Calculate date range
	endDate := time.Now()
	startDate := endDate.AddDate(-o.config.YearsOfHistory, 0, 0)

	// Determine worker count
	workerCount := GetWorkerCount(o.config.Workers)

	fmt.Printf("Generating audit logs using %d workers...\n", workerCount)

	// Set defaults if not configured
	sessionsPerMonth := o.config.SessionsPerCustomerPerMonth
	if sessionsPerMonth <= 0 {
		sessionsPerMonth = 3
	}
	balanceChecks := o.config.BalanceChecksPerSession
	if balanceChecks <= 0 {
		balanceChecks = 2
	}
	failedLoginRate := o.config.FailedLoginRate
	if failedLoginRate <= 0 {
		failedLoginRate = 0.03
	}

	// Partition customers across workers
	customersPerWorker := len(o.customers) / workerCount
	if customersPerWorker < 1 {
		customersPerWorker = 1
	}

	// Estimate total audit logs for progress and ID allocation
	estimatedTotal := EstimateAuditLogCount(0, len(o.customers), o.config.YearsOfHistory)
	idRanges := CalculateIDRanges(estimatedTotal, workerCount)

	// Fork RNGs for each worker
	workerRNGs := o.rng.ForkN(workerCount)

	// Create progress reporter
	var progress *AggregatedProgressReporter
	if o.showProgress {
		progress = NewAggregatedProgressReporter(AggregatedProgressConfig{
			Total:       estimatedTotal,
			Label:       "  Audit logs",
			WorkerCount: workerCount,
		})
		progress.Start()
	}

	// Launch workers
	var wg sync.WaitGroup
	results := make([]WorkerResult, workerCount)
	errChan := make(chan error, workerCount)

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			// Determine customer range for this worker
			start := workerID * customersPerWorker
			end := start + customersPerWorker
			if workerID == workerCount-1 {
				end = len(o.customers) // Last worker takes remainder
			}
			if start >= len(o.customers) {
				return // No customers for this worker
			}
			workerCustomers := o.customers[start:end]

			var progressChan chan<- workerProgress
			if progress != nil {
				progressChan = progress.GetProgressChan()
			}

			gen, err := NewStreamingAuditGenerator(workerRNGs[workerID], o.refData, StreamingAuditConfig{
				Customers:                      workerCustomers,
				Accounts:                       o.accounts,
				ATMs:                           o.atms,
				FailedLoginRate:                failedLoginRate,
				LockedAccountRate:              0.1,
				SessionTimeoutRate:             0.15,
				AvgSessionsPerCustomerPerMonth: sessionsPerMonth,
				AvgBalanceChecksPerSession:     balanceChecks,
				StartDate:                      startDate,
				EndDate:                        endDate,
				WorkerID:                       workerID,
				WorkerCount:                    workerCount,
				StartID:                        idRanges[workerID].Start,
				EndID:                          idRanges[workerID].End,
				OutputDir:                      o.config.OutputDir,
				Compress:                       o.config.Compress,
				ProgressChan:                   progressChan,
			})
			if err != nil {
				errChan <- fmt.Errorf("worker %d: failed to create generator: %w", workerID, err)
				return
			}

			workerStart := time.Now()
			count, err := gen.GenerateAndStream()
			if err != nil {
				errChan <- fmt.Errorf("worker %d: %w", workerID, err)
				return
			}

			results[workerID] = WorkerResult{
				WorkerID:      workerID,
				AuditLogCount: count,
				Duration:      time.Since(workerStart),
				ShardFile:     gen.ShardFile(),
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// Check for errors
	for err := range errChan {
		if progress != nil {
			progress.Finish()
		}
		return nil, err
	}

	// Finish progress
	if progress != nil {
		progress.Finish()
	}

	// Sum up results
	for _, r := range results {
		result.AuditLogCount += int(r.AuditLogCount)
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// GenerateAll generates all entities, transactions, and audit logs in one call.
func (o *Orchestrator) GenerateAll() (*GenerationResult, error) {
	// Generate entities first
	entityResult, err := o.GenerateEntities()
	if err != nil {
		return nil, err
	}

	// Generate transactions
	txnResult, err := o.GenerateTransactions()
	if err != nil {
		return nil, err
	}

	// Generate audit logs
	auditResult, err := o.GenerateAuditLogs()
	if err != nil {
		return nil, err
	}

	// Combine results
	entityResult.TransactionCount = txnResult.TransactionCount
	entityResult.AuditLogCount = auditResult.AuditLogCount
	entityResult.Duration += txnResult.Duration + auditResult.Duration

	return entityResult, nil
}

// log prints a message if verbose mode is enabled
func (o *Orchestrator) log(format string, args ...interface{}) {
	if o.verbose {
		fmt.Printf(format+"\n", args...)
	}
}

// PrintSummary prints a summary of the generation results
func PrintSummary(result *GenerationResult) {
	fmt.Println()
	fmt.Println("=== Generation Complete ===")
	fmt.Printf("Branches:      %d\n", result.BranchCount)
	fmt.Printf("ATMs:          %d\n", result.ATMCount)
	fmt.Printf("Customers:     %d\n", result.CustomerCount)
	fmt.Printf("Businesses:    %d\n", result.BusinessCount)
	fmt.Printf("Accounts:      %d\n", result.AccountCount)
	fmt.Printf("Beneficiaries: %d\n", result.BeneficiaryCount)
	fmt.Printf("Transactions:  %d\n", result.TransactionCount)
	fmt.Printf("Audit Logs:    %d\n", result.AuditLogCount)
	fmt.Printf("Duration:      %s\n", result.Duration.Round(time.Millisecond))
	fmt.Println()
}

// ParallelWriteTask represents a CSV write task
type ParallelWriteTask struct {
	Name string
	Fn   func() error
}

// RunParallelWrites executes multiple CSV write tasks in parallel
func RunParallelWrites(tasks []ParallelWriteTask, verbose bool) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(tasks))

	for _, task := range tasks {
		wg.Add(1)
		go func(t ParallelWriteTask) {
			defer wg.Done()
			if err := t.Fn(); err != nil {
				errChan <- fmt.Errorf("%s: %w", t.Name, err)
			}
		}(task)
	}

	wg.Wait()
	close(errChan)

	// Collect any errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return errs[0] // Return first error
	}

	return nil
}
