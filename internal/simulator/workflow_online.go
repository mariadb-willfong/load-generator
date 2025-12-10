// Package simulator provides live banking session simulation for load testing.
//
// FILE: workflow_online.go
// PURPOSE: Online banking session workflow implementation. Handles varied
// online banking activity patterns including browsing and transactions.
//
// KEY FUNCTIONS:
// - RunOnlineWorkflow: Executes a complete online banking session
// - viewTransactionHistory: Queries recent transactions
// - executeTransfer: Performs an internal transfer
//
// RELATED FILES:
// - state.go: Base session types and authentication
// - workflow_atm.go: ATM session workflow
// - workflow_business.go: Business customer workflow
// - session_helpers.go: Shared helper functions
package simulator

import (
	"fmt"
	"os"

	"github.com/willfong/load-generator/internal/models"
)

// RunOnlineWorkflow executes a typical online banking session
func (s *CustomerSession) RunOnlineWorkflow() {
	s.State = StateBrowsing

	// Online sessions have more varied activity
	numActions := 2 + s.rng.IntN(4) // 2-5 actions per session

	for i := 0; i < numActions; i++ {
		select {
		case <-s.ctx.Done():
			return
		default:
		}

		// Choose action based on read/write ratio
		isRead := s.rng.Float64() < (s.config.ReadWriteRatio / (s.config.ReadWriteRatio + 1))

		if isRead {
			// Read actions: balance check, transaction history
			switch s.rng.IntN(2) {
			case 0:
				s.checkBalance()
			case 1:
				s.viewTransactionHistory()
			}
		} else {
			// Write actions: transfer
			s.State = StateTransacting
			s.executeTransfer()
			s.State = StateBrowsing
		}

		s.thinkTime()
	}

	// Session complete
	s.recordAuditLog(models.AuditSessionEnded, models.OutcomeSuccess, nil, "")
}

// viewTransactionHistory queries recent transactions
func (s *CustomerSession) viewTransactionHistory() error {
	if len(s.Accounts) == 0 {
		return fmt.Errorf("no accounts available")
	}

	account := s.Accounts[s.rng.IntN(len(s.Accounts))]
	start := s.startTimer()

	// Check for simulated timeout
	if s.errorSim.ShouldSimulateTimeout(s.rng) {
		s.recordAuditLog(models.AuditHistoryViewed, models.OutcomeFailure, &account.ID, "Operation timeout")
		s.metrics.RecordError(ErrorTypeTimeout)
		return ErrTimeout
	}

	ctx, cancel := s.timeoutContext(10)
	defer cancel()

	_, err := s.queries.GetTransactionHistory(ctx, account.ID, 20)
	latency := s.elapsed(start)

	if err != nil {
		if IsInfrastructureError(err) {
			fmt.Fprintf(os.Stderr, "\nFatal: transaction history query failed: %v\n", err)
			os.Exit(1)
		}
		s.recordAuditLog(models.AuditHistoryViewed, models.OutcomeFailure, &account.ID, err.Error())
		s.metrics.RecordError(ClassifyError(err))
		return err
	}

	s.recordAuditLog(models.AuditHistoryViewed, models.OutcomeSuccess, &account.ID, "")
	s.metrics.RecordOperation(OpHistoryView, false, latency)
	return nil
}

// executeTransfer performs an internal transfer
func (s *CustomerSession) executeTransfer() error {
	if len(s.Accounts) == 0 {
		return fmt.Errorf("no accounts available")
	}

	// Source: pick a random account with balance
	sourceAccount := s.Accounts[s.rng.IntN(len(s.Accounts))]

	// Check for simulated insufficient funds
	if s.errorSim.ShouldSimulateInsufficientFunds(s.rng) {
		s.recordAuditLog(models.AuditTransactionDeclined, models.OutcomeDenied, &sourceAccount.ID, "Insufficient funds (simulated)")
		s.metrics.RecordError(ErrorTypeFunds)
		return ErrInsufficientFunds
	}

	// Destination: get a random business account
	ctx, cancel := s.timeoutContext(10)
	defer cancel()

	destAccount, err := s.queries.GetRandomBusinessAccount(ctx)
	if err != nil {
		if IsInfrastructureError(err) {
			fmt.Fprintf(os.Stderr, "\nFatal: failed to get business account: %v\n", err)
			os.Exit(1)
		}
		// Fall back to another customer account if available
		if len(s.Accounts) > 1 {
			for _, acc := range s.Accounts {
				if acc.ID != sourceAccount.ID {
					destAccount = acc
					break
				}
			}
		}
		if destAccount == nil {
			return fmt.Errorf("no destination account available")
		}
	}

	// Generate transfer amount
	amount := s.generateTransferAmount()

	start := s.startTimer()

	description := fmt.Sprintf("Transfer - Session %s", s.ID[:8])
	channel := models.ChannelOnline
	if s.Type == SessionTypeATM {
		channel = models.ChannelATM
	}

	result, err := s.queries.ExecuteTransfer(ctx, sourceAccount.ID, destAccount.ID, amount, description, channel)
	latency := s.elapsed(start)

	if err != nil {
		errStr := err.Error()
		if len(errStr) >= 17 && errStr[:17] == "insufficient fund" {
			s.recordAuditLog(models.AuditTransactionDeclined, models.OutcomeDenied, &sourceAccount.ID, "Insufficient funds")
			s.metrics.RecordError(ErrorTypeFunds)
			return ErrInsufficientFunds
		}
		if IsInfrastructureError(err) {
			fmt.Fprintf(os.Stderr, "\nFatal: transfer transaction failed: %v\n", err)
			os.Exit(1)
		}
		s.recordAuditLog(models.AuditTransactionFailed, models.OutcomeFailure, &sourceAccount.ID, err.Error())
		s.metrics.RecordError(ClassifyError(err))
		return err
	}

	s.recordAuditLog(models.AuditTransactionCompleted, models.OutcomeSuccess, &sourceAccount.ID,
		fmt.Sprintf("Transfer $%.2f to account %d, txn=%d",
			float64(amount)/100, destAccount.ID, result.SourceTransactionID))
	s.metrics.RecordOperation(OpTransfer, true, latency)
	return nil
}
