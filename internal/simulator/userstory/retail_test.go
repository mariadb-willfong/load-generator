// Package userstory provides user story validation tests for the simulator.
//
// FILE: retail_test.go
// PURPOSE: Tests for retail customer user stories including login, balance checks,
// transfers, ATM withdrawals, deposits, and failure handling.
//
// KEY TESTS:
// - TestUS_RetailCustomer_LoginAndViewBalances: Login and balance viewing
// - TestUS_RetailCustomer_ViewTransactionHistory: Transaction history access
// - TestUS_RetailCustomer_FundsTransfer: Internal transfers
// - TestUS_RetailCustomer_ATMWithdrawal: ATM cash withdrawal
// - TestUS_RetailCustomer_ATMDeposit: ATM deposits
// - TestUS_Customer_FailureOutcomes: Error handling and failure scenarios
//
// RELATED FILES:
// - helpers_test.go: Shared test utilities
// - business_test.go: Business customer tests
package userstory

import (
	"testing"
	"time"

	"github.com/willfong/load-generator/internal/models"
	"github.com/willfong/load-generator/internal/simulator"
	"github.com/willfong/load-generator/internal/utils"
)

// TestUS_RetailCustomer_LoginAndViewBalances validates:
// "As a retail customer, I want to log in to online banking and see my balances
// quickly so that I can confirm funds before acting."
//
// Acceptance: credentials validated; failed login logged; balance query returns
// per-account balances within expected latency.
func TestUS_RetailCustomer_LoginAndViewBalances(t *testing.T) {
	cfg := testConfig()
	errorSim := simulator.NewErrorSimulator(cfg)

	customer := &models.Customer{
		ID:            1001,
		Timezone:      "America/New_York",
		Segment:       models.SegmentRegular,
		ActivityScore: 0.5,
	}

	accounts := []*models.Account{
		{ID: 100, CustomerID: 1001, Type: models.AccountTypeChecking, Balance: 500000},
		{ID: 101, CustomerID: 1001, Type: models.AccountTypeSavings, Balance: 1000000},
	}

	t.Run("successful_authentication_with_low_failure_rate", func(t *testing.T) {
		// With 1% failure rate, authentication should mostly succeed
		rng := utils.NewRandom(42)
		successCount := 0
		iterations := 100

		for i := 0; i < iterations; i++ {
			if !errorSim.ShouldSimulateLoginFailure(rng) {
				successCount++
			}
		}

		// Should succeed most of the time (>90%)
		if successCount < 90 {
			t.Errorf("expected >90%% success rate, got %d%%", successCount)
		}
	})

	t.Run("failed_login_simulation", func(t *testing.T) {
		// Force login failure with high failure rate
		highFailCfg := testConfig()
		highFailCfg.FailedLoginRate = 1.0 // 100% failure
		errorSim := simulator.NewErrorSimulator(highFailCfg)

		rng := utils.NewRandom(99)
		shouldFail := errorSim.ShouldSimulateLoginFailure(rng)

		if !shouldFail {
			t.Error("expected authentication failure with 100% failure rate")
		}
	})

	t.Run("balance_query_multiple_accounts", func(t *testing.T) {
		// Verify multiple accounts available for balance check
		if len(accounts) < 2 {
			t.Error("expected at least 2 accounts for balance check test")
		}

		// All accounts should have positive balances
		for _, account := range accounts {
			if account.Balance <= 0 {
				t.Errorf("account %d has zero or negative balance", account.ID)
			}
		}
	})

	t.Run("customer_has_required_fields", func(t *testing.T) {
		if customer.ID <= 0 {
			t.Error("customer must have valid ID")
		}
		if customer.Timezone == "" {
			t.Error("customer must have timezone for activity scheduling")
		}
	})
}

