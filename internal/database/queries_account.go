// Package database provides database operations for the load generator simulation.
//
// FILE: queries_account.go
// PURPOSE: Account-related database queries including account lookup, balance checks,
// withdrawals, deposits, transfers, and sweep operations.
//
// KEY FUNCTIONS:
// - GetCustomerAccounts: Retrieves all accounts for a customer
// - GetAccountByID: Retrieves account by ID
// - GetAccountBalance: Gets current balance
// - GetRandomBusinessAccount: Gets random business account for transfers
// - GetEmployeeAccounts: Gets sample accounts for payroll
// - ExecuteWithdrawal: Performs ATM withdrawal
// - ExecuteDeposit: Performs deposit
// - ExecuteTransfer: Performs internal transfer
// - ExecuteSweep: Moves excess funds between accounts
//
// RELATED FILES:
// - queries.go: Base Queries struct
// - queries_customer.go: Customer queries
// - queries_transaction.go: Batch operations
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/willfong/load-generator/internal/models"
)

// GetCustomerAccounts retrieves all accounts for a customer
func (q *Queries) GetCustomerAccounts(ctx context.Context, customerID int64) ([]*models.Account, error) {
	query := `
		SELECT id, account_number, customer_id, type, status, currency,
			balance, credit_limit, overdraft_limit,
			daily_withdraw_limit, daily_transfer_limit, interest_rate,
			branch_id, opened_at, closed_at, updated_at
		FROM accounts
		WHERE customer_id = ? AND status = 'active'
		ORDER BY type, id`

	rows, err := q.pool.QueryContext(ctx, query, customerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query accounts: %w", err)
	}
	defer rows.Close()

	var accounts []*models.Account
	for rows.Next() {
		account, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, rows.Err()
}

// GetAccountByID retrieves an account by ID
func (q *Queries) GetAccountByID(ctx context.Context, accountID int64) (*models.Account, error) {
	query := `
		SELECT id, account_number, customer_id, type, status, currency,
			balance, credit_limit, overdraft_limit,
			daily_withdraw_limit, daily_transfer_limit, interest_rate,
			branch_id, opened_at, closed_at, updated_at
		FROM accounts
		WHERE id = ?`

	row := q.pool.QueryRowContext(ctx, query, accountID)
	return scanAccountRow(row)
}

// GetAccountBalance retrieves just the current balance for an account
func (q *Queries) GetAccountBalance(ctx context.Context, accountID int64) (int64, error) {
	query := `SELECT balance FROM accounts WHERE id = ?`

	var balance int64
	err := q.pool.QueryRowContext(ctx, query, accountID).Scan(&balance)
	if err != nil {
		return 0, fmt.Errorf("failed to get balance: %w", err)
	}
	return balance, nil
}

// GetRandomBusinessAccount gets a random active business/merchant account for transfers
func (q *Queries) GetRandomBusinessAccount(ctx context.Context) (*models.Account, error) {
	query := `
		SELECT id, account_number, customer_id, type, status, currency,
			balance, credit_limit, overdraft_limit,
			daily_withdraw_limit, daily_transfer_limit, interest_rate,
			branch_id, opened_at, closed_at, updated_at
		FROM accounts
		WHERE type IN ('business', 'merchant', 'payroll') AND status = 'active'
		ORDER BY RAND()
		LIMIT 1`

	row := q.pool.QueryRowContext(ctx, query)
	return scanAccountRow(row)
}

// GetEmployeeAccounts retrieves a sample of customer accounts for payroll simulation
func (q *Queries) GetEmployeeAccounts(ctx context.Context, limit int) ([]int64, error) {
	query := `
		SELECT id FROM accounts
		WHERE type = 'checking' AND status = 'active'
		ORDER BY RAND()
		LIMIT ?`

	rows, err := q.pool.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query employee accounts: %w", err)
	}
	defer rows.Close()

	var accounts []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		accounts = append(accounts, id)
	}
	return accounts, rows.Err()
}

// ExecuteWithdrawal performs a cash withdrawal from an account
func (q *Queries) ExecuteWithdrawal(ctx context.Context, accountID, amount int64, atmID *int64, description string) (int64, error) {
	tx, err := q.pool.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()

	// Lock and check account
	var balance int64
	var currency string
	err = tx.QueryRowContext(ctx,
		`SELECT balance, currency FROM accounts WHERE id = ? FOR UPDATE`,
		accountID,
	).Scan(&balance, &currency)
	if err != nil {
		return 0, fmt.Errorf("failed to lock account: %w", err)
	}

	if balance < amount {
		return 0, fmt.Errorf("insufficient funds: balance %d, requested %d", balance, amount)
	}

	newBalance := balance - amount

	// Update account balance
	_, err = tx.ExecContext(ctx,
		`UPDATE accounts SET balance = ?, updated_at = ? WHERE id = ?`,
		newBalance, now, accountID,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to update account: %w", err)
	}

	// Insert withdrawal transaction
	ref := generateReferenceNumber(now, accountID)
	result, err := tx.ExecContext(ctx, `
		INSERT INTO transactions (
			reference_number, account_id, type, status, channel,
			amount, currency, balance_after, description, atm_id,
			timestamp, posted_at, value_date
		) VALUES (?, ?, 'withdrawal', 'completed', 'atm', ?, ?, ?, ?, ?, ?, ?, ?)`,
		ref, accountID, amount, currency, newBalance, description, atmID, now, now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert transaction: %w", err)
	}

	transactionID, _ := result.LastInsertId()

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit withdrawal: %w", err)
	}

	return transactionID, nil
}

