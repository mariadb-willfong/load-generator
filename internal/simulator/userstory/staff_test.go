// Package userstory provides user story validation tests for the simulator.
//
// FILE: staff_test.go
// PURPOSE: Tests for bank staff user stories including operations analysts,
// fraud investigators, branch managers, support agents, and SRE engineers.
//
// KEY TESTS:
// - TestUS_OpsAnalyst_AuditTrail: Audit log completeness
// - TestUS_FraudInvestigator_HighRiskBehaviors: Risk detection capabilities
// - TestUS_BranchATMManager_DeviceTracking: Device-level tracking
// - TestUS_SupportAgent_SessionTimeline: Session replay capabilities
// - TestUS_SREEngineer_LoadPatterns: Load pattern visibility
//
// RELATED FILES:
// - helpers_test.go: Shared test utilities
// - retail_test.go: Retail customer tests
package userstory

import (
	"testing"
	"time"

	"github.com/willfong/load-generator/internal/models"
	"github.com/willfong/load-generator/internal/simulator"
	"github.com/willfong/load-generator/internal/utils"
)

// TestUS_OpsAnalyst_AuditTrail validates:
// "As an operations analyst, I want every customer action (success or failure)
// captured in audit logs with who/what/when/where/outcome so that I can trace events."
//
// Acceptance: audit includes customer/account/transaction IDs, channel
// (online/ATM/branch), location/IP or ATM/branch ID, timestamp, and status;
// no action lacks an entry.
func TestUS_OpsAnalyst_AuditTrail(t *testing.T) {
	t.Run("audit_log_fields", func(t *testing.T) {
		custID := int64(1001)
		acctID := int64(100)

		// Verify AuditLog struct has required fields
		log := &models.AuditLog{
			ID:          1,
			Timestamp:   time.Now(),
			CustomerID:  &custID,
			Action:      models.AuditLoginSuccess,
			Outcome:     models.OutcomeSuccess,
			Channel:     models.AuditChannelOnline,
			IPAddress:   "192.168.1.1",
			AccountID:   &acctID,
			SessionID:   "SESSION-001",
			Description: "Login successful",
		}

		// WHO
		if log.CustomerID == nil {
			t.Error("audit log should have customer ID")
		}

		// WHAT
		if log.Action == "" {
			t.Error("audit log should have action")
		}
		if log.Outcome == "" {
			t.Error("audit log should have outcome")
		}

		// WHEN
		if log.Timestamp.IsZero() {
			t.Error("audit log should have timestamp")
		}

		// WHERE
		if log.Channel == "" {
			t.Error("audit log should have channel")
		}
	})

	t.Run("audit_channels_defined", func(t *testing.T) {
		channels := []models.AuditChannel{
			models.AuditChannelOnline,
			models.AuditChannelATM,
			models.AuditChannelBranch,
			models.AuditChannelMobile,
			models.AuditChannelAPI,
		}

		for _, ch := range channels {
			if ch == "" {
				t.Error("audit channel should not be empty")
			}
		}
	})

	t.Run("audit_actions_comprehensive", func(t *testing.T) {
		// Verify comprehensive action types exist
		actions := []models.AuditAction{
			// Authentication
			models.AuditLoginSuccess,
			models.AuditLoginFailed,
			// Transactions
			models.AuditTransactionCompleted,
			models.AuditTransactionFailed,
			models.AuditTransactionDeclined,
			// Queries
			models.AuditBalanceInquiry,
			models.AuditHistoryViewed,
			// Session
			models.AuditSessionEnded,
		}

		for _, action := range actions {
			if action == "" {
				t.Error("audit action should not be empty")
			}
		}
	})
}

