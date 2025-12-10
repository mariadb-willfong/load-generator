// Package database provides database operations for the load generator simulation.
//
// FILE: queries.go
// PURPOSE: Base Queries struct and constructor. This is the entry point for all
// database operations in the simulator.
//
// KEY TYPES:
// - Queries: Main struct holding database pool connection
//
// RELATED FILES:
// - queries_customer.go: Customer lookup and authentication
// - queries_account.go: Account operations (balance, withdraw, deposit, transfer)
// - queries_transaction.go: Transaction history and batch operations
// - queries_atm.go: ATM queries
// - queries_audit.go: Audit log insertion
// - scanners.go: Row scanning helper functions
package database

// Queries provides database operations for the simulation
type Queries struct {
	pool *Pool
}

// NewQueries creates a new Queries instance
func NewQueries(pool *Pool) *Queries {
	return &Queries{pool: pool}
}
