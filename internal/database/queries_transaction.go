// Package database provides database operations for the load generator simulation.
//
// FILE: queries_transaction.go
// PURPOSE: Transaction-related database queries including transaction history
// and batch payroll operations.
//
// KEY FUNCTIONS:
// - GetTransactionHistory: Retrieves recent transactions for an account
// - ExecuteBatchPayroll: Performs batch salary payments
//
// RELATED FILES:
// - queries.go: Base Queries struct
// - queries_account.go: Account operations
// - scanners.go: Row scanning utilities
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/willfong/load-generator/internal/models"
)

// GetTransactionHistory retrieves recent transactions for an account
func (q *Queries) GetTransactionHistory(ctx context.Context, accountID int64, limit int) ([]*models.Transaction, error) {
	query := `
		SELECT id, reference_number, account_id, counterparty_account_id, beneficiary_id,
			type, status, channel, amount, currency, balance_after,
			description, metadata, branch_id, atm_id, linked_transaction_id,
			timestamp, posted_at, value_date, failure_reason
		FROM transactions
		WHERE account_id = ?
		ORDER BY timestamp DESC
		LIMIT ?`

	rows, err := q.pool.QueryContext(ctx, query, accountID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %w", err)
	}
	defer rows.Close()

	var transactions []*models.Transaction
	for rows.Next() {
		tx, err := scanTransaction(rows)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, tx)
	}
	return transactions, rows.Err()
}

// PayrollPayment represents a single payment in a batch
type PayrollPayment struct {
	DestAccountID int64
	Amount        int64
}

// BatchPayrollResult contains the results of a batch payroll operation
type BatchPayrollResult struct {
	SourceTransactionID int64
	SuccessCount        int
	FailureCount        int
	TotalAmount         int64
	NewSourceBalance    int64
}

// ExecuteBatchPayroll performs a batch of salary payments from a payroll account
// Returns the number of successful transfers and total amount transferred
func (q *Queries) ExecuteBatchPayroll(ctx context.Context, sourceAccountID int64, payments []PayrollPayment, description string) (*BatchPayrollResult, error) {
	tx, err := q.pool.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()

	// Lock and check source account
	var sourceBalance int64
	var currency string
	err = tx.QueryRowContext(ctx,
		`SELECT balance, currency FROM accounts WHERE id = ? FOR UPDATE`,
		sourceAccountID,
	).Scan(&sourceBalance, &currency)
	if err != nil {
		return nil, fmt.Errorf("failed to lock source account: %w", err)
	}

	// Calculate total needed
	var totalAmount int64
	for _, p := range payments {
		totalAmount += p.Amount
	}

	if sourceBalance < totalAmount {
		return nil, fmt.Errorf("insufficient funds: balance %d, needed %d", sourceBalance, totalAmount)
	}

	result := &BatchPayrollResult{
		SuccessCount: 0,
		TotalAmount:  0,
	}

	runningBalance := sourceBalance

	// Process each payment
	for _, payment := range payments {
		// Debit source
		runningBalance -= payment.Amount

		refSource := generateReferenceNumber(now, sourceAccountID)
		_, err := tx.ExecContext(ctx, `
			INSERT INTO transactions (
				reference_number, account_id, counterparty_account_id, type, status, channel,
				amount, currency, balance_after, description, timestamp, posted_at, value_date
			) VALUES (?, ?, ?, 'payroll_batch', 'completed', 'internal', ?, ?, ?, ?, ?, ?, ?)`,
			refSource, sourceAccountID, payment.DestAccountID, payment.Amount, currency,
			runningBalance, description, now, now, now,
		)
		if err != nil {
			result.FailureCount++
			continue // Skip failed individual payments, continue batch
		}

		// Credit destination
		_, err = tx.ExecContext(ctx,
			`UPDATE accounts SET balance = balance + ?, updated_at = ? WHERE id = ?`,
			payment.Amount, now, payment.DestAccountID,
		)
		if err != nil {
			result.FailureCount++
			continue
		}

		// Insert credit transaction for destination
		refDest := generateReferenceNumber(now, payment.DestAccountID)
		tx.ExecContext(ctx, `
			INSERT INTO transactions (
				reference_number, account_id, counterparty_account_id, type, status, channel,
				amount, currency, description, timestamp, posted_at, value_date
			) VALUES (?, ?, ?, 'salary', 'completed', 'internal', ?, ?, ?, ?, ?, ?)`,
			refDest, payment.DestAccountID, sourceAccountID, payment.Amount, currency,
			"Salary Payment", now, now, now,
		)

		result.SuccessCount++
		result.TotalAmount += payment.Amount
	}

	// Update source account final balance
	_, err = tx.ExecContext(ctx,
		`UPDATE accounts SET balance = ?, updated_at = ? WHERE id = ?`,
		runningBalance, now, sourceAccountID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update source balance: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit batch payroll: %w", err)
	}

	result.NewSourceBalance = runningBalance
	return result, nil
}
