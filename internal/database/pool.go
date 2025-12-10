package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/willfong/load-generator/internal/config"
)

// ensureParseTime adds parseTime=true to MySQL DSN if not already present.
// This is required for scanning DATE/DATETIME columns into time.Time values.
func ensureParseTime(dsn string) string {
	// Check if parseTime is already specified (case-insensitive)
	lower := strings.ToLower(dsn)
	if strings.Contains(lower, "parsetime=") {
		return dsn
	}

	// Add parseTime=true to the query string
	if strings.Contains(dsn, "?") {
		return dsn + "&parseTime=true"
	}
	return dsn + "?parseTime=true"
}

// Pool wraps a sql.DB with additional monitoring and lifecycle management
type Pool struct {
	db     *sql.DB
	config config.DatabaseConfig

	// Metrics
	totalQueries   int64
	failedQueries  int64
	totalLatencyNs int64
}

// NewPool creates a new database connection pool with the given configuration
func NewPool(cfg config.DatabaseConfig) (*Pool, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("database DSN is required")
	}

	driver := cfg.Driver
	if driver == "" {
		driver = "mysql"
	}

	// Ensure parseTime=true for MySQL to properly scan DATE/DATETIME columns
	dsn := cfg.DSN
	if driver == "mysql" {
		dsn = ensureParseTime(dsn)
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Apply pool configuration
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}

	pool := &Pool{
		db:     db,
		config: cfg,
	}

	return pool, nil
}

// Connect verifies the database connection is working
func (p *Pool) Connect(ctx context.Context) error {
	if err := p.db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}
	return nil
}

// Close gracefully shuts down the connection pool
func (p *Pool) Close() error {
	return p.db.Close()
}

// DB returns the underlying sql.DB for direct access when needed
func (p *Pool) DB() *sql.DB {
	return p.db
}

// QueryContext executes a query and returns rows
func (p *Pool) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
	start := time.Now()
	rows, err := p.db.QueryContext(ctx, query, args...)
	p.recordQuery(time.Since(start), err)
	return rows, err
}

// QueryRowContext executes a query expected to return at most one row
func (p *Pool) QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row {
	start := time.Now()
	row := p.db.QueryRowContext(ctx, query, args...)
	p.recordQuery(time.Since(start), nil)
	return row
}

// ExecContext executes a query that doesn't return rows
func (p *Pool) ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error) {
	start := time.Now()
	result, err := p.db.ExecContext(ctx, query, args...)
	p.recordQuery(time.Since(start), err)
	return result, err
}

// BeginTx starts a new transaction with the given options
func (p *Pool) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return p.db.BeginTx(ctx, opts)
}

// recordQuery updates internal metrics
func (p *Pool) recordQuery(duration time.Duration, err error) {
	p.totalQueries++
	p.totalLatencyNs += duration.Nanoseconds()
	if err != nil {
		p.failedQueries++
	}
}

// Stats returns current pool statistics
func (p *Pool) Stats() PoolStats {
	dbStats := p.db.Stats()
	return PoolStats{
		OpenConnections:  dbStats.OpenConnections,
		InUse:            dbStats.InUse,
		Idle:             dbStats.Idle,
		WaitCount:        dbStats.WaitCount,
		WaitDuration:     dbStats.WaitDuration,
		MaxIdleClosed:    dbStats.MaxIdleClosed,
		MaxLifetimeClosed: dbStats.MaxLifetimeClosed,
		TotalQueries:     p.totalQueries,
		FailedQueries:    p.failedQueries,
		AvgLatency:       p.averageLatency(),
	}
}

func (p *Pool) averageLatency() time.Duration {
	if p.totalQueries == 0 {
		return 0
	}
	return time.Duration(p.totalLatencyNs / p.totalQueries)
}

// PoolStats contains connection pool and query statistics
type PoolStats struct {
	// Connection pool stats
	OpenConnections   int
	InUse             int
	Idle              int
	WaitCount         int64
	WaitDuration      time.Duration
	MaxIdleClosed     int64
	MaxLifetimeClosed int64

	// Query stats
	TotalQueries  int64
	FailedQueries int64
	AvgLatency    time.Duration
}