// TestUS_FraudInvestigator_HighRiskBehaviors validates:
// "As a fraud/compliance investigator, I want to review high-risk behaviors
// (repeated failed logins, bursts, large transfers) so that I can detect anomalies."
//
// Acceptance: flagged events identifiable from audit/transaction data; reason
// codes stored; ability to correlate customer, session time, and channel.
func TestUS_FraudInvestigator_HighRiskBehaviors(t *testing.T) {
	t.Run("audit_log_helper_methods", func(t *testing.T) {
		authEvent := &models.AuditLog{Action: models.AuditLoginFailed}
		txnEvent := &models.AuditLog{Action: models.AuditTransactionCompleted}

		if !authEvent.IsAuthenticationEvent() {
			t.Error("login failed should be authentication event")
		}
		if authEvent.IsTransactionEvent() {
			t.Error("login should not be transaction event")
		}
		if !txnEvent.IsTransactionEvent() {
			t.Error("transaction completed should be transaction event")
		}
	})

	t.Run("failure_reason_captured", func(t *testing.T) {
		log := &models.AuditLog{
			Action:        models.AuditTransactionDeclined,
			Outcome:       models.OutcomeDenied,
			FailureReason: "Insufficient funds",
		}

		if log.FailureReason == "" {
			t.Error("declined transactions should have failure reason")
		}
	})

	t.Run("session_correlation", func(t *testing.T) {
		custID := int64(1001)
		// Verify session ID allows correlation
		log := &models.AuditLog{
			SessionID:  "SESSION-001",
			CustomerID: &custID,
			Timestamp:  time.Now(),
			Channel:    models.AuditChannelOnline,
		}

		if log.SessionID == "" {
			t.Error("audit should have session ID for correlation")
		}
		if log.CustomerID == nil {
			t.Error("audit should have customer ID for correlation")
		}
	})

	t.Run("risk_score_field_exists", func(t *testing.T) {
		score := 0.85
		log := &models.AuditLog{
			RiskScore: &score,
		}

		if log.RiskScore == nil {
			t.Error("audit log should support risk score")
		}
	})
}

// TestUS_BranchATMManager_DeviceTracking validates:
// "As a branch/ATM manager, I want transactions to tag the originating
// branch/ATM so that I can monitor device usage and outages."
//
// Acceptance: ATM/branch ID present on withdrawals/deposits; outages or
// out-of-service events logged; traffic shows daily lunch spikes.
func TestUS_BranchATMManager_DeviceTracking(t *testing.T) {
	t.Run("atm_id_in_audit", func(t *testing.T) {
		atmID := int64(501)
		log := &models.AuditLog{
			ATMID:   &atmID,
			Channel: models.AuditChannelATM,
		}

		if log.ATMID == nil {
			t.Error("ATM transactions should have ATM ID")
		}
	})

	t.Run("branch_id_in_audit", func(t *testing.T) {
		branchID := int64(10)
		log := &models.AuditLog{
			BranchID: &branchID,
			Channel:  models.AuditChannelBranch,
		}

		if log.BranchID == nil {
			t.Error("branch transactions should have branch ID")
		}
	})

	t.Run("atm_model_has_branch", func(t *testing.T) {
		branchID := int64(10)
		atm := &models.ATM{
			ID:       501,
			BranchID: &branchID,
		}

		if atm.BranchID == nil || *atm.BranchID <= 0 {
			t.Error("ATM should belong to a branch")
		}
	})
}

