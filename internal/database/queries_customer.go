// Package database provides database operations for the load generator simulation.
//
// FILE: queries_customer.go
// PURPOSE: Customer-related database queries including customer lookup,
// authentication, and timezone information retrieval.
//
// KEY FUNCTIONS:
// - GetRandomCustomer: Selects a random active customer
// - GetCustomerByID: Retrieves customer by ID
// - AuthenticateCustomer: Verifies login credentials
// - GetAllCustomerTimezones: Gets timezone info for scheduling
//
// RELATED FILES:
// - queries.go: Base Queries struct and NewQueries constructor
// - queries_account.go: Account operations
// - scanners.go: Row scanning utilities
package database

import (
	"context"
	"database/sql"

	"github.com/willfong/load-generator/internal/models"
)

// GetRandomCustomer selects a random active customer for simulation
func (q *Queries) GetRandomCustomer(ctx context.Context) (*models.Customer, error) {
	query := `
		SELECT id, first_name, last_name, email, phone, date_of_birth,
			address_line1, address_line2, city, state, postal_code, country,
			timezone, home_branch_id, segment, status, activity_score,
			username, password_hash, pin, created_at, updated_at
		FROM customers
		WHERE status = 'active'
		ORDER BY RAND()
		LIMIT 1`

	row := q.pool.QueryRowContext(ctx, query)
	return scanCustomer(row)
}

// GetCustomerByID retrieves a customer by their ID
func (q *Queries) GetCustomerByID(ctx context.Context, customerID int64) (*models.Customer, error) {
	query := `
		SELECT id, first_name, last_name, email, phone, date_of_birth,
			address_line1, address_line2, city, state, postal_code, country,
			timezone, home_branch_id, segment, status, activity_score,
			username, password_hash, pin, created_at, updated_at
		FROM customers
		WHERE id = ?`

	row := q.pool.QueryRowContext(ctx, query, customerID)
	return scanCustomer(row)
}

// AuthenticateCustomer verifies login credentials
// Returns customer if successful, nil if credentials don't match
func (q *Queries) AuthenticateCustomer(ctx context.Context, username, passwordHash string) (*models.Customer, error) {
	query := `
		SELECT id, first_name, last_name, email, phone, date_of_birth,
			address_line1, address_line2, city, state, postal_code, country,
			timezone, home_branch_id, segment, status, activity_score,
			username, password_hash, pin, created_at, updated_at
		FROM customers
		WHERE username = ? AND password_hash = ? AND status = 'active'`

	row := q.pool.QueryRowContext(ctx, query, username, passwordHash)
	customer, err := scanCustomer(row)
	if err == sql.ErrNoRows {
		return nil, nil // Authentication failed
	}
	return customer, err
}

// CustomerTimezoneInfo contains minimal customer data for timezone-based scheduling
type CustomerTimezoneInfo struct {
	ID       int64
	Timezone string
}

// GetAllCustomerTimezones retrieves all active customer IDs with their timezones.
// This is used by the scheduler to build a weighted selection cache.
func (q *Queries) GetAllCustomerTimezones(ctx context.Context) ([]CustomerTimezoneInfo, error) {
	query := `SELECT id, timezone FROM customers WHERE status = 'active'`

	rows, err := q.pool.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var customers []CustomerTimezoneInfo
	for rows.Next() {
		var c CustomerTimezoneInfo
		if err := rows.Scan(&c.ID, &c.Timezone); err != nil {
			return nil, err
		}
		customers = append(customers, c)
	}
	return customers, rows.Err()
}
