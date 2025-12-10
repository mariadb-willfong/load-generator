#!/bin/bash
# Bank-in-a-Box Load Generator - Setup and Load Script
#
# Usage:
#   ./scripts/setup_and_load.sh [output_dir]
#
# Default output_dir is ./output
#
# Prerequisites:
#   - MariaDB/MySQL running and accessible
#   - Data generated via: ./loadgen generate --output ./output

set -e

OUTPUT_DIR="${1:-./output}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=== Bank-in-a-Box Data Loader ==="
echo "Output directory: $OUTPUT_DIR"
echo ""

# Check if CSV files exist
if [ ! -f "$OUTPUT_DIR/customers.csv" ]; then
    echo "Error: CSV files not found in $OUTPUT_DIR"
    echo "Run './loadgen generate --output $OUTPUT_DIR' first"
    exit 1
fi

# Count rows in each file
echo "Files to load:"
for f in branches atms customers accounts beneficiaries transactions audit_logs; do
    if [ -f "$OUTPUT_DIR/${f}.csv" ]; then
        count=$(($(wc -l < "$OUTPUT_DIR/${f}.csv") - 1))
        printf "  %-15s %'d rows\n" "$f:" "$count"
    fi
done
echo ""

# Prompt for confirmation
read -p "Continue with loading? [y/N] " -n 1 -r
echo ""
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Aborted."
    exit 0
fi

# Database connection
echo ""
echo "Enter MariaDB/MySQL credentials:"
read -p "  Host [localhost]: " DB_HOST
DB_HOST="${DB_HOST:-localhost}"
read -p "  Port [3306]: " DB_PORT
DB_PORT="${DB_PORT:-3306}"
read -p "  User [root]: " DB_USER
DB_USER="${DB_USER:-root}"
read -s -p "  Password: " DB_PASS
echo ""

# Build MySQL command
MYSQL_CMD="mysql -h $DB_HOST -P $DB_PORT -u $DB_USER"
if [ -n "$DB_PASS" ]; then
    MYSQL_CMD="$MYSQL_CMD -p$DB_PASS"
fi

# Step 1: Create schema (no indexes)
echo ""
echo "Step 1: Creating database schema..."
$MYSQL_CMD < "$PROJECT_DIR/internal/database/schema_no_indexes.sql"
echo "  Done."

# Step 2: Load data
echo ""
echo "Step 2: Loading data..."
cd "$OUTPUT_DIR"
$MYSQL_CMD --local-infile=1 bank < "$PROJECT_DIR/scripts/load_data.sql"
cd - > /dev/null
echo "  Done."

# Step 3: Create indexes
echo ""
echo "Step 3: Creating indexes (this may take a while)..."
$MYSQL_CMD bank < "$PROJECT_DIR/internal/database/schema_indexes.sql"
echo "  Done."

# Final summary
echo ""
echo "=== Loading Complete ==="
$MYSQL_CMD bank -e "
SELECT
    (SELECT COUNT(*) FROM branches) AS branches,
    (SELECT COUNT(*) FROM atms) AS atms,
    (SELECT COUNT(*) FROM customers) AS customers,
    (SELECT COUNT(*) FROM accounts) AS accounts,
    (SELECT COUNT(*) FROM beneficiaries) AS beneficiaries,
    (SELECT COUNT(*) FROM transactions) AS transactions,
    (SELECT COUNT(*) FROM audit_logs) AS audit_logs;
"
echo ""
echo "Database 'bank' is ready!"