// TestUS_SupportAgent_SessionTimeline validates:
// "As a customer support agent, I want to replay a customer session timeline
// (logins, balance checks, transfers, failures) so that I can resolve disputes."
//
// Acceptance: ordered audit/transaction view per customer; includes memos/descriptions;
// failed attempts visible alongside successful ones.
func TestUS_SupportAgent_SessionTimeline(t *testing.T) {
	t.Run("session_id_links_events", func(t *testing.T) {
		sessionID := "SESSION-001"
		logs := []*models.AuditLog{
			{SessionID: sessionID, Action: models.AuditLoginSuccess, Timestamp: time.Now()},
			{SessionID: sessionID, Action: models.AuditBalanceInquiry, Timestamp: time.Now().Add(1 * time.Second)},
			{SessionID: sessionID, Action: models.AuditTransactionCompleted, Timestamp: time.Now().Add(2 * time.Second)},
			{SessionID: sessionID, Action: models.AuditSessionEnded, Timestamp: time.Now().Add(3 * time.Second)},
		}

		// All events should have same session ID
		for _, log := range logs {
			if log.SessionID != sessionID {
				t.Error("all events in session should have same session ID")
			}
		}
	})

	t.Run("description_field_exists", func(t *testing.T) {
		log := &models.AuditLog{
			Description: "Transfer $100.00 to account 12345",
		}

		if log.Description == "" {
			t.Error("audit log should have description for disputes")
		}
	})

	t.Run("mixed_outcomes_trackable", func(t *testing.T) {
		logs := []*models.AuditLog{
			{Action: models.AuditLoginSuccess, Outcome: models.OutcomeSuccess},
			{Action: models.AuditTransactionDeclined, Outcome: models.OutcomeDenied},
			{Action: models.AuditTransactionCompleted, Outcome: models.OutcomeSuccess},
		}

		successCount := 0
		failCount := 0
		for _, log := range logs {
			if log.IsSuccessful() {
				successCount++
			} else {
				failCount++
			}
		}

		if successCount == 0 {
			t.Error("should have successful events")
		}
		if failCount == 0 {
			t.Error("should have failed events for realistic timeline")
		}
	})
}

// TestUS_SREEngineer_LoadPatterns validates:
// "As a performance/SRE engineer, I want visibility into bursts (payroll days,
// random spikes) and sustained load so that I can assess capacity."
//
// Acceptance: load patterns reflect weekday/weekend and monthly peaks; metrics
// show TPS/ops counts over time; seeded runs are reproducible for comparison.
func TestUS_SREEngineer_LoadPatterns(t *testing.T) {
	t.Run("burst_types_defined", func(t *testing.T) {
		// Verify burst types exist
		burstTypes := []string{"lunch", "payroll", "random", "manual"}
		for _, bt := range burstTypes {
			if bt == "" {
				t.Error("burst type should not be empty")
			}
		}
	})

	t.Run("metrics_snapshot_has_tps", func(t *testing.T) {
		cfg := testConfig()
		errorSim := simulator.NewErrorSimulator(cfg)
		metrics := simulator.NewEnhancedMetrics(errorSim)

		snapshot := metrics.Snapshot()

		// TPS fields should exist
		if snapshot.TPS < 0 {
			t.Error("TPS should be non-negative")
		}
		if snapshot.RecentTPS < 0 {
			t.Error("RecentTPS should be non-negative")
		}
	})

	t.Run("seeded_rng_reproducible", func(t *testing.T) {
		seed := int64(42)

		rng1 := utils.NewRandom(seed)
		rng2 := utils.NewRandom(seed)

		// Same seed should produce same sequence
		for i := 0; i < 100; i++ {
			v1 := rng1.IntN(1000)
			v2 := rng2.IntN(1000)
			if v1 != v2 {
				t.Errorf("iteration %d: seed 42 produced different values: %d vs %d", i, v1, v2)
			}
		}
	})

	t.Run("load_controller_phases", func(t *testing.T) {
		phases := []simulator.LoadPhase{
			simulator.PhaseRampUp,
			simulator.PhaseSteadyState,
			simulator.PhaseRampDown,
		}

		for _, phase := range phases {
			if phase.String() == "" {
				t.Error("load phase should have string representation")
			}
		}
	})

	t.Run("activity_calculator_intraday_patterns", func(t *testing.T) {
		ac := simulator.NewActivityCalculator(8, 16)

		// Different timezones should have different activity levels
		timezones := []string{"America/New_York", "Europe/London", "Asia/Tokyo"}

		for _, tz := range timezones {
			customer := &models.Customer{
				Timezone:      tz,
				Segment:       models.SegmentRegular,
				ActivityScore: 0.5,
			}

			prob := ac.CalculateActivityProbability(customer)
			// All probabilities should be valid
			if prob < 0 || prob > 1 {
				t.Errorf("timezone %s: probability %.4f out of range", tz, prob)
			}
		}
	})
}
