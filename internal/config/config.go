package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config holds all configuration for the load generator
type Config struct {
	// Database configuration
	Database DatabaseConfig `mapstructure:"database"`

	// Data generation configuration (Phase 1)
	Generate GenerateConfig `mapstructure:"generate"`

	// Live simulation configuration (Phase 2)
	Simulate SimulateConfig `mapstructure:"simulate"`

	// Data files location
	DataDir string `mapstructure:"data_dir"`

	// Logging
	Verbose bool `mapstructure:"verbose"`
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	// Connection string (DSN)
	// Format: user:password@tcp(host:port)/database
	DSN string `mapstructure:"dsn"`

	// Driver (mysql, postgres, etc.)
	Driver string `mapstructure:"driver"`

	// Connection pool settings
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

// GenerateConfig holds data generation settings
type GenerateConfig struct {
	// Random seed for reproducibility (0 = random)
	Seed int64 `mapstructure:"seed"`

	// Output directory for generated files
	OutputDir string `mapstructure:"output_dir"`

	// Volume settings
	NumCustomers   int `mapstructure:"num_customers"`
	NumBusinesses  int `mapstructure:"num_businesses"`  // Business/merchant accounts
	NumBranches    int `mapstructure:"num_branches"`
	NumATMs        int `mapstructure:"num_atms"`
	YearsOfHistory int `mapstructure:"years_of_history"`

	// Transaction patterns
	TransactionsPerCustomerPerMonth int     `mapstructure:"transactions_per_customer_per_month"`
	PayrollDay                       int     `mapstructure:"payroll_day"` // Day of month (1-31)
	ParetoRatio                      float64 `mapstructure:"pareto_ratio"` // Top X% accounts generate Y% transactions

	// Error simulation rates (0.0-1.0)
	FailedLoginRate        float64 `mapstructure:"failed_login_rate"`
	InsufficientFundsRate  float64 `mapstructure:"insufficient_funds_rate"`

	// Parallelism for generation
	NumWorkers int `mapstructure:"num_workers"`
}

// SimulateConfig holds live simulation settings
type SimulateConfig struct {
	// Random seed for reproducibility (0 = random)
	Seed int64 `mapstructure:"seed"`

	// Concurrency
	NumSessions int `mapstructure:"num_sessions"` // Concurrent customer sessions

	// Workload mix
	ReadWriteRatio float64 `mapstructure:"read_write_ratio"` // Reads per write

	// Session type distribution (should sum to 1.0)
	ATMSessionRatio     float64 `mapstructure:"atm_session_ratio"`
	OnlineSessionRatio  float64 `mapstructure:"online_session_ratio"`
	BusinessSessionRatio float64 `mapstructure:"business_session_ratio"`

	// Timing
	Duration         time.Duration `mapstructure:"duration"` // 0 = run until killed
	MinThinkTime     time.Duration `mapstructure:"min_think_time"`
	MaxThinkTime     time.Duration `mapstructure:"max_think_time"`

	// Active hours (local time for each customer's timezone)
	ActiveHourStart int `mapstructure:"active_hour_start"` // 0-23
	ActiveHourEnd   int `mapstructure:"active_hour_end"`   // 0-23

	// Burst settings
	EnablePayrollBurst bool    `mapstructure:"enable_payroll_burst"`
	EnableLunchBurst   bool    `mapstructure:"enable_lunch_burst"`
	EnableRandomBurst  bool    `mapstructure:"enable_random_burst"`
	BurstMultiplier    float64 `mapstructure:"burst_multiplier"` // Default multiplier during bursts

	// Detailed burst configuration
	LunchBurstMultiplier   float64       `mapstructure:"lunch_burst_multiplier"`
	LunchBurstDuration     time.Duration `mapstructure:"lunch_burst_duration"`
	PayrollBurstMultiplier float64       `mapstructure:"payroll_burst_multiplier"`
	PayrollBurstDuration   time.Duration `mapstructure:"payroll_burst_duration"`
	RandomBurstProbability float64       `mapstructure:"random_burst_probability"`
	RandomBurstMinDuration time.Duration `mapstructure:"random_burst_min_duration"`
	RandomBurstMaxDuration time.Duration `mapstructure:"random_burst_max_duration"`
	RandomBurstMinMultiplier float64     `mapstructure:"random_burst_min_multiplier"`
	RandomBurstMaxMultiplier float64     `mapstructure:"random_burst_max_multiplier"`
	RandomBurstCooldown    time.Duration `mapstructure:"random_burst_cooldown"`

	// Load ramp-up/ramp-down
	EnableRamp       bool          `mapstructure:"enable_ramp"`
	RampUpDuration   time.Duration `mapstructure:"ramp_up_duration"`
	RampDownDuration time.Duration `mapstructure:"ramp_down_duration"`
	RampSteps        int           `mapstructure:"ramp_steps"`

	// Error simulation
	FailedLoginRate       float64 `mapstructure:"failed_login_rate"`
	InsufficientFundsRate float64 `mapstructure:"insufficient_funds_rate"`
	TimeoutRate           float64 `mapstructure:"timeout_rate"`

	// Metrics
	MetricsInterval time.Duration `mapstructure:"metrics_interval"`
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	return &Config{
		Database: DatabaseConfig{
			Driver:          "mysql",
			MaxOpenConns:    100,
			MaxIdleConns:    10,
			ConnMaxLifetime: 5 * time.Minute,
			ConnMaxIdleTime: 1 * time.Minute,
		},
		Generate: GenerateConfig{
			Seed:                             0,
			OutputDir:                        "./output",
			NumCustomers:                     10000,
			NumBusinesses:                    500,
			NumBranches:                      100,
			NumATMs:                          500,
			YearsOfHistory:                   3,
			TransactionsPerCustomerPerMonth:  15,
			PayrollDay:                       25,
			ParetoRatio:                      0.2, // Top 20% generate 80% of activity
			FailedLoginRate:                  0.02,
			InsufficientFundsRate:            0.01,
			NumWorkers:                       4,
		},
		Simulate: SimulateConfig{
			Seed:                  0,
			NumSessions:          100,
			ReadWriteRatio:       5.0,
			ATMSessionRatio:      0.3,
			OnlineSessionRatio:   0.5,
			BusinessSessionRatio: 0.2,
			Duration:             0, // Run until killed
			MinThinkTime:         500 * time.Millisecond,
			MaxThinkTime:         5 * time.Second,
			ActiveHourStart:      8,
			ActiveHourEnd:        16,
			// Burst settings
			EnablePayrollBurst:       true,
			EnableLunchBurst:         true,
			EnableRandomBurst:        false,
			BurstMultiplier:          2.0,
			LunchBurstMultiplier:     1.5,
			LunchBurstDuration:       2 * time.Hour,
			PayrollBurstMultiplier:   3.0,
			PayrollBurstDuration:     8 * time.Hour,
			RandomBurstProbability:   0.01,
			RandomBurstMinDuration:   5 * time.Minute,
			RandomBurstMaxDuration:   30 * time.Minute,
			RandomBurstMinMultiplier: 1.5,
			RandomBurstMaxMultiplier: 4.0,
			RandomBurstCooldown:      15 * time.Minute,
			// Ramp settings
			EnableRamp:           false,
			RampUpDuration:       5 * time.Minute,
			RampDownDuration:     2 * time.Minute,
			RampSteps:            10,
			// Error rates
			FailedLoginRate:      0.02,
			InsufficientFundsRate: 0.01,
			TimeoutRate:          0.001,
			MetricsInterval:      5 * time.Second,
		},
		DataDir: "./data",
		Verbose: false,
	}
}

// Load reads configuration from viper into a Config struct
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Unmarshal viper config into struct
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	var errs []string

	// Validate generation config
	if c.Generate.NumCustomers <= 0 {
		errs = append(errs, "generate.num_customers must be positive")
	}
	if c.Generate.NumBusinesses < 0 {
		errs = append(errs, "generate.num_businesses must be non-negative")
	}
	if c.Generate.NumBranches <= 0 {
		errs = append(errs, "generate.num_branches must be positive")
	}
	if c.Generate.NumATMs < 0 {
		errs = append(errs, "generate.num_atms must be non-negative")
	}
	if c.Generate.YearsOfHistory <= 0 {
		errs = append(errs, "generate.years_of_history must be positive")
	}
	if c.Generate.PayrollDay < 1 || c.Generate.PayrollDay > 31 {
		errs = append(errs, "generate.payroll_day must be between 1 and 31")
	}
	if c.Generate.ParetoRatio <= 0 || c.Generate.ParetoRatio >= 1 {
		errs = append(errs, "generate.pareto_ratio must be between 0 and 1 (exclusive)")
	}
	if c.Generate.FailedLoginRate < 0 || c.Generate.FailedLoginRate > 1 {
		errs = append(errs, "generate.failed_login_rate must be between 0.0 and 1.0")
	}
	if c.Generate.InsufficientFundsRate < 0 || c.Generate.InsufficientFundsRate > 1 {
		errs = append(errs, "generate.insufficient_funds_rate must be between 0.0 and 1.0")
	}

	// Validate simulation config
	if c.Simulate.NumSessions <= 0 {
		errs = append(errs, "simulate.num_sessions must be positive")
	}
	if c.Simulate.ReadWriteRatio < 0 {
		errs = append(errs, "simulate.read_write_ratio must be non-negative")
	}

	// Validate session ratios sum to approximately 1.0
	ratioSum := c.Simulate.ATMSessionRatio + c.Simulate.OnlineSessionRatio + c.Simulate.BusinessSessionRatio
	if ratioSum < 0.99 || ratioSum > 1.01 {
		errs = append(errs, fmt.Sprintf("session ratios must sum to 1.0 (got %.2f)", ratioSum))
	}

	// Validate active hours
	if c.Simulate.ActiveHourStart < 0 || c.Simulate.ActiveHourStart > 23 {
		errs = append(errs, "simulate.active_hour_start must be 0-23")
	}
	if c.Simulate.ActiveHourEnd < 0 || c.Simulate.ActiveHourEnd > 23 {
		errs = append(errs, "simulate.active_hour_end must be 0-23")
	}

	// Validate error rates
	if c.Simulate.FailedLoginRate < 0 || c.Simulate.FailedLoginRate > 1 {
		errs = append(errs, "simulate.failed_login_rate must be between 0.0 and 1.0")
	}
	if c.Simulate.InsufficientFundsRate < 0 || c.Simulate.InsufficientFundsRate > 1 {
		errs = append(errs, "simulate.insufficient_funds_rate must be between 0.0 and 1.0")
	}
	if c.Simulate.TimeoutRate < 0 || c.Simulate.TimeoutRate > 1 {
		errs = append(errs, "simulate.timeout_rate must be between 0.0 and 1.0")
	}

	// Validate burst settings
	if c.Simulate.BurstMultiplier < 1 {
		errs = append(errs, "simulate.burst_multiplier must be >= 1.0")
	}
	if c.Simulate.LunchBurstMultiplier < 1 {
		errs = append(errs, "simulate.lunch_burst_multiplier must be >= 1.0")
	}
	if c.Simulate.PayrollBurstMultiplier < 1 {
		errs = append(errs, "simulate.payroll_burst_multiplier must be >= 1.0")
	}

	// Validate ramp settings
	if c.Simulate.RampSteps < 1 {
		errs = append(errs, "simulate.ramp_steps must be >= 1")
	}

	// Validate database pool settings
	if c.Database.MaxOpenConns < 1 {
		errs = append(errs, "database.max_open_conns must be >= 1")
	}
	if c.Database.MaxIdleConns < 0 {
		errs = append(errs, "database.max_idle_conns must be >= 0")
	}
	if c.Database.MaxIdleConns > c.Database.MaxOpenConns {
		errs = append(errs, "database.max_idle_conns should not exceed max_open_conns")
	}

	if len(errs) > 0 {
		return fmt.Errorf("validation errors:\n  - %s", joinErrors(errs))
	}

	return nil
}

// joinErrors joins error messages with newline and bullet points
func joinErrors(errs []string) string {
	result := errs[0]
	for i := 1; i < len(errs); i++ {
		result += "\n  - " + errs[i]
	}
	return result
}