// ExecuteDeposit performs a cash/check deposit to an account
func (q *Queries) ExecuteDeposit(ctx context.Context, accountID, amount int64, atmID *int64, channel models.TransactionChannel, description string) (int64, error) {
	tx, err := q.pool.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()

	// Lock account
	var balance int64
	var currency string
	err = tx.QueryRowContext(ctx,
		`SELECT balance, currency FROM accounts WHERE id = ? FOR UPDATE`,
		accountID,
	).Scan(&balance, &currency)
	if err != nil {
		return 0, fmt.Errorf("failed to lock account: %w", err)
	}

	newBalance := balance + amount

	// Update account balance
	_, err = tx.ExecContext(ctx,
		`UPDATE accounts SET balance = ?, updated_at = ? WHERE id = ?`,
		newBalance, now, accountID,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to update account: %w", err)
	}

	// Insert deposit transaction
	ref := generateReferenceNumber(now, accountID)
	result, err := tx.ExecContext(ctx, `
		INSERT INTO transactions (
			reference_number, account_id, type, status, channel,
			amount, currency, balance_after, description, atm_id,
			timestamp, posted_at, value_date
		) VALUES (?, ?, 'deposit', 'completed', ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ref, accountID, channel, amount, currency, newBalance, description, atmID, now, now, now,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to insert transaction: %w", err)
	}

	transactionID, _ := result.LastInsertId()

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit deposit: %w", err)
	}

	return transactionID, nil
}

// TransferResult contains the results of a transfer operation
type TransferResult struct {
	SourceTransactionID int64
	DestTransactionID   int64
	NewSourceBalance    int64
	NewDestBalance      int64
}

// ExecuteTransfer performs an internal transfer between two accounts
// This uses a transaction to ensure atomicity
func (q *Queries) ExecuteTransfer(ctx context.Context, fromAccountID, toAccountID, amount int64, description string, channel models.TransactionChannel) (*TransferResult, error) {
	tx, err := q.pool.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()

	// Lock and check source account
	var sourceBalance int64
	var sourceCurrency string
	err = tx.QueryRowContext(ctx,
		`SELECT balance, currency FROM accounts WHERE id = ? FOR UPDATE`,
		fromAccountID,
	).Scan(&sourceBalance, &sourceCurrency)
	if err != nil {
		return nil, fmt.Errorf("failed to lock source account: %w", err)
	}

	if sourceBalance < amount {
		return nil, fmt.Errorf("insufficient funds: balance %d, requested %d", sourceBalance, amount)
	}

	// Lock destination account
	var destBalance int64
	err = tx.QueryRowContext(ctx,
		`SELECT balance FROM accounts WHERE id = ? FOR UPDATE`,
		toAccountID,
	).Scan(&destBalance)
	if err != nil {
		return nil, fmt.Errorf("failed to lock destination account: %w", err)
	}

	// Calculate new balances
	newSourceBalance := sourceBalance - amount
	newDestBalance := destBalance + amount

	// Update source account
	_, err = tx.ExecContext(ctx,
		`UPDATE accounts SET balance = ?, updated_at = ? WHERE id = ?`,
		newSourceBalance, now, fromAccountID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update source account: %w", err)
	}

	// Update destination account
	_, err = tx.ExecContext(ctx,
		`UPDATE accounts SET balance = ?, updated_at = ? WHERE id = ?`,
		newDestBalance, now, toAccountID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update destination account: %w", err)
	}

	// Generate reference numbers
	refSource := generateReferenceNumber(now, fromAccountID)
	refDest := generateReferenceNumber(now, toAccountID)

	// Insert debit transaction (source)
	sourceResult, err := tx.ExecContext(ctx, `
		INSERT INTO transactions (
			reference_number, account_id, counterparty_account_id, type, status, channel,
			amount, currency, balance_after, description, timestamp, posted_at, value_date
		) VALUES (?, ?, ?, 'transfer_out', 'completed', ?, ?, ?, ?, ?, ?, ?, ?)`,
		refSource, fromAccountID, toAccountID, channel,
		amount, sourceCurrency, newSourceBalance, description, now, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert source transaction: %w", err)
	}

	sourceTransactionID, _ := sourceResult.LastInsertId()

	// Insert credit transaction (destination)
	destResult, err := tx.ExecContext(ctx, `
		INSERT INTO transactions (
			reference_number, account_id, counterparty_account_id, type, status, channel,
			amount, currency, balance_after, description, linked_transaction_id,
			timestamp, posted_at, value_date
		) VALUES (?, ?, ?, 'transfer_in', 'completed', ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		refDest, toAccountID, fromAccountID, channel,
		amount, sourceCurrency, newDestBalance, description, sourceTransactionID, now, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert destination transaction: %w", err)
	}

	destTransactionID, _ := destResult.LastInsertId()

	// Update source transaction with linked ID
	_, err = tx.ExecContext(ctx,
		`UPDATE transactions SET linked_transaction_id = ? WHERE id = ?`,
		destTransactionID, sourceTransactionID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to link transactions: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transfer: %w", err)
	}

	return &TransferResult{
		SourceTransactionID: sourceTransactionID,
		DestTransactionID:   destTransactionID,
		NewSourceBalance:    newSourceBalance,
		NewDestBalance:      newDestBalance,
	}, nil
}

// ExecuteSweep moves excess funds from one account to another (cash management)
func (q *Queries) ExecuteSweep(ctx context.Context, fromAccountID, toAccountID, targetBalance int64, description string) (*TransferResult, error) {
	tx, err := q.pool.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()

	// Lock source account
	var sourceBalance int64
	var currency string
	err = tx.QueryRowContext(ctx,
		`SELECT balance, currency FROM accounts WHERE id = ? FOR UPDATE`,
		fromAccountID,
	).Scan(&sourceBalance, &currency)
	if err != nil {
		return nil, fmt.Errorf("failed to lock source account: %w", err)
	}

	// Calculate sweep amount (move excess above target to destination)
	sweepAmount := sourceBalance - targetBalance
	if sweepAmount <= 0 {
		return nil, fmt.Errorf("no excess funds to sweep: balance %d, target %d", sourceBalance, targetBalance)
	}

	// Lock destination account
	var destBalance int64
	err = tx.QueryRowContext(ctx,
		`SELECT balance FROM accounts WHERE id = ? FOR UPDATE`,
		toAccountID,
	).Scan(&destBalance)
	if err != nil {
		return nil, fmt.Errorf("failed to lock destination account: %w", err)
	}

	newSourceBalance := targetBalance
	newDestBalance := destBalance + sweepAmount

	// Update source
	_, err = tx.ExecContext(ctx,
		`UPDATE accounts SET balance = ?, updated_at = ? WHERE id = ?`,
		newSourceBalance, now, fromAccountID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update source: %w", err)
	}

	// Update destination
	_, err = tx.ExecContext(ctx,
		`UPDATE accounts SET balance = ?, updated_at = ? WHERE id = ?`,
		newDestBalance, now, toAccountID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update destination: %w", err)
	}

	// Insert sweep transactions
	refSource := generateReferenceNumber(now, fromAccountID)
	sourceResult, err := tx.ExecContext(ctx, `
		INSERT INTO transactions (
			reference_number, account_id, counterparty_account_id, type, status, channel,
			amount, currency, balance_after, description, timestamp, posted_at, value_date
		) VALUES (?, ?, ?, 'transfer_out', 'completed', 'internal', ?, ?, ?, ?, ?, ?, ?)`,
		refSource, fromAccountID, toAccountID, sweepAmount, currency,
		newSourceBalance, description, now, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert source transaction: %w", err)
	}
	sourceTransactionID, _ := sourceResult.LastInsertId()

	refDest := generateReferenceNumber(now, toAccountID)
	destResult, err := tx.ExecContext(ctx, `
		INSERT INTO transactions (
			reference_number, account_id, counterparty_account_id, type, status, channel,
			amount, currency, balance_after, description, linked_transaction_id,
			timestamp, posted_at, value_date
		) VALUES (?, ?, ?, 'transfer_in', 'completed', 'internal', ?, ?, ?, ?, ?, ?, ?, ?)`,
		refDest, toAccountID, fromAccountID, sweepAmount, currency,
		newDestBalance, description, sourceTransactionID, now, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to insert dest transaction: %w", err)
	}
	destTransactionID, _ := destResult.LastInsertId()

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit sweep: %w", err)
	}

	return &TransferResult{
		SourceTransactionID: sourceTransactionID,
		DestTransactionID:   destTransactionID,
		NewSourceBalance:    newSourceBalance,
		NewDestBalance:      newDestBalance,
	}, nil
}
