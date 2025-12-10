// Package simulator provides live banking session simulation for load testing.
//
// FILE: workflow_business.go
// PURPOSE: Business customer session workflow implementation. Handles business-specific
// patterns including account reviews, batch payroll, and treasury sweeps.
//
// KEY FUNCTIONS:
// - RunBusinessWorkflow: Executes a complete business session
// - isPayrollPeriod: Checks if we're in payroll window
// - executeBatchPayroll: Performs batch salary payments
// - executeAccountSweep: Moves excess funds between accounts
// - generateTransferAmount: Creates segment-appropriate transfer amounts
//
// RELATED FILES:
// - state.go: Base session types and authentication
// - workflow_atm.go: ATM session workflow
// - workflow_online.go: Online banking workflow
// - session_helpers.go: Shared helper functions
package simulator

import (
	"fmt"
	"os"
	"time"

	"github.com/willfong/load-generator/internal/database"
	"github.com/willfong/load-generator/internal/models"
)

// RunBusinessWorkflow executes a business account session
// Business sessions have distinct patterns: account reviews, batch payments (payroll),
// account sweeps (cash management), and individual vendor payments
func (s *CustomerSession) RunBusinessWorkflow() {
	s.State = StateBrowsing

	// Step 1: Check balances across all accounts (business customers review all accounts)
	for _, account := range s.Accounts {
		select {
		case <-s.ctx.Done():
			return
		default:
		}
		s.checkBalanceForAccount(account)
		s.thinkTime()
	}

	// Step 2: View recent transaction history for primary account
	s.viewTransactionHistory()
	s.thinkTime()

	// Step 3: Decide on business action based on activity patterns
	// Payroll days (25-28) have higher batch payment probability
	action := s.rng.Float64()
	isPayrollPeriod := s.isPayrollPeriod()

	if isPayrollPeriod && action < 0.4 {
		// 40% chance of batch payroll during payroll period
		s.State = StateTransacting
		s.executeBatchPayroll()
		s.State = StateBrowsing
		s.thinkTime()
	} else if action < 0.3 {
		// 30% chance of account sweep (cash management)
		s.State = StateTransacting
		s.executeAccountSweep()
		s.State = StateBrowsing
		s.thinkTime()
	}

	// Step 4: Execute vendor payments / transfers
	numTransfers := 1 + s.rng.IntN(3) // 1-3 vendor payments
	for i := 0; i < numTransfers; i++ {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		s.State = StateTransacting
		s.executeTransfer()
		s.State = StateBrowsing
		s.thinkTime()
	}

	// Session complete
	s.recordAuditLog(models.AuditSessionEnded, models.OutcomeSuccess, nil, "")
}

// isPayrollPeriod checks if we're in the payroll window (days 25-28 of month)
func (s *CustomerSession) isPayrollPeriod() bool {
	day := time.Now().Day()
	return day >= 25 && day <= 28
}

// executeBatchPayroll performs a batch payroll payment (for business sessions)
// This simulates a company paying multiple employees in a single batch
func (s *CustomerSession) executeBatchPayroll() error {
	if len(s.Accounts) == 0 {
		return fmt.Errorf("no accounts available")
	}

	// Find a suitable payroll source account (business/corporate account)
	var sourceAccount *models.Account
	for _, acc := range s.Accounts {
		if acc.Type == models.AccountTypeBusiness || acc.Type == models.AccountTypePayroll {
			sourceAccount = acc
			break
		}
	}
	if sourceAccount == nil {
		sourceAccount = s.Accounts[0] // Fall back to first account
	}

	ctx, cancel := s.timeoutContext(30)
	defer cancel()

	// Get employee accounts to pay (5-20 employees per batch)
	numEmployees := 5 + s.rng.IntN(16)
	employeeAccounts, err := s.queries.GetEmployeeAccounts(ctx, numEmployees)
	if err != nil {
		if IsInfrastructureError(err) {
			fmt.Fprintf(os.Stderr, "\nFatal: failed to get employee accounts: %v\n", err)
			os.Exit(1)
		}
	}
	if err != nil || len(employeeAccounts) == 0 {
		s.recordAuditLog(models.AuditTransactionFailed, models.OutcomeFailure, &sourceAccount.ID, "No employee accounts found")
		s.metrics.RecordError(ErrorTypeDatabase)
		return fmt.Errorf("no employee accounts found")
	}

	// Build payment batch with realistic salary amounts
	payments := make([]database.PayrollPayment, len(employeeAccounts))
	for i, empAcctID := range employeeAccounts {
		// Realistic salary range: $2,000 - $8,000 per pay period
		salary := int64(200000 + s.rng.IntN(600000)) // cents
		payments[i] = database.PayrollPayment{
			DestAccountID: empAcctID,
			Amount:        salary,
		}
	}

	start := s.startTimer()
	description := fmt.Sprintf("Payroll Batch - Session %s", s.ID[:8])

	result, err := s.queries.ExecuteBatchPayroll(ctx, sourceAccount.ID, payments, description)
	latency := s.elapsed(start)

	if err != nil {
		errStr := err.Error()
		if len(errStr) >= 17 && errStr[:17] == "insufficient fund" {
			s.recordAuditLog(models.AuditTransactionDeclined, models.OutcomeDenied, &sourceAccount.ID, "Insufficient funds for payroll")
			s.metrics.RecordError(ErrorTypeFunds)
		} else {
			if IsInfrastructureError(err) {
				fmt.Fprintf(os.Stderr, "\nFatal: batch payroll transaction failed: %v\n", err)
				os.Exit(1)
			}
			s.recordAuditLog(models.AuditTransactionFailed, models.OutcomeFailure, &sourceAccount.ID, err.Error())
			s.metrics.RecordError(ClassifyError(err))
		}
		return err
	}

	s.recordAuditLog(models.AuditTransactionCompleted, models.OutcomeSuccess, &sourceAccount.ID,
		fmt.Sprintf("Payroll batch: %d employees, total $%.2f",
			result.SuccessCount, float64(result.TotalAmount)/100))
	s.metrics.RecordOperation(OpBatchPayroll, true, latency)
	return nil
}

