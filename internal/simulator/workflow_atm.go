// Package simulator provides live banking session simulation for load testing.
//
// FILE: workflow_atm.go
// PURPOSE: ATM session workflow implementation. Handles the typical ATM session
// pattern: balance check, then withdraw/deposit/exit.
//
// KEY FUNCTIONS:
// - RunATMWorkflow: Executes a complete ATM session
// - withdraw: Performs ATM cash withdrawal
// - deposit: Performs ATM cash/check deposit
//
// RELATED FILES:
// - state.go: Base session types and authentication
// - workflow_online.go: Online banking workflow
// - workflow_business.go: Business customer workflow
// - session_helpers.go: Shared helper functions
package simulator

import (
	"fmt"
	"os"

	"github.com/willfong/load-generator/internal/models"
)

// RunATMWorkflow executes a typical ATM session
// ATM sessions follow realistic patterns: most users check balance, then either
// withdraw cash (most common), deposit funds, or just check balance and leave
func (s *CustomerSession) RunATMWorkflow() {
	// Step 1: Balance inquiry (most ATM users check balance first)
	if err := s.checkBalance(); err != nil {
		if IsInfrastructureError(err) {
			fmt.Fprintf(os.Stderr, "\nFatal: ATM balance check failed: %v\n", err)
			os.Exit(1)
		}
		s.metrics.RecordError(ClassifyError(err))
		return
	}
	s.thinkTime()

	// Step 2: Decide on action based on realistic probabilities
	// ~75% withdraw, ~10% deposit (if ATM supports it), ~15% just balance check
	action := s.rng.Float64()

	switch {
	case action < 0.75: // 75% withdrawal
		if err := s.withdraw(); err != nil {
			if IsInfrastructureError(err) {
				fmt.Fprintf(os.Stderr, "\nFatal: ATM withdrawal failed: %v\n", err)
				os.Exit(1)
			}
			if err.Error() == "insufficient funds" {
				s.recordAuditLog(models.AuditTransactionDeclined, models.OutcomeDenied, nil, "Insufficient funds")
			}
			// Simulated errors are already recorded in withdraw()
		}

	case action < 0.85: // 10% deposit (if ATM supports deposits)
		if s.ATM != nil && s.ATM.SupportsDeposit {
			if err := s.deposit(); err != nil {
				if IsInfrastructureError(err) {
					fmt.Fprintf(os.Stderr, "\nFatal: ATM deposit failed: %v\n", err)
					os.Exit(1)
				}
				// Simulated errors are already recorded in deposit()
			}
		}

	// remaining 15% just checked balance and leave
	}

	// Session complete
	s.recordAuditLog(models.AuditSessionEnded, models.OutcomeSuccess, nil, "")
}

// withdraw performs an ATM withdrawal
func (s *CustomerSession) withdraw() error {
	if len(s.Accounts) == 0 {
		return fmt.Errorf("no accounts available")
	}

	// Pick a checking or savings account
	var account *models.Account
	for _, acc := range s.Accounts {
		if acc.Type == models.AccountTypeChecking || acc.Type == models.AccountTypeSavings {
			account = acc
			break
		}
	}
	if account == nil {
		account = s.Accounts[0]
	}

	// Check for simulated insufficient funds
	if s.errorSim.ShouldSimulateInsufficientFunds(s.rng) {
		s.recordAuditLog(models.AuditTransactionDeclined, models.OutcomeDenied, &account.ID, "Insufficient funds (simulated)")
		s.metrics.RecordError(ErrorTypeFunds)
		return ErrInsufficientFunds
	}

	// Generate realistic withdrawal amount (multiples of 20)
	amounts := []int64{2000, 4000, 6000, 8000, 10000, 20000} // cents
	amount := amounts[s.rng.IntN(len(amounts))]

	start := s.startTimer()

	ctx, cancel := s.timeoutContext(10)
	defer cancel()

	description := fmt.Sprintf("ATM Withdrawal - Session %s", s.ID[:8])

	var atmID *int64
	if s.ATM != nil {
		atmID = &s.ATM.ID
	}

	txnID, err := s.queries.ExecuteWithdrawal(ctx, account.ID, amount, atmID, description)
	latency := s.elapsed(start)

	if err != nil {
		errStr := err.Error()
		if len(errStr) >= 17 && errStr[:17] == "insufficient fund" {
			s.recordAuditLog(models.AuditTransactionDeclined, models.OutcomeDenied, &account.ID, "Insufficient funds")
			s.metrics.RecordError(ErrorTypeFunds)
			return ErrInsufficientFunds
		}
		// Any other error from the database is infrastructure failure
		if IsInfrastructureError(err) {
			fmt.Fprintf(os.Stderr, "\nFatal: withdrawal transaction failed: %v\n", err)
			os.Exit(1)
		}
		s.recordAuditLog(models.AuditTransactionFailed, models.OutcomeFailure, &account.ID, err.Error())
		s.metrics.RecordError(ClassifyError(err))
		return err
	}

	s.recordAuditLog(models.AuditTransactionCompleted, models.OutcomeSuccess, &account.ID,
		fmt.Sprintf("Withdrawal $%.2f, txn=%d", float64(amount)/100, txnID))
	s.metrics.RecordOperation(OpWithdrawal, true, latency)
	return nil
}

// deposit performs an ATM deposit
func (s *CustomerSession) deposit() error {
	if len(s.Accounts) == 0 {
		return fmt.Errorf("no accounts available")
	}

	// Pick a checking or savings account for deposit
	var account *models.Account
	for _, acc := range s.Accounts {
		if acc.Type == models.AccountTypeChecking || acc.Type == models.AccountTypeSavings {
			account = acc
			break
		}
	}
	if account == nil {
		account = s.Accounts[0]
	}

	// Generate realistic deposit amount
	// Deposits tend to be rounder numbers and larger than withdrawals
	amounts := []int64{5000, 10000, 20000, 50000, 10000, 25000, 50000} // cents
	amount := amounts[s.rng.IntN(len(amounts))]

	start := s.startTimer()

	ctx, cancel := s.timeoutContext(10)
	defer cancel()

	description := fmt.Sprintf("ATM Deposit - Session %s", s.ID[:8])

	var atmID *int64
	if s.ATM != nil {
		atmID = &s.ATM.ID
	}

	txnID, err := s.queries.ExecuteDeposit(ctx, account.ID, amount, atmID, models.ChannelATM, description)
	latency := s.elapsed(start)

	if err != nil {
		// Deposit errors from database are infrastructure failures
		if IsInfrastructureError(err) {
			fmt.Fprintf(os.Stderr, "\nFatal: deposit transaction failed: %v\n", err)
			os.Exit(1)
		}
		s.recordAuditLog(models.AuditTransactionFailed, models.OutcomeFailure, &account.ID, err.Error())
		s.metrics.RecordError(ClassifyError(err))
		return err
	}

	s.recordAuditLog(models.AuditTransactionCompleted, models.OutcomeSuccess, &account.ID,
		fmt.Sprintf("Deposit $%.2f, txn=%d", float64(amount)/100, txnID))
	s.metrics.RecordOperation(OpDeposit, true, latency)
	return nil
}
