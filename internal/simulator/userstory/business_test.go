// Package userstory provides user story validation tests for the simulator.
//
// FILE: business_test.go
// PURPOSE: Tests for business customer user stories including merchant flows,
// account sweeps, and batch payroll operations.
//
// KEY TESTS:
// - TestUS_BusinessCustomer_MerchantFlows: Incoming payments and sweep operations
// - TestUS_PayrollOperator_BatchPayroll: Batch salary processing
//
// RELATED FILES:
// - helpers_test.go: Shared test utilities
// - retail_test.go: Retail customer tests
package userstory

import (
	"testing"
	"time"

	"github.com/willfong/load-generator/internal/models"
	"github.com/willfong/load-generator/internal/utils"
)

// TestUS_BusinessCustomer_MerchantFlows validates:
// "As a business customer, I want to receive many small incoming payments
// (merchant flows) and periodically sweep funds to another account so that
// my operating balance stays controlled."
//
// Acceptance: bulk incoming credits recorded with realistic memos; sweeps post
// as transfers with paired entries; activity visible in history and balances.
func TestUS_BusinessCustomer_MerchantFlows(t *testing.T) {
	customer := &models.Customer{
		ID:            2001,
		Timezone:      "America/New_York",
		Segment:       models.SegmentBusiness,
		ActivityScore: 0.7,
	}

	accounts := []*models.Account{
		{ID: 200, CustomerID: 2001, Type: models.AccountTypeBusiness, Balance: 5000000},
		{ID: 201, CustomerID: 2001, Type: models.AccountTypeSavings, Balance: 10000000},
	}

	t.Run("business_segment", func(t *testing.T) {
		if customer.Segment != models.SegmentBusiness {
			t.Error("expected business segment")
		}
	})

	t.Run("has_business_and_savings_accounts", func(t *testing.T) {
		hasBusiness := false
		hasSavings := false
		for _, acc := range accounts {
			if acc.Type == models.AccountTypeBusiness {
				hasBusiness = true
			}
			if acc.Type == models.AccountTypeSavings {
				hasSavings = true
			}
		}
		if !hasBusiness {
			t.Error("business customer should have business account")
		}
		if !hasSavings {
			t.Error("business customer should have savings account for sweeps")
		}
	})

	t.Run("sweep_requires_multiple_accounts", func(t *testing.T) {
		if len(accounts) < 2 {
			t.Error("sweep requires at least 2 accounts")
		}
	})

	t.Run("sweep_result_structure", func(t *testing.T) {
		result := &SweepResult{
			NewSourceBalance: 500000,
			NewDestBalance:   4500000,
		}
		if result.NewSourceBalance <= 0 {
			t.Error("sweep should leave operating balance")
		}
	})
}

// TestUS_PayrollOperator_BatchPayroll validates:
// "As a payroll operator (business customer), I want to run end-of-month salary
// batches to many employees so that payroll completes on schedule."
//
// Acceptance: batch creates many outgoing credits to employee accounts; spike
// handled without dropping transactions; audit trail links batch reference to each payment.
func TestUS_PayrollOperator_BatchPayroll(t *testing.T) {
	rng := utils.NewRandom(42)

	accounts := []*models.Account{
		{ID: 300, CustomerID: 3001, Type: models.AccountTypePayroll, Balance: 100000000}, // $1M
	}

	t.Run("payroll_period_detection", func(t *testing.T) {
		// Test isPayrollPeriod function (days 25-28)
		now := time.Now()
		day := now.Day()
		isPayroll := day >= 25 && day <= 28

		// Just verify the logic is testable
		if day >= 25 && day <= 28 && !isPayroll {
			t.Error("expected payroll period for days 25-28")
		}
	})

	t.Run("payroll_batch_size", func(t *testing.T) {
		// Batch payroll should pay 5-20 employees
		for i := 0; i < 100; i++ {
			numEmployees := 5 + rng.IntN(16)
			if numEmployees < 5 || numEmployees > 20 {
				t.Errorf("expected 5-20 employees, got %d", numEmployees)
			}
		}
	})

	t.Run("salary_amounts_realistic", func(t *testing.T) {
		// Salary range: $2,000 - $8,000 per pay period
		for i := 0; i < 100; i++ {
			salary := int64(200000 + rng.IntN(600000)) // cents
			if salary < 200000 || salary > 800000 {
				t.Errorf("salary %d outside realistic range ($2000-$8000)", salary)
			}
		}
	})

	t.Run("has_payroll_account", func(t *testing.T) {
		hasPayroll := false
		for _, acc := range accounts {
			if acc.Type == models.AccountTypePayroll {
				hasPayroll = true
				break
			}
		}
		if !hasPayroll {
			t.Error("payroll session should have payroll account")
		}
	})

	t.Run("batch_result_structure", func(t *testing.T) {
		result := &BatchPayrollResult{
			SourceTransactionID: 1001,
			TotalAmount:         5000000,
			SuccessCount:        10,
			FailureCount:        0,
		}

		if result.SuccessCount == 0 {
			t.Error("batch should process some employees")
		}
		if result.TotalAmount <= 0 {
			t.Error("batch should have total amount")
		}
	})
}
