package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/willfong/load-generator/internal/config"
	"github.com/willfong/load-generator/internal/database"
	"github.com/willfong/load-generator/internal/simulator"
	"github.com/willfong/load-generator/internal/ui"
)

var (
	// Simulation parameters (frequently changed)
	concurrency  int
	simSeed      int64
	dbConnection string
	duration     string

	// Database pool settings
	dbMaxOpenConns int
	dbMaxIdleConns int
)

// simulateCmd represents the simulate command
var simulateCmd = &cobra.Command{
	Use:   "simulate",
	Short: "Run live customer interaction simulation",
	Long: `Simulate concurrent banking customers interacting with the database.

This command runs a continuous simulation of:
- ATM sessions (balance inquiries, withdrawals)
- Online banking sessions (logins, transfers, history views)
- Business account activity (payroll, merchant transactions)

Session ratios, burst settings, and error rates are in config/defaults.go.

The simulation runs indefinitely until interrupted (Ctrl+C).

Example:
  loadgen simulate --db "user:pass@tcp(localhost:3306)/bank"
  loadgen simulate --concurrency 1000 --db "..."
  loadgen simulate --duration 1h --db "..."
  loadgen simulate --seed 42 --db "..."`,
	Run: runSimulate,
}

func init() {
	rootCmd.AddCommand(simulateCmd)

	simulateCmd.Flags().IntVar(&concurrency, "concurrency", 100, "number of concurrent customer sessions")
	simulateCmd.Flags().Int64Var(&simSeed, "seed", 0, "random seed for reproducibility (0 = random)")
	simulateCmd.Flags().StringVar(&dbConnection, "db", "", "database connection string (required)")
	simulateCmd.Flags().StringVar(&duration, "duration", "", "simulation duration (e.g., 1h, 30m). Empty = run until killed")
	simulateCmd.Flags().IntVar(&dbMaxOpenConns, "db-max-open", config.DBMaxOpenConns, "max open database connections")
	simulateCmd.Flags().IntVar(&dbMaxIdleConns, "db-max-idle", config.DBMaxIdleConns, "max idle database connections")

	simulateCmd.MarkFlagRequired("db")
}

func runSimulate(cmd *cobra.Command, args []string) {
	// Initialize UI
	u := ui.New()
	if noColor {
		u.SetNoColor(true)
	}

	fmt.Println(u.Header("Bank-in-a-Box Load Simulator"))
	fmt.Println()
	fmt.Println(u.KeyValue("Concurrency", fmt.Sprintf("%d sessions", concurrency)))
	fmt.Println(u.KeyValue("R/W Ratio", fmt.Sprintf("%.0f:1", config.ReadWriteRatio)))
	fmt.Println(u.KeyValue("Session Mix", fmt.Sprintf("ATM %.0f%% / Online %.0f%% / Business %.0f%%",
		config.ATMSessionRatio*100,
		config.OnlineSessionRatio*100,
		config.BusinessSessionRatio*100)))
	fmt.Println(u.KeyValue("DB Pool", fmt.Sprintf("%d open / %d idle", dbMaxOpenConns, dbMaxIdleConns)))
	if simSeed != 0 {
		fmt.Println(u.KeyValue("Seed", fmt.Sprintf("%d", simSeed)))
	}
	if duration != "" {
		fmt.Println(u.KeyValue("Duration", duration))
	} else {
		fmt.Println(u.KeyValue("Duration", "until stopped (Ctrl+C)"))
	}
	fmt.Println()

	// Build simulation config from defaults
	simConfig := buildSimulateConfig()

	// Override with CLI values
	simConfig.NumSessions = concurrency
	simConfig.Seed = simSeed

	if duration != "" {
		d, err := time.ParseDuration(duration)
		if err != nil {
			fmt.Fprintln(os.Stderr, u.Error(fmt.Sprintf("Error parsing duration: %v", err)))
			return
		}
		simConfig.Duration = d
	}

	// Build database config from CLI flags and defaults
	dbConfig := config.DatabaseConfig{
		DSN:             dbConnection,
		Driver:          config.DBDriver,
		MaxOpenConns:    dbMaxOpenConns,
		MaxIdleConns:    dbMaxIdleConns,
		ConnMaxLifetime: config.DBConnMaxLifetime,
		ConnMaxIdleTime: config.DBConnMaxIdleTime,
	}

	// Create database connection pool
	pool, err := database.NewPool(dbConfig)
	if err != nil {
		fmt.Fprintln(os.Stderr, u.Error(fmt.Sprintf("Error creating database pool: %v", err)))
		return
	}
	defer pool.Close()

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	spin := u.NewSpinner("Connecting to database")
	spin.Start()
	if err := pool.Connect(ctx); err != nil {
		spin.Error("connection failed: " + err.Error())
		return
	}
	spin.Success("connected!")
	fmt.Println()

	// Create and start session manager
	manager := simulator.NewSessionManager(pool, simConfig, simSeed)

	spinStart := u.NewSpinner("Starting simulation")
	spinStart.Start()
	if err := manager.Start(); err != nil {
		spinStart.Error("failed: " + err.Error())
		return
	}
	spinStart.Success(fmt.Sprintf("running with %d sessions", concurrency))

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Duration timer if set
	var durationCh <-chan time.Time
	if simConfig.Duration > 0 {
		timer := time.NewTimer(simConfig.Duration)
		defer timer.Stop()
		durationCh = timer.C
	}

	// Wait for shutdown signal or duration
	select {
	case <-sigCh:
		fmt.Println()
		fmt.Println(u.Warning("Received shutdown signal"))
	case <-durationCh:
		fmt.Println()
		fmt.Println(u.Success(fmt.Sprintf("Duration (%s) reached", duration)))
	}

	// Graceful shutdown
	spinStop := u.NewSpinner("Stopping simulation")
	spinStop.Start()
	manager.Stop()
	spinStop.Success("stopped")
}

// buildSimulateConfig creates a SimulateConfig from compile-time defaults
func buildSimulateConfig() config.SimulateConfig {
	return config.SimulateConfig{
		NumSessions:           concurrency,
		Seed:                  simSeed,
		ReadWriteRatio:        config.ReadWriteRatio,
		ATMSessionRatio:       config.ATMSessionRatio,
		OnlineSessionRatio:    config.OnlineSessionRatio,
		BusinessSessionRatio:  config.BusinessSessionRatio,
		ActiveHourStart:       config.ActiveHourStart,
		ActiveHourEnd:         config.ActiveHourEnd,
		MinThinkTime:          config.MinThinkTime,
		MaxThinkTime:          config.MaxThinkTime,
		EnablePayrollBurst:    config.EnablePayrollBurst,
		EnableLunchBurst:      config.EnableLunchBurst,
		EnableRandomBurst:     config.EnableRandomBurst,
		LunchBurstMultiplier:  config.LunchBurstMultiplier,
		PayrollBurstMultiplier: config.PayrollBurstMultiplier,
		FailedLoginRate:        config.SimFailedLoginRate,
		InsufficientFundsRate:  config.SimInsufficientFundsRate,
		TimeoutRate:            config.SimTimeoutRate,
		MetricsInterval:        config.MetricsInterval,
		EnableRamp:             config.EnableRamp,
		RampUpDuration:         config.RampUpDuration,
		RampDownDuration:       config.RampDownDuration,
		RampSteps:              config.RampSteps,
	}
}
