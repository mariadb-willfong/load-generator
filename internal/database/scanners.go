// Package database provides database operations for the load generator simulation.
//
// FILE: scanners.go
// PURPOSE: Row scanning helper functions for converting database rows to model structs.
//
// KEY FUNCTIONS:
// - scanCustomer: Scans a customer row
// - scanAccount: Scans an account from sql.Rows
// - scanAccountRow: Scans an account from sql.Row
// - scanTransaction: Scans a transaction row
// - scanATM: Scans an ATM row
// - generateReferenceNumber: Creates unique transaction reference numbers
//
// RELATED FILES:
// - queries_customer.go: Uses scanCustomer
// - queries_account.go: Uses scanAccount, scanAccountRow
// - queries_transaction.go: Uses scanTransaction
// - queries_atm.go: Uses scanATM
package database

import (
	"database/sql"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/willfong/load-generator/internal/models"
)

// refCounter provides unique suffix for reference numbers to avoid collisions
var refCounter atomic.Uint64

func scanCustomer(row *sql.Row) (*models.Customer, error) {
	c := &models.Customer{}

	// Nullable fields need sql.Null* types for scanning
	var (
		phone        sql.NullString
		dateOfBirth  sql.NullTime
		addressLine1 sql.NullString
		addressLine2 sql.NullString
		city         sql.NullString
		state        sql.NullString
		postalCode   sql.NullString
	)

	err := row.Scan(
		&c.ID, &c.FirstName, &c.LastName, &c.Email, &phone, &dateOfBirth,
		&addressLine1, &addressLine2, &city, &state, &postalCode, &c.Country,
		&c.Timezone, &c.HomeBranch, &c.Segment, &c.Status, &c.ActivityScore,
		&c.Username, &c.PasswordHash, &c.PIN, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	// Convert nullable fields to their values (empty string/zero time if NULL)
	c.Phone = phone.String
	c.DateOfBirth = dateOfBirth.Time
	c.AddressLine1 = addressLine1.String
	c.AddressLine2 = addressLine2.String
	c.City = city.String
	c.State = state.String
	c.PostalCode = postalCode.String

	return c, nil
}

func scanAccount(rows *sql.Rows) (*models.Account, error) {
	a := &models.Account{}
	err := rows.Scan(
		&a.ID, &a.AccountNumber, &a.CustomerID, &a.Type, &a.Status, &a.Currency,
		&a.Balance, &a.CreditLimit, &a.OverdraftLimit,
		&a.DailyWithdrawLimit, &a.DailyTransferLimit, &a.InterestRate,
		&a.BranchID, &a.OpenedAt, &a.ClosedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func scanAccountRow(row *sql.Row) (*models.Account, error) {
	a := &models.Account{}
	err := row.Scan(
		&a.ID, &a.AccountNumber, &a.CustomerID, &a.Type, &a.Status, &a.Currency,
		&a.Balance, &a.CreditLimit, &a.OverdraftLimit,
		&a.DailyWithdrawLimit, &a.DailyTransferLimit, &a.InterestRate,
		&a.BranchID, &a.OpenedAt, &a.ClosedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func scanTransaction(rows *sql.Rows) (*models.Transaction, error) {
	t := &models.Transaction{}

	// Nullable string fields
	var (
		description sql.NullString
		metadata    sql.NullString
	)

	err := rows.Scan(
		&t.ID, &t.ReferenceNumber, &t.AccountID, &t.CounterpartyAccountID, &t.BeneficiaryID,
		&t.Type, &t.Status, &t.Channel, &t.Amount, &t.Currency, &t.BalanceAfter,
		&description, &metadata, &t.BranchID, &t.ATMID, &t.LinkedTransactionID,
		&t.Timestamp, &t.PostedAt, &t.ValueDate, &t.FailureReason,
	)
	if err != nil {
		return nil, err
	}

	t.Description = description.String
	t.Metadata = metadata.String

	return t, nil
}

func scanATM(row *sql.Row) (*models.ATM, error) {
	a := &models.ATM{}
	err := row.Scan(
		&a.ID, &a.ATMID, &a.BranchID, &a.Status, &a.LocationName,
		&a.AddressLine1, &a.City, &a.State, &a.PostalCode, &a.Country,
		&a.Latitude, &a.Longitude, &a.Timezone,
		&a.SupportsDeposit, &a.SupportsTransfer, &a.Is24Hours,
		&a.AvgDailyTransactions, &a.InstalledAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func generateReferenceNumber(timestamp time.Time, accountID int64) string {
	counter := refCounter.Add(1)
	return fmt.Sprintf("TXN%s%06d%06d", timestamp.Format("20060102150405"), accountID%1000000, counter%1000000)
}