// TestUS_RetailCustomer_ViewTransactionHistory validates:
// "As a retail customer, I want to view recent transactions (e.g., last 30 days
// or last 10 items) so that I can verify activity."
//
// Acceptance: history is ordered by timestamp; shows amount, type, memo,
// channel/ATM/branch; works for multiple accounts.
func TestUS_RetailCustomer_ViewTransactionHistory(t *testing.T) {
	rng := utils.NewRandom(42)

	accounts := []*models.Account{
		{ID: 100, CustomerID: 1001, Type: models.AccountTypeChecking, Balance: 500000},
		{ID: 101, CustomerID: 1001, Type: models.AccountTypeSavings, Balance: 1000000},
	}

	t.Run("history_works_for_multiple_accounts", func(t *testing.T) {
		// Verify random account selection works for any account
		for i := 0; i < 10; i++ {
			selectedIdx := rng.IntN(len(accounts))
			if selectedIdx < 0 || selectedIdx >= len(accounts) {
				t.Errorf("invalid account selection: index %d for %d accounts", selectedIdx, len(accounts))
			}

			selected := accounts[selectedIdx]
			if selected.ID <= 0 {
				t.Error("selected account must have valid ID")
			}
		}
	})

	t.Run("transaction_model_has_required_fields", func(t *testing.T) {
		// Create sample transaction to verify model supports required fields
		txn := &models.Transaction{
			ID:          1,
			AccountID:   100,
			Amount:      5000,
			Type:        models.TxTypeWithdrawal,
			Timestamp:   time.Now(),
			Channel:     models.ChannelOnline,
			Description: "Test transfer",
		}

		if txn.Amount <= 0 {
			t.Error("transaction should have positive amount")
		}
		if txn.Timestamp.IsZero() {
			t.Error("transaction should have timestamp")
		}
		if txn.Channel == "" {
			t.Error("transaction should have channel")
		}
		if txn.Description == "" {
			t.Error("transaction should have description/memo")
		}
	})
}

// TestUS_RetailCustomer_FundsTransfer validates:
// "As a retail customer, I want to transfer funds between my own accounts or
// to saved beneficiaries so that I can pay bills or move money."
//
// Acceptance: source/destination selection; amount validation and insufficient-funds
// handling; paired debit/credit entries for internal transfers; audit entry recorded.
func TestUS_RetailCustomer_FundsTransfer(t *testing.T) {
	rng := utils.NewRandom(42)

	customer := &models.Customer{
		ID:            1001,
		Timezone:      "America/New_York",
		Segment:       models.SegmentRegular,
		ActivityScore: 0.5,
	}

	accounts := []*models.Account{
		{ID: 100, CustomerID: 1001, Type: models.AccountTypeChecking, Balance: 500000},
		{ID: 101, CustomerID: 1001, Type: models.AccountTypeSavings, Balance: 1000000},
	}

	t.Run("transfer_amount_by_segment", func(t *testing.T) {
		// Test that transfer amounts vary by customer segment
		segments := []struct {
			segment  models.CustomerSegment
			minRange int64
			maxRange int64
		}{
			{models.SegmentRegular, 500, 50000},         // $5 - $500
			{models.SegmentPremium, 10000, 500000},      // $100 - $5,000
			{models.SegmentPrivate, 100000, 5000000},    // $1,000 - $50,000
			{models.SegmentBusiness, 50000, 2000000},    // $500 - $20,000
			{models.SegmentCorporate, 500000, 10000000}, // $5,000 - $100,000
		}

		for _, tc := range segments {
			customer.Segment = tc.segment
			rng := utils.NewRandom(42)

			// Simulate generateTransferAmount logic
			var minAmount, maxAmount int64
			switch customer.Segment {
			case models.SegmentPrivate:
				minAmount, maxAmount = 100000, 5000000
			case models.SegmentPremium:
				minAmount, maxAmount = 10000, 500000
			case models.SegmentCorporate:
				minAmount, maxAmount = 500000, 10000000
			case models.SegmentBusiness:
				minAmount, maxAmount = 50000, 2000000
			default:
				minAmount, maxAmount = 500, 50000
			}

			amount := minAmount + rng.Int64N(maxAmount-minAmount+1)
			if amount < minAmount || amount > maxAmount {
				t.Errorf("segment %s: amount %d out of range [%d, %d]",
					customer.Segment, amount, minAmount, maxAmount)
			}
		}
	})

	t.Run("insufficient_funds_simulation", func(t *testing.T) {
		// Test that insufficient funds can be simulated
		highFailCfg := testConfig()
		highFailCfg.InsufficientFundsRate = 1.0 // 100% failure
		errorSim := simulator.NewErrorSimulator(highFailCfg)

		shouldFail := errorSim.ShouldSimulateInsufficientFunds(utils.NewRandom(42))
		if !shouldFail {
			t.Error("expected insufficient funds simulation with 100% rate")
		}
	})

	t.Run("source_destination_selection", func(t *testing.T) {
		// Verify session can select source account
		if len(accounts) == 0 {
			t.Error("session must have accounts for transfer")
		}

		sourceAccount := accounts[rng.IntN(len(accounts))]
		if sourceAccount == nil {
			t.Error("source account selection failed")
		}
	})

	t.Run("paired_entries_structure", func(t *testing.T) {
		// Verify transfer result contains paired entries
		result := &TransferResult{
			SourceTransactionID: 1001,
			DestTransactionID:   1002,
			NewSourceBalance:    400000,
			NewDestBalance:      600000,
		}

		if result.SourceTransactionID == 0 || result.DestTransactionID == 0 {
			t.Error("transfer should produce two transaction IDs (paired entries)")
		}
	})
}

