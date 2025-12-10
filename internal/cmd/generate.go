package cmd

import (
	"fmt"
	"os"

	"github.com/willfong/load-generator/internal/config"
	"github.com/willfong/load-generator/internal/generator"
	"github.com/willfong/load-generator/internal/ui"

	"github.com/spf13/cobra"
)

var (
	// Generation parameters (frequently changed)
	numCustomers int
	numYears     int
	outputDir    string
	seed         int64
	entitiesOnly bool
	compress     bool
	workers      int
)

// generateCmd represents the generate command
var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate bulk historical banking data",
	Long: `Generate realistic historical banking data for database seeding.

This command creates CSV files containing:
- Customers with realistic PII and geographic distribution
- Bank accounts of various types
- Historical transactions with realistic patterns
- Beneficiaries (external payees)
- Branches and ATMs
- Audit logs

Entity counts are derived from customer count using ratios in config/defaults.go.
Transaction patterns and error rates are also configured there.

Example:
  loadgen generate --customers 100000 --years 5
  loadgen generate --customers 10000 --entities   # Static data only
  loadgen generate --seed 42                      # Reproducible`,
	Run: runGenerate,
}

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().IntVar(&numCustomers, "customers", 10000, "number of customers to generate")
	generateCmd.Flags().IntVar(&numYears, "years", 3, "years of historical data to generate")
	generateCmd.Flags().StringVar(&outputDir, "output", "./output", "output directory for CSV files")
	generateCmd.Flags().Int64Var(&seed, "seed", 0, "random seed for reproducibility (0 = random)")
	generateCmd.Flags().BoolVar(&entitiesOnly, "entities", false, "generate only static entities (no transactions)")
	generateCmd.Flags().BoolVar(&compress, "compress", false, "compress output with xz (creates .csv.xz files)")
	generateCmd.Flags().IntVar(&workers, "workers", 0, "number of parallel workers (0 = auto-detect CPUs)")
}

func runGenerate(cmd *cobra.Command, args []string) {
	// Initialize UI
	u := ui.New()
	if noColor {
		u.SetNoColor(true)
	}

	// Check xz availability if compression is requested
	if compress {
		if err := generator.CheckXZAvailable(); err != nil {
			fmt.Fprintln(os.Stderr, u.Error("xz compression requested but xz is not available"))
			fmt.Fprintln(os.Stderr, "Install with: apt install xz-utils (Linux) or brew install xz (macOS)")
			os.Exit(1)
		}
	}

	// Calculate derived counts from customer count
	numBusinesses := int(float64(numCustomers) * config.BusinessRatio)
	numBranches := int(float64(numCustomers) * config.BranchRatio)
	numATMs := int(float64(numCustomers) * config.ATMRatio)

	// Ensure minimums
	if numBusinesses < 10 {
		numBusinesses = 10
	}
	if numBranches < 5 {
		numBranches = 5
	}
	if numATMs < 10 {
		numATMs = 10
	}

	fmt.Println(u.Header("Bank-in-a-Box Data Generator"))
	fmt.Println()
	fmt.Println(u.KeyValue("Customers", fmt.Sprintf("%d", numCustomers)))
	fmt.Println(u.KeyValue("Businesses", fmt.Sprintf("%d (%.0f%% of customers)", numBusinesses, config.BusinessRatio*100)))
	fmt.Println(u.KeyValue("Branches", fmt.Sprintf("%d", numBranches)))
	fmt.Println(u.KeyValue("ATMs", fmt.Sprintf("%d", numATMs)))
	fmt.Println(u.KeyValue("Years", fmt.Sprintf("%d", numYears)))
	fmt.Println(u.KeyValue("Output", outputDir))
	if seed != 0 {
		fmt.Println(u.KeyValue("Seed", fmt.Sprintf("%d", seed)))
	}
	if compress {
		fmt.Println(u.KeyValue("Compression", "xz (.csv.xz)"))
	}
	workerCount := generator.GetWorkerCount(workers)
	fmt.Println(u.KeyValue("Workers", fmt.Sprintf("%d", workerCount)))
	if entitiesOnly {
		fmt.Println(u.KeyValue("Mode", "entities only (no transactions)"))
	}
	fmt.Println()

	// Create orchestrator with defaults from config
	orchestrator, err := generator.NewOrchestrator(generator.OrchestratorConfig{
		NumCustomers:                    numCustomers,
		NumBusinesses:                   numBusinesses,
		NumBranches:                     numBranches,
		NumATMs:                         numATMs,
		YearsOfHistory:                  numYears,
		OutputDir:                       outputDir,
		Seed:                            seed,
		TransactionsPerCustomerPerMonth: config.TransactionsPerCustomerPerMonth,
		PayrollDay:                      config.PayrollDay,
		ParetoRatio:                     config.ParetoRatio,
		DeclinedTransactionRate:         config.DeclinedTransactionRate,
		InsufficientFundsRate:           config.InsufficientFundsRate,
		FailedLoginRate:                 config.FailedLoginRate,
		Compress:                        compress,
		Workers:                         workers,
	}, generator.OrchestratorOptions{
		Verbose:      verbose,
		ShowProgress: true,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, u.Error(err.Error()))
		os.Exit(1)
	}

	var result *generator.GenerationResult

	if entitiesOnly {
		spin := u.NewSpinner("Generating entities")
		spin.Start()
		result, err = orchestrator.GenerateEntities()
		if err != nil {
			spin.Error(err.Error())
			os.Exit(1)
		}
		spin.Success("complete")
	} else {
		spin := u.NewSpinner("Generating all data (entities + transactions)")
		spin.Start()
		result, err = orchestrator.GenerateAll()
		if err != nil {
			spin.Error(err.Error())
			os.Exit(1)
		}
		spin.Success("complete")
	}

	printGenerateSummary(u, result)
	fmt.Println()
	fmt.Println(u.Success("Output files written to: " + outputDir))
}

// printGenerateSummary prints a styled generation summary
func printGenerateSummary(u *ui.UI, result *generator.GenerationResult) {
	items := []ui.KV{
		{Key: "Branches", Value: fmt.Sprintf("%d", result.BranchCount)},
		{Key: "ATMs", Value: fmt.Sprintf("%d", result.ATMCount)},
		{Key: "Customers", Value: fmt.Sprintf("%d", result.CustomerCount)},
		{Key: "Businesses", Value: fmt.Sprintf("%d", result.BusinessCount)},
		{Key: "Accounts", Value: fmt.Sprintf("%d", result.AccountCount)},
		{Key: "Beneficiaries", Value: fmt.Sprintf("%d", result.BeneficiaryCount)},
		{Key: "Transactions", Value: fmt.Sprintf("%d", result.TransactionCount)},
		{Key: "Audit Logs", Value: fmt.Sprintf("%d", result.AuditLogCount)},
		{Key: "Duration", Value: result.Duration.Round(1 * 1e6).String()},
		{Key: "Status", Value: "Success"},
	}

	fmt.Println(u.SummaryBox("Generation Complete", items))
}
