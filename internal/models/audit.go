package models

import (
	"time"
)

// AuditAction represents the type of action being logged
type AuditAction string

const (
	// Authentication actions
	AuditLoginSuccess    AuditAction = "login_success"
	AuditLoginFailed     AuditAction = "login_failed"
	AuditLogout          AuditAction = "logout"
	AuditPINSuccess      AuditAction = "pin_success"
	AuditPINFailed       AuditAction = "pin_failed"
	AuditPasswordChanged AuditAction = "password_changed"
	AuditAccountLocked   AuditAction = "account_locked"

	// Transaction actions
	AuditTransactionInitiated AuditAction = "transaction_initiated"
	AuditTransactionCompleted AuditAction = "transaction_completed"
	AuditTransactionFailed    AuditAction = "transaction_failed"
	AuditTransactionDeclined  AuditAction = "transaction_declined"

	// Account actions
	AuditAccountOpened     AuditAction = "account_opened"
	AuditAccountClosed     AuditAction = "account_closed"
	AuditAccountUpdated    AuditAction = "account_updated"
	AuditBeneficiaryAdded  AuditAction = "beneficiary_added"
	AuditBeneficiaryRemoved AuditAction = "beneficiary_removed"

	// Profile actions
	AuditProfileViewed   AuditAction = "profile_viewed"
	AuditProfileUpdated  AuditAction = "profile_updated"
	AuditAddressChanged  AuditAction = "address_changed"
	AuditContactChanged  AuditAction = "contact_changed"

	// Session actions
	AuditSessionStarted  AuditAction = "session_started"
	AuditSessionEnded    AuditAction = "session_ended"
	AuditSessionTimeout  AuditAction = "session_timeout"

	// Query actions
	AuditBalanceInquiry    AuditAction = "balance_inquiry"
	AuditStatementViewed   AuditAction = "statement_viewed"
	AuditHistoryViewed     AuditAction = "history_viewed"
)

// AuditOutcome represents the result of the action
type AuditOutcome string

const (
	OutcomeSuccess AuditOutcome = "success"
	OutcomeFailure AuditOutcome = "failure"
	OutcomeDenied  AuditOutcome = "denied"
	OutcomeError   AuditOutcome = "error"
)

// AuditChannel represents where the action originated
type AuditChannel string

const (
	AuditChannelOnline AuditChannel = "online"
	AuditChannelATM    AuditChannel = "atm"
	AuditChannelBranch AuditChannel = "branch"
	AuditChannelMobile AuditChannel = "mobile"
	AuditChannelPhone  AuditChannel = "phone"
	AuditChannelAPI    AuditChannel = "api"
	AuditChannelSystem AuditChannel = "system"
)

// AuditLog represents an audit trail entry for compliance and security
type AuditLog struct {
	// Primary identifier
	ID int64 `db:"id" json:"id"`

	// Timestamp - when the action occurred
	Timestamp time.Time `db:"timestamp" json:"timestamp"`

	// WHO - the actor
	CustomerID *int64 `db:"customer_id" json:"customer_id"` // Customer if user action
	EmployeeID *int64 `db:"employee_id" json:"employee_id"` // Employee if staff action
	SystemID   string `db:"system_id" json:"system_id"`     // System identifier for automated actions

	// WHAT - the action
	Action  AuditAction  `db:"action" json:"action"`
	Outcome AuditOutcome `db:"outcome" json:"outcome"`

	// WHERE - the channel and location
	Channel  AuditChannel `db:"channel" json:"channel"`
	BranchID *int64       `db:"branch_id" json:"branch_id"`
	ATMID    *int64       `db:"atm_id" json:"atm_id"`
	IPAddress string      `db:"ip_address" json:"ip_address"`
	UserAgent string      `db:"user_agent" json:"user_agent"`

	// WHICH - the target entity
	AccountID     *int64 `db:"account_id" json:"account_id"`
	TransactionID *int64 `db:"transaction_id" json:"transaction_id"`
	BeneficiaryID *int64 `db:"beneficiary_id" json:"beneficiary_id"`

	// Additional context
	Description   string `db:"description" json:"description"`     // Human-readable description
	FailureReason string `db:"failure_reason" json:"failure_reason"` // If outcome is failure
	Metadata      string `db:"metadata" json:"metadata"`           // JSON for additional details

	// Session tracking
	SessionID string `db:"session_id" json:"session_id"` // Group related events

	// Risk/security scoring (optional, for fraud detection)
	RiskScore *float64 `db:"risk_score" json:"risk_score"` // 0.0-1.0

	// For linking to request traces
	RequestID string `db:"request_id" json:"request_id"`
}

// IsSuccessful returns true if the action completed successfully
func (a *AuditLog) IsSuccessful() bool {
	return a.Outcome == OutcomeSuccess
}

// IsAuthenticationEvent returns true if this is a login/auth related event
func (a *AuditLog) IsAuthenticationEvent() bool {
	switch a.Action {
	case AuditLoginSuccess, AuditLoginFailed, AuditLogout,
		AuditPINSuccess, AuditPINFailed, AuditPasswordChanged,
		AuditAccountLocked:
		return true
	default:
		return false
	}
}

// IsTransactionEvent returns true if this is a financial transaction event
func (a *AuditLog) IsTransactionEvent() bool {
	switch a.Action {
	case AuditTransactionInitiated, AuditTransactionCompleted,
		AuditTransactionFailed, AuditTransactionDeclined:
		return true
	default:
		return false
	}
}