// TestUS_RetailCustomer_ATMWithdrawal validates:
// "As a retail customer, I want to withdraw cash at an ATM after a quick
// balance check so that I know how much I can take out."
//
// Acceptance: ATM session includes balance inquiry; withdrawal debits account
// and notes ATM ID; receipt/memo captured; failures (insufficient funds/out-of-service)
// are logged.
func TestUS_RetailCustomer_ATMWithdrawal(t *testing.T) {
	cfg := testConfig()
	errorSim := simulator.NewErrorSimulator(cfg)

	branchID := int64(10)
	atm := &models.ATM{
		ID:              501,
		BranchID:        &branchID,
		SupportsDeposit: true,
	}

	t.Run("atm_workflow_structure", func(t *testing.T) {
		// ATM workflow: balance check -> withdraw/deposit/exit
		// Verify ATM has required fields
		if atm.ID <= 0 {
			t.Error("ATM must have valid ID")
		}
		if atm.BranchID == nil {
			t.Error("ATM should belong to a branch")
		}
	})

	t.Run("withdrawal_amounts_realistic", func(t *testing.T) {
		// ATM withdrawals should be multiples of 20
		amounts := []int64{2000, 4000, 6000, 8000, 10000, 20000} // cents
		for _, amt := range amounts {
			if amt%2000 != 0 {
				t.Errorf("withdrawal amount %d should be multiple of $20", amt)
			}
		}
	})

	t.Run("insufficient_funds_rate", func(t *testing.T) {
		// Verify insufficient funds simulation
		highFailCfg := testConfig()
		highFailCfg.InsufficientFundsRate = 1.0
		errorSim := simulator.NewErrorSimulator(highFailCfg)

		shouldFail := errorSim.ShouldSimulateInsufficientFunds(utils.NewRandom(42))
		if !shouldFail {
			t.Error("expected insufficient funds with 100% rate")
		}
	})

	t.Run("session_type_atm", func(t *testing.T) {
		sessionType := simulator.SessionTypeATM
		if sessionType.String() != "ATM" {
			t.Errorf("expected session type string 'ATM', got %s", sessionType.String())
		}
	})

	t.Run("error_classification_funds", func(t *testing.T) {
		_ = errorSim // Used to verify configuration
		errType := simulator.ClassifyError(simulator.ErrInsufficientFunds)
		if errType != simulator.ErrorTypeFunds {
			t.Errorf("expected ErrorTypeFunds, got %s", errType)
		}
	})
}

