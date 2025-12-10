// Package database provides database operations for the load generator simulation.
//
// FILE: queries_audit.go
// PURPOSE: Audit log database operations.
//
// KEY FUNCTIONS:
// - InsertAuditLog: Records an audit event
//
// RELATED FILES:
// - queries.go: Base Queries struct
package database

import (
	"context"

	"github.com/willfong/load-generator/internal/models"
)

// InsertAuditLog records an audit event
func (q *Queries) InsertAuditLog(ctx context.Context, log *models.AuditLog) error {
	query := `
		INSERT INTO audit_logs (
			timestamp, customer_id, session_id, action, outcome,
			channel, branch_id, atm_id, ip_address, user_agent,
			account_id, transaction_id, beneficiary_id,
			description, failure_reason, metadata, risk_score, request_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := q.pool.ExecContext(ctx, query,
		log.Timestamp, log.CustomerID, log.SessionID, log.Action, log.Outcome,
		log.Channel, log.BranchID, log.ATMID, log.IPAddress, log.UserAgent,
		log.AccountID, log.TransactionID, log.BeneficiaryID,
		log.Description, log.FailureReason, log.Metadata, log.RiskScore, log.RequestID,
	)
	return err
}
