# Bank-in-a-Box Load Generator

A realistic banking data generator and database load simulator written in Go.

## Quick Start

```bash
# Build
go build -o loadgen ./cmd/loadgen

# Generate 100k customers with 5 years of history
./loadgen generate --customers 100000 --years 5

# Run simulation against database
./loadgen simulate --concurrency 1000 --db "user:pass@tcp(localhost:3306)/bank"
```

## Building

### Current Platform

```bash
go build -o loadgen ./cmd/loadgen
```

### Cross-Platform Builds

Build binaries for specific platforms using Go's cross-compilation:

**macOS Apple Silicon (arm64):**
```bash
GOOS=darwin GOARCH=arm64 go build -o loadgen-darwin-arm64 ./cmd/loadgen
```

**Rocky Linux 10 (x86_64):**
```bash
GOOS=linux GOARCH=amd64 go build -o loadgen-linux-amd64 ./cmd/loadgen
```

### Build All Platforms

```bash
# macOS Apple Silicon
GOOS=darwin GOARCH=arm64 go build -o loadgen-darwin-arm64 ./cmd/loadgen

# Rocky Linux 10 / RHEL-compatible x86_64
GOOS=linux GOARCH=amd64 go build -o loadgen-linux-amd64 ./cmd/loadgen
```

No CGO dependencies required - cross-compilation works from any platform.

## Commands

### generate

Generate bulk historical banking data as CSV files.

```bash
./loadgen generate [flags]

Flags:
  --customers int   Number of customers (default 10000)
  --years int       Years of history (default 3)
  --output string   Output directory (default "./output")
  --seed int        Random seed for reproducibility (0 = random)
  --entities        Generate only static entities, no transactions
  --compress        Compress output with xz (creates .csv.xz files)
```

Entity counts (branches, ATMs, businesses) are derived automatically from customer count.

### simulate

Run live customer sessions against the database.

```bash
./loadgen simulate [flags]

Flags:
  --db string         Database connection string (required)
  --concurrency int   Concurrent sessions (default 100)
  --duration string   Run duration, e.g. "1h" (default: until Ctrl+C)
  --seed int          Random seed for reproducibility (0 = random)
```

### import

Import CSV data into MySQL/MariaDB using parallel LOAD DATA INFILE.

```bash
./loadgen import [flags]

Flags:
  --db string       Database connection string (required)
  --input string    Input directory containing CSV files (default "./output")
```

Automatically:
- Creates tables if they don't exist
- Loads all tables in parallel
- Decompresses .csv.xz files on-the-fly
- Creates indexes after loading

### schema

Output database schema SQL.

```bash
./loadgen schema [type]

Types:
  full      Complete schema with indexes (default)
  tables    Tables only, no indexes (for bulk loading)
  indexes   Indexes only (run after bulk load)
```

## Database Setup

### Connection String Format

The database connection string uses Go's MySQL driver DSN format:

```
username:password@tcp(hostname:port)/database
```

**Examples:**
```bash
# Local development
"root:secret@tcp(localhost:3306)/bank"

# Remote server with custom port
"admin:mypass@tcp(db.example.com:3307)/banking"

# Docker container (host.docker.internal from inside container)
"root:root@tcp(host.docker.internal:3306)/bank"
```

### Quick Setup

```bash
# Quick setup with import command (recommended)
./loadgen generate --customers 100000 --years 5 --compress
./loadgen import --db "root:secret@tcp(localhost:3306)/bank"

# Or manual setup:
./loadgen schema | mysql -u root -p bank
# ... load CSV files with LOAD DATA INFILE ...
```

## Tuning

All tunable parameters are compile-time constants in `internal/config/defaults.go`:

- Entity ratios (businesses, branches, ATMs per customer)
- Transaction patterns (payroll day, pareto ratio)
- Session distribution (ATM/Online/Business ratios)
- Burst settings (lunch, payroll, random spikes)
- Error rates (failed logins, insufficient funds, timeouts)
- Database pool settings

Edit and recompile to change behavior.

## Output Files

```
output/
├── branches.csv          # or branches.csv.xz with --compress
├── atms.csv
├── customers.csv
├── accounts.csv
├── beneficiaries.csv
├── businesses.csv
├── transactions.csv
└── audit_logs.csv
```

With `--compress`, files are xz-compressed (~90% size reduction for large datasets).

## Requirements

- Go 1.21+
- MariaDB 11.8+ or MySQL 8+
