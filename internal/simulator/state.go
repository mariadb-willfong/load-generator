// Package simulator provides live banking session simulation for load testing.
//
// FILE: state.go
// PURPOSE: Core session types, state machine definitions, and authentication.
// This file defines the CustomerSession struct and session lifecycle management.
//
// KEY TYPES:
// - SessionState: State machine states (Initialized, Authenticating, etc.)
// - SessionType: Session channel types (ATM, Online, Business)
// - CustomerSession: Main session struct with all dependencies
//
// KEY FUNCTIONS:
// - Authenticate: Verifies customer credentials
//
// RELATED FILES:
// - workflow_atm.go: ATM session workflow
// - workflow_online.go: Online banking workflow
// - workflow_business.go: Business customer workflow
// - session_helpers.go: Shared helper functions
package simulator

import (
	"context"
	"time"

	"github.com/willfong/load-generator/internal/config"
	"github.com/willfong/load-generator/internal/database"
	"github.com/willfong/load-generator/internal/models"
	"github.com/willfong/load-generator/internal/utils"
)

// SessionState represents the current state of a customer session
type SessionState int

const (
	StateInitialized SessionState = iota
	StateAuthenticating
	StateAuthenticated
	StateBrowsing
	StateTransacting
	StateEnded
	StateFailed
)

// SessionType represents the type of banking session
type SessionType int

const (
	SessionTypeATM SessionType = iota
	SessionTypeOnline
	SessionTypeBusiness
)

func (st SessionType) String() string {
	switch st {
	case SessionTypeATM:
		return "ATM"
	case SessionTypeOnline:
		return "Online"
	case SessionTypeBusiness:
		return "Business"
	default:
		return "Unknown"
	}
}

// CustomerSession represents an active banking session
type CustomerSession struct {
	// Session identity
	ID       string
	WorkerID int

	// Customer and accounts
	Customer *models.Customer
	Accounts []*models.Account

	// Session characteristics
	Type      SessionType
	State     SessionState
	StartTime time.Time
	EndTime   time.Time

	// For ATM sessions
	ATM *models.ATM

	// Dependencies
	rng         *utils.Random
	queries     *database.Queries
	config      config.SimulateConfig
	metrics     *EnhancedMetrics
	errorSim    *ErrorSimulator
	auditWriter *AuditWriter
	ctx         context.Context
}

// Authenticate simulates login or PIN verification
func (s *CustomerSession) Authenticate() bool {
	s.State = StateAuthenticating
	start := time.Now()

	// Simulate failed login using error simulator
	if s.errorSim.ShouldSimulateLoginFailure(s.rng) {
		s.recordAuditLog(models.AuditLoginFailed, models.OutcomeFailure, nil, "Invalid credentials")
		s.metrics.RecordError(ErrorTypeAuth)
		s.errorSim.RecordError(ErrorTypeAuth)
		s.State = StateFailed
		s.thinkTime()
		return false
	}

	// Successful authentication
	s.recordAuditLog(models.AuditLoginSuccess, models.OutcomeSuccess, nil, "")
	s.metrics.RecordOperation(OpLogin, false, time.Since(start))
	s.State = StateAuthenticated
	s.thinkTime()
	return true
}
