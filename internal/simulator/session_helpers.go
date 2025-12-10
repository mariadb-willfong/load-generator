// Package simulator provides live banking session simulation for load testing.
//
// FILE: session_helpers.go
// PURPOSE: Shared helper functions for customer sessions including balance checks,
// timing utilities, audit logging, and context generation.
//
// KEY FUNCTIONS:
// - checkBalance: Queries balance for primary account
// - checkBalanceForAccount: Queries balance for specific account
// - thinkTime: Waits for realistic user delay
// - recordAuditLog: Creates audit log entries
// - generateFakeIP: Creates plausible IP addresses
// - generateUserAgent: Returns user agent strings
// - startTimer/elapsed: Timing utilities
// - timeoutContext: Creates context with timeout
//
// RELATED FILES:
// - state.go: Base session types
// - workflow_atm.go: Uses these helpers
// - workflow_online.go: Uses these helpers
// - workflow_business.go: Uses these helpers
package simulator

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/willfong/load-generator/internal/models"
)

// checkBalance queries the balance of the primary account
func (s *CustomerSession) checkBalance() error {
	if len(s.Accounts) == 0 {
		return fmt.Errorf("no accounts available")
	}
	return s.checkBalanceForAccount(s.Accounts[0])
}

// checkBalanceForAccount queries the balance of a specific account
func (s *CustomerSession) checkBalanceForAccount(account *models.Account) error {
	start := s.startTimer()

	// Check for simulated timeout
	if s.errorSim.ShouldSimulateTimeout(s.rng) {
		s.recordAuditLog(models.AuditBalanceInquiry, models.OutcomeFailure, &account.ID, "Operation timeout")
		s.metrics.RecordError(ErrorTypeTimeout)
		return ErrTimeout
	}

	ctx, cancel := s.timeoutContext(5)
	defer cancel()

	_, err := s.queries.GetAccountBalance(ctx, account.ID)
	latency := s.elapsed(start)

	if err != nil {
		if IsInfrastructureError(err) {
			fmt.Fprintf(os.Stderr, "\nFatal: balance inquiry failed: %v\n", err)
			os.Exit(1)
		}
		s.recordAuditLog(models.AuditBalanceInquiry, models.OutcomeFailure, &account.ID, err.Error())
		s.metrics.RecordError(ClassifyError(err))
		return err
	}

	s.recordAuditLog(models.AuditBalanceInquiry, models.OutcomeSuccess, &account.ID, "")
	s.metrics.RecordOperation(OpBalanceCheck, false, latency)
	return nil
}

// thinkTime waits for a realistic delay between user actions
func (s *CustomerSession) thinkTime() {
	minMs := s.config.MinThinkTime.Milliseconds()
	maxMs := s.config.MaxThinkTime.Milliseconds()

	delayMs := minMs + s.rng.Int64N(maxMs-minMs+1)
	delay := time.Duration(delayMs) * time.Millisecond

	select {
	case <-time.After(delay):
	case <-s.ctx.Done():
	}
}

// recordAuditLog creates an audit log entry for the action
func (s *CustomerSession) recordAuditLog(action models.AuditAction, outcome models.AuditOutcome, accountID *int64, reason string) {
	var channel models.AuditChannel
	switch s.Type {
	case SessionTypeATM:
		channel = models.AuditChannelATM
	case SessionTypeOnline:
		channel = models.AuditChannelOnline
	default:
		channel = models.AuditChannelAPI
	}

	// Build options for the audit log
	opts := []AuditOption{
		WithIP(s.generateFakeIP()),
		WithUserAgent(s.generateUserAgent()),
	}

	if accountID != nil {
		opts = append(opts, WithAccount(*accountID))
	}

	if reason != "" {
		opts = append(opts, WithFailureReason(reason))
	}

	if s.ATM != nil {
		opts = append(opts, WithATM(s.ATM.ID))
	}

	// Use the buffered audit writer instead of direct insert
	s.auditWriter.AuditAction(s.Customer, s.ID, action, outcome, channel, opts...)
}

// generateFakeIP creates a plausible IP address for the session
func (s *CustomerSession) generateFakeIP() string {
	// Generate IP based on session type
	if s.Type == SessionTypeATM {
		// ATM internal network range
		return fmt.Sprintf("10.%d.%d.%d", s.rng.IntN(256), s.rng.IntN(256), s.rng.IntN(256))
	}
	// External customer IPs
	return fmt.Sprintf("%d.%d.%d.%d",
		s.rng.IntN(223)+1, // Avoid 0.x.x.x and 224+
		s.rng.IntN(256),
		s.rng.IntN(256),
		s.rng.IntN(254)+1)
}

// generateUserAgent returns a user agent string based on session type
func (s *CustomerSession) generateUserAgent() string {
	if s.Type == SessionTypeATM {
		return "ATM/2.0 (NCR-6622)"
	}

	agents := []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15",
		"Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36",
		"BankApp/3.5.0 (iOS 17.0)",
		"BankApp/3.5.0 (Android 14)",
	}
	return agents[s.rng.IntN(len(agents))]
}

// startTimer returns the current time for timing operations
func (s *CustomerSession) startTimer() time.Time {
	return time.Now()
}

// elapsed returns the duration since start
func (s *CustomerSession) elapsed(start time.Time) time.Duration {
	return time.Since(start)
}

// timeoutContext creates a context with the specified timeout in seconds
func (s *CustomerSession) timeoutContext(seconds int) (context.Context, context.CancelFunc) {
	return context.WithTimeout(s.ctx, time.Duration(seconds)*time.Second)
}