// executeAccountSweep performs an automated cash sweep between accounts
// This simulates treasury management where excess funds are moved to savings/investment
func (s *CustomerSession) executeAccountSweep() error {
	if len(s.Accounts) < 2 {
		return fmt.Errorf("need at least 2 accounts for sweep")
	}

	// Find source (checking) and destination (savings) accounts
	var sourceAccount, destAccount *models.Account
	for _, acc := range s.Accounts {
		if sourceAccount == nil && (acc.Type == models.AccountTypeChecking || acc.Type == models.AccountTypeBusiness) {
			sourceAccount = acc
		} else if destAccount == nil && (acc.Type == models.AccountTypeSavings || acc.Type == models.AccountTypeInvestment) {
			destAccount = acc
		}
	}

	// Fall back if we couldn't find specific types
	if sourceAccount == nil {
		sourceAccount = s.Accounts[0]
	}
	if destAccount == nil {
		for _, acc := range s.Accounts {
			if acc.ID != sourceAccount.ID {
				destAccount = acc
				break
			}
		}
	}
	if destAccount == nil {
		return fmt.Errorf("no destination account for sweep")
	}

	// Target balance: keep some operating funds in checking
	// Sweep anything above target to savings
	targetBalance := int64(500000 + s.rng.IntN(1000000)) // $5,000 - $15,000

	ctx, cancel := s.timeoutContext(10)
	defer cancel()

	start := s.startTimer()
	description := fmt.Sprintf("Account Sweep - Session %s", s.ID[:8])

	result, err := s.queries.ExecuteSweep(ctx, sourceAccount.ID, destAccount.ID, targetBalance, description)
	latency := s.elapsed(start)

	if err != nil {
		// "No excess funds" is expected sometimes - not an error
		errStr := err.Error()
		if len(errStr) >= 14 && errStr[:14] == "no excess fund" {
			return nil // Silent success - nothing to sweep
		}
		if IsInfrastructureError(err) {
			fmt.Fprintf(os.Stderr, "\nFatal: account sweep transaction failed: %v\n", err)
			os.Exit(1)
		}
		s.recordAuditLog(models.AuditTransactionFailed, models.OutcomeFailure, &sourceAccount.ID, err.Error())
		s.metrics.RecordError(ClassifyError(err))
		return err
	}

	sweepAmount := result.NewDestBalance - result.NewSourceBalance
	s.recordAuditLog(models.AuditTransactionCompleted, models.OutcomeSuccess, &sourceAccount.ID,
		fmt.Sprintf("Sweep $%.2f to savings, new balance $%.2f",
			float64(sweepAmount)/100, float64(result.NewSourceBalance)/100))
	s.metrics.RecordOperation(OpAccountSweep, true, latency)
	return nil
}

// generateTransferAmount creates a realistic transfer amount based on customer segment
func (s *CustomerSession) generateTransferAmount() int64 {
	// Different ranges based on customer segment
	var minAmount, maxAmount int64

	switch s.Customer.Segment {
	case models.SegmentPrivate:
		minAmount = 100000   // $1,000
		maxAmount = 5000000  // $50,000
	case models.SegmentPremium:
		minAmount = 10000    // $100
		maxAmount = 500000   // $5,000
	case models.SegmentCorporate:
		minAmount = 500000   // $5,000
		maxAmount = 10000000 // $100,000
	case models.SegmentBusiness:
		minAmount = 50000    // $500
		maxAmount = 2000000  // $20,000
	default: // Regular
		minAmount = 500      // $5
		maxAmount = 50000    // $500
	}

	return minAmount + s.rng.Int64N(maxAmount-minAmount+1)
}
