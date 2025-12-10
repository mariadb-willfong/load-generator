// Package userstory provides user story validation tests for the simulator.
//
// FILE: helpers_test.go
// PURPOSE: Shared test helper types and configuration functions used across all user story tests.
//
// KEY TYPES/FUNCTIONS:
// - TransferResult: Mock struct for testing transfer operations
// - SweepResult: Mock struct for testing sweep operations
// - BatchPayrollResult: Mock struct for testing batch payroll operations
// - testConfig(): Creates a standard SimulateConfig for tests
//
// RELATED FILES:
// - retail_test.go: Retail customer user story tests
// - business_test.go: Business customer user story tests
// - staff_test.go: Bank staff user story tests
// - timezone_test.go: Timezone awareness tests
package userstory

import (
	"time"

	"github.com/willfong/load-generator/internal/config"
)

// TransferResult matches database.TransferResult for testing
type TransferResult struct {
	SourceTransactionID int64
	DestTransactionID   int64
	NewSourceBalance    int64
	NewDestBalance      int64
}

// SweepResult matches database.SweepResult for testing
type SweepResult struct {
	NewSourceBalance int64
	NewDestBalance   int64
}

// BatchPayrollResult matches database.BatchPayrollResult for testing
type BatchPayrollResult struct {
	SourceTransactionID int64
	TotalAmount         int64
	SuccessCount        int
	FailureCount        int
}

// testConfig creates a standard SimulateConfig for tests
func testConfig() config.SimulateConfig {
	return config.SimulateConfig{
		NumSessions:           10,
		MinThinkTime:          10 * time.Millisecond,
		MaxThinkTime:          50 * time.Millisecond,
		MetricsInterval:       1 * time.Second,
		ReadWriteRatio:        5.0,
		FailedLoginRate:       0.01,
		InsufficientFundsRate: 0.005,
		TimeoutRate:           0.001,
	}
}
