// Package config contains compile-time defaults for the load generator.
// Edit these values and recompile to tune behavior.
package config

import "time"

// =============================================================================
// PHASE 1: DATA GENERATION DEFAULTS
// =============================================================================

// Entity ratios relative to customer count
const (
	// BusinessRatio is businesses per customer (0.05 = 1 business per 20 customers)
	BusinessRatio = 0.05

	// BranchRatio is branches per customer (0.01 = 1 branch per 100 customers)
	BranchRatio = 0.01

	// ATMRatio is ATMs per customer (0.05 = 1 ATM per 20 customers)
	ATMRatio = 0.05
)

// Transaction generation
const (
	// TransactionsPerCustomerPerMonth is average monthly transaction count
	TransactionsPerCustomerPerMonth = 15

	// PayrollDay is the day of month for salary deposits (1-31)
	PayrollDay = 25

	// ParetoRatio controls activity distribution (0.2 = top 20% generate 80% volume)
	ParetoRatio = 0.2
)

// Error simulation rates for generated data
const (
	// DeclinedTransactionRate is the fraction of transactions marked as declined
	DeclinedTransactionRate = 0.01

	// InsufficientFundsRate is the fraction with insufficient funds errors
	InsufficientFundsRate = 0.02

	// FailedLoginRate is the fraction of login attempts that fail
	FailedLoginRate = 0.02
)

// =============================================================================
// PHASE 2: SIMULATION DEFAULTS
// =============================================================================

// Session distribution
const (
	// ATMSessionRatio is the fraction of ATM sessions (0.3 = 30%)
	ATMSessionRatio = 0.3

	// OnlineSessionRatio is the fraction of online banking sessions (0.5 = 50%)
	OnlineSessionRatio = 0.5

	// BusinessSessionRatio is the fraction of business sessions (0.2 = 20%)
	BusinessSessionRatio = 0.2
)

// Workload mix
const (
	// ReadWriteRatio is reads per write operation (5.0 = 5 reads per 1 write)
	ReadWriteRatio = 5.0
)

// Active hours (local time for each customer's timezone)
const (
	// ActiveHourStart is when customers become active (24-hour format)
	ActiveHourStart = 8

	// ActiveHourEnd is when customers become inactive (24-hour format)
	ActiveHourEnd = 16
)

// Think time between operations
const (
	// MinThinkTime is minimum delay between session actions
	MinThinkTime = 500 * time.Millisecond

	// MaxThinkTime is maximum delay between session actions
	MaxThinkTime = 5 * time.Second
)

// Burst simulation
const (
	// EnablePayrollBurst enables end-of-month payroll surge
	EnablePayrollBurst = true

	// EnableLunchBurst enables lunch-time ATM activity spike
	EnableLunchBurst = true

	// EnableRandomBurst enables random load spikes
	EnableRandomBurst = false

	// LunchBurstMultiplier is the activity multiplier during lunch (1.5 = 50% increase)
	LunchBurstMultiplier = 1.5

	// PayrollBurstMultiplier is the activity multiplier on payroll days
	PayrollBurstMultiplier = 3.0

	// RandomBurstProbability is chance per check interval to trigger random burst
	RandomBurstProbability = 0.01

	// RandomBurstCooldown is minimum time between random bursts
	RandomBurstCooldown = 15 * time.Minute
)

// Ramp-up/down for gradual load changes
const (
	// EnableRamp enables gradual load ramp-up and ramp-down
	EnableRamp = false

	// RampUpDuration is how long to ramp up to full load
	RampUpDuration = 5 * time.Minute

	// RampDownDuration is how long to ramp down during shutdown
	RampDownDuration = 2 * time.Minute

	// RampSteps is how many steps to divide the ramp into
	RampSteps = 10
)

// Error simulation rates during simulation
const (
	// SimFailedLoginRate is the fraction of login attempts that fail
	SimFailedLoginRate = 0.02

	// SimInsufficientFundsRate is the fraction of transactions declined
	SimInsufficientFundsRate = 0.01

	// SimTimeoutRate is the fraction of operations that timeout
	SimTimeoutRate = 0.001
)

// =============================================================================
// DATABASE DEFAULTS
// =============================================================================

const (
	// DBDriver is the database driver to use
	DBDriver = "mysql"

	// DBMaxOpenConns is maximum open connections in the pool
	DBMaxOpenConns = 100

	// DBMaxIdleConns is maximum idle connections in the pool
	DBMaxIdleConns = 10

	// DBConnMaxLifetime is how long a connection can be reused
	DBConnMaxLifetime = 5 * time.Minute

	// DBConnMaxIdleTime is how long an idle connection is kept
	DBConnMaxIdleTime = 1 * time.Minute
)

// =============================================================================
// METRICS AND MONITORING
// =============================================================================

const (
	// MetricsInterval is how often to report real-time metrics
	MetricsInterval = 5 * time.Second

	// GracefulShutdownTimeout is max wait time for graceful shutdown
	GracefulShutdownTimeout = 30 * time.Second
)
