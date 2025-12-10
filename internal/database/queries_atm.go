// Package database provides database operations for the load generator simulation.
//
// FILE: queries_atm.go
// PURPOSE: ATM-related database queries.
//
// KEY FUNCTIONS:
// - GetRandomATM: Retrieves a random online ATM
//
// RELATED FILES:
// - queries.go: Base Queries struct
// - scanners.go: Row scanning utilities
package database

import (
	"context"

	"github.com/willfong/load-generator/internal/models"
)

// GetRandomATM retrieves a random ATM, optionally filtered by country
func (q *Queries) GetRandomATM(ctx context.Context, country string) (*models.ATM, error) {
	query := `
		SELECT id, atm_id, branch_id, status, location_name,
			address_line1, city, state, postal_code, country,
			latitude, longitude, timezone,
			supports_deposit, supports_transfer, is_24_hours,
			avg_daily_transactions, installed_at, updated_at
		FROM atms
		WHERE status = 'online'`

	if country != "" {
		query += ` AND country = ?`
		query += ` ORDER BY RAND() LIMIT 1`
		row := q.pool.QueryRowContext(ctx, query, country)
		return scanATM(row)
	}

	query += ` ORDER BY RAND() LIMIT 1`
	row := q.pool.QueryRowContext(ctx, query)
	return scanATM(row)
}