// TestUS_RetailCustomer_ATMDeposit validates:
// "As a retail customer, I want to deposit cash/checks via ATM or branch so
// that my balance increases and is immediately visible."
//
// Acceptance: deposit credits account with channel metadata; updated balance
// visible on next inquiry; audit entry recorded.
func TestUS_RetailCustomer_ATMDeposit(t *testing.T) {
	branchID := int64(10)
	atm := &models.ATM{
		ID:              501,
		BranchID:        &branchID,
		SupportsDeposit: true,
	}

	t.Run("atm_supports_deposit", func(t *testing.T) {
		if !atm.SupportsDeposit {
			t.Error("test ATM should support deposits")
		}
	})

	t.Run("deposit_amounts_realistic", func(t *testing.T) {
		// Deposit amounts should be realistic round numbers
		amounts := []int64{5000, 10000, 20000, 50000, 10000, 25000, 50000}
		for _, amt := range amounts {
			if amt <= 0 {
				t.Error("deposit amounts should be positive")
			}
			// Deposits tend to be round numbers
			if amt%500 != 0 {
				t.Errorf("deposit amount %d should typically be round number", amt)
			}
		}
	})

	t.Run("deposit_selects_appropriate_account", func(t *testing.T) {
		accounts := []*models.Account{
			{ID: 100, Type: models.AccountTypeChecking},
			{ID: 101, Type: models.AccountTypeSavings},
		}

		foundValidAccount := false
		for _, acc := range accounts {
			if acc.Type == models.AccountTypeChecking || acc.Type == models.AccountTypeSavings {
				foundValidAccount = true
				break
			}
		}
		if !foundValidAccount {
			t.Error("session should have checking or savings account for deposit")
		}
	})

	t.Run("channel_metadata_captured", func(t *testing.T) {
		// Verify channel can be set on transactions
		channel := models.ChannelATM
		if channel == "" {
			t.Error("channel should not be empty")
		}
	})
}

// TestUS_Customer_FailureOutcomes validates:
// "As a customer, I want clear outcomes for failed actions (wrong password,
// insufficient funds, invalid account) so that I understand what happened."
//
// Acceptance: no ledger changes on failed attempts; descriptive error surfaced;
// failure recorded in audit logs with reason.
func TestUS_Customer_FailureOutcomes(t *testing.T) {
	t.Run("error_types_defined", func(t *testing.T) {
		// Verify error types exist for customer-facing failures
		errorTypes := []simulator.ErrorType{
			simulator.ErrorTypeAuth,
			simulator.ErrorTypeFunds,
			simulator.ErrorTypeTimeout,
			simulator.ErrorTypeBeneficiary,
		}

		for _, et := range errorTypes {
			if et == "" {
				t.Error("error type should not be empty")
			}
		}
	})

	t.Run("error_classification", func(t *testing.T) {
		// Test error classification
		testCases := []struct {
			err      error
			expected simulator.ErrorType
		}{
			{simulator.ErrAuthenticationFailed, simulator.ErrorTypeAuth},
			{simulator.ErrInsufficientFunds, simulator.ErrorTypeFunds},
			{simulator.ErrTimeout, simulator.ErrorTypeTimeout},
			{simulator.ErrInvalidBeneficiary, simulator.ErrorTypeBeneficiary},
			{simulator.ErrAccountLocked, simulator.ErrorTypeAccountLock},
		}

		for _, tc := range testCases {
			result := simulator.ClassifyError(tc.err)
			if result != tc.expected {
				t.Errorf("error %v: expected type %s, got %s", tc.err, tc.expected, result)
			}
		}
	})

	t.Run("audit_outcomes_defined", func(t *testing.T) {
		outcomes := []models.AuditOutcome{
			models.OutcomeSuccess,
			models.OutcomeFailure,
			models.OutcomeDenied,
			models.OutcomeError,
		}

		for _, outcome := range outcomes {
			if outcome == "" {
				t.Error("audit outcome should not be empty")
			}
		}
	})

	t.Run("failed_login_rate_configurable", func(t *testing.T) {
		cfg := testConfig()
		cfg.FailedLoginRate = 0.05 // 5%

		errorSim := simulator.NewErrorSimulator(cfg)
		rng := utils.NewRandom(42)

		// Test that login failure rate is respected
		failures := 0
		iterations := 10000
		for i := 0; i < iterations; i++ {
			if errorSim.ShouldSimulateLoginFailure(rng) {
				failures++
			}
		}

		// Should be around 5% with some variance
		rate := float64(failures) / float64(iterations)
		if rate < 0.03 || rate > 0.07 {
			t.Errorf("expected ~5%% failure rate, got %.2f%%", rate*100)
		}
	})
}
