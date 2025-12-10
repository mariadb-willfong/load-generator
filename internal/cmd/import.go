package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/spf13/cobra"
	"github.com/willfong/load-generator/internal/ui"
)

var (
	importDBConnection string
	importInputDir     string
	importMaxOpenConns int
	importMaxIdleConns int
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import CSV data into MySQL/MariaDB database",
	Long: `Import generated CSV data into a MySQL/MariaDB database using LOAD DATA LOCAL INFILE.

This command performs bulk data loading with automatic parallelization.
It handles both plain CSV files and xz-compressed files (.csv.xz).

The import process:
1. Creates tables if they don't exist
2. Disables foreign key and unique checks for speed
3. Loads all tables in parallel with progress reporting
4. Creates indexes and foreign keys after loading

Examples:
  loadgen import --db "user:pass@tcp(localhost:3306)/bank"
  loadgen import --db "user:pass@tcp(localhost:3306)/bank" --input ./my-data`,
	Run: runImport,
}

func init() {
	rootCmd.AddCommand(importCmd)

	importCmd.Flags().StringVar(&importDBConnection, "db", "", "database connection string (required)")
	importCmd.Flags().StringVar(&importInputDir, "input", "./output", "input directory containing CSV files")
	importCmd.Flags().IntVar(&importMaxOpenConns, "db-max-open", 10, "max open database connections")
	importCmd.Flags().IntVar(&importMaxIdleConns, "db-max-idle", 10, "max idle database connections")

	importCmd.MarkFlagRequired("db")
}

// tableConfig holds metadata for loading a single table
type tableConfig struct {
	name    string
	csvFile string
	loadSQL string
}

// loadResult holds the result of loading a table
type loadResult struct {
	table    string
	rows     int64
	duration time.Duration
	err      error
}

// All tables with their LOAD DATA INFILE SQL (adapted from scripts/load_data.sql)
var tablesToLoad = []tableConfig{
	{
		name:    "branches",
		csvFile: "branches",
		loadSQL: `LOAD DATA LOCAL INFILE '%s'
INTO TABLE branches
FIELDS TERMINATED BY ','
ENCLOSED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, branch_code, name, type, status, address_line1, @address_line2, city, @state,
 @postal_code, country, @latitude, @longitude, timezone, @monday_hours, @tuesday_hours,
 @wednesday_hours, @thursday_hours, @friday_hours, @saturday_hours, @sunday_hours,
 @phone, @email, customer_capacity, atm_count, opened_at, @closed_at, updated_at)
SET
    address_line2 = NULLIF(@address_line2, ''),
    state = NULLIF(@state, ''),
    postal_code = NULLIF(@postal_code, ''),
    latitude = NULLIF(@latitude, ''),
    longitude = NULLIF(@longitude, ''),
    monday_hours = NULLIF(@monday_hours, ''),
    tuesday_hours = NULLIF(@tuesday_hours, ''),
    wednesday_hours = NULLIF(@wednesday_hours, ''),
    thursday_hours = NULLIF(@thursday_hours, ''),
    friday_hours = NULLIF(@friday_hours, ''),
    saturday_hours = NULLIF(@saturday_hours, ''),
    sunday_hours = NULLIF(@sunday_hours, ''),
    phone = NULLIF(@phone, ''),
    email = NULLIF(@email, ''),
    closed_at = NULLIF(@closed_at, '')`,
	},
	{
		name:    "atms",
		csvFile: "atms",
		loadSQL: `LOAD DATA LOCAL INFILE '%s'
INTO TABLE atms
FIELDS TERMINATED BY ','
ENCLOSED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, atm_id, @branch_id, status, @location_name, address_line1, city, @state,
 @postal_code, country, @latitude, @longitude, timezone, supports_deposit,
 supports_transfer, is_24_hours, avg_daily_transactions, installed_at, updated_at)
SET
    branch_id = NULLIF(@branch_id, ''),
    location_name = NULLIF(@location_name, ''),
    state = NULLIF(@state, ''),
    postal_code = NULLIF(@postal_code, ''),
    latitude = NULLIF(@latitude, ''),
    longitude = NULLIF(@longitude, '')`,
	},
	{
		name:    "customers",
		csvFile: "customers",
		loadSQL: `LOAD DATA LOCAL INFILE '%s'
INTO TABLE customers
FIELDS TERMINATED BY ','
ENCLOSED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, first_name, last_name, email, @phone, @date_of_birth, @address_line1, @address_line2,
 @city, @state, @postal_code, country, timezone, @home_branch_id, segment, status,
 activity_score, username, password_hash, pin, created_at, updated_at)
SET
    phone = NULLIF(@phone, ''),
    date_of_birth = NULLIF(@date_of_birth, ''),
    address_line1 = NULLIF(@address_line1, ''),
    address_line2 = NULLIF(@address_line2, ''),
    city = NULLIF(@city, ''),
    state = NULLIF(@state, ''),
    postal_code = NULLIF(@postal_code, ''),
    home_branch_id = NULLIF(@home_branch_id, '')`,
	},
	{
		name:    "accounts",
		csvFile: "accounts",
		loadSQL: `LOAD DATA LOCAL INFILE '%s'
INTO TABLE accounts
FIELDS TERMINATED BY ','
ENCLOSED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, account_number, customer_id, type, status, currency, balance, credit_limit,
 overdraft_limit, daily_withdraw_limit, daily_transfer_limit, interest_rate,
 @branch_id, opened_at, @closed_at, updated_at)
SET
    branch_id = NULLIF(@branch_id, ''),
    closed_at = NULLIF(@closed_at, '')`,
	},
	{
		name:    "beneficiaries",
		csvFile: "beneficiaries",
		loadSQL: `LOAD DATA LOCAL INFILE '%s'
INTO TABLE beneficiaries
FIELDS TERMINATED BY ','
ENCLOSED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, customer_id, @nickname, name, type, status, @bank_name, @bank_code, @routing_number,
 @account_number, @iban, @address_line1, @address_line2, @city, @state, @postal_code,
 @country, currency, payment_method, @account_reference, @last_used_at, transfer_count,
 created_at, updated_at)
SET
    nickname = NULLIF(@nickname, ''),
    bank_name = NULLIF(@bank_name, ''),
    bank_code = NULLIF(@bank_code, ''),
    routing_number = NULLIF(@routing_number, ''),
    account_number = NULLIF(@account_number, ''),
    iban = NULLIF(@iban, ''),
    address_line1 = NULLIF(@address_line1, ''),
    address_line2 = NULLIF(@address_line2, ''),
    city = NULLIF(@city, ''),
    state = NULLIF(@state, ''),
    postal_code = NULLIF(@postal_code, ''),
    country = NULLIF(@country, ''),
    account_reference = NULLIF(@account_reference, ''),
    last_used_at = NULLIF(@last_used_at, '')`,
	},
	{
		name:    "transactions",
		csvFile: "transactions",
		loadSQL: `LOAD DATA LOCAL INFILE '%s'
INTO TABLE transactions
FIELDS TERMINATED BY ','
ENCLOSED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, reference_number, account_id, @counterparty_account_id, @beneficiary_id,
 type, status, channel, amount, currency, balance_after, description, @metadata,
 @branch_id, @atm_id, @linked_transaction_id, timestamp, posted_at, value_date,
 @failure_reason)
SET
    counterparty_account_id = NULLIF(@counterparty_account_id, ''),
    beneficiary_id = NULLIF(@beneficiary_id, ''),
    metadata = NULLIF(@metadata, ''),
    branch_id = NULLIF(@branch_id, ''),
    atm_id = NULLIF(@atm_id, ''),
    linked_transaction_id = NULLIF(@linked_transaction_id, ''),
    failure_reason = NULLIF(@failure_reason, '')`,
	},
	{
		name:    "audit_logs",
		csvFile: "audit_logs",
		loadSQL: `LOAD DATA LOCAL INFILE '%s'
INTO TABLE audit_logs
FIELDS TERMINATED BY ','
ENCLOSED BY '"'
LINES TERMINATED BY '\n'
IGNORE 1 LINES
(id, timestamp, @customer_id, @employee_id, @system_id, action, outcome, channel,
 @branch_id, @atm_id, @ip_address, @user_agent, @account_id, @transaction_id,
 @beneficiary_id, @description, @failure_reason, @metadata, @session_id, @risk_score,
 @request_id)
SET
    customer_id = NULLIF(@customer_id, ''),
    employee_id = NULLIF(@employee_id, ''),
    system_id = NULLIF(@system_id, ''),
    branch_id = NULLIF(@branch_id, ''),
    atm_id = NULLIF(@atm_id, ''),
    ip_address = NULLIF(@ip_address, ''),
    user_agent = NULLIF(@user_agent, ''),
    account_id = NULLIF(@account_id, ''),
    transaction_id = NULLIF(@transaction_id, ''),
    beneficiary_id = NULLIF(@beneficiary_id, ''),
    description = NULLIF(@description, ''),
    failure_reason = NULLIF(@failure_reason, ''),
    metadata = NULLIF(@metadata, ''),
    session_id = NULLIF(@session_id, ''),
    risk_score = NULLIF(@risk_score, ''),
    request_id = NULLIF(@request_id, '')`,
	},
}

func runImport(cmd *cobra.Command, args []string) {
	// Initialize UI
	u := ui.New()
	if noColor {
		u.SetNoColor(true)
	}

	fmt.Println(u.Header("Bank-in-a-Box Data Importer"))
	fmt.Println()
	fmt.Println(u.KeyValue("Database", maskDSN(importDBConnection)))
	fmt.Println(u.KeyValue("Input", importInputDir))
	fmt.Println(u.KeyValue("DB Pool", fmt.Sprintf("%d open / %d idle", importMaxOpenConns, importMaxIdleConns)))
	fmt.Println()

	// Validate input directory
	if err := validateInputDir(importInputDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Check xz availability if we have compressed files
	hasCompressed := hasCompressedFiles(importInputDir)
	if hasCompressed {
		if _, err := exec.LookPath("xz"); err != nil {
			fmt.Fprintln(os.Stderr, "Error: xz not found but compressed files detected")
			fmt.Fprintln(os.Stderr, "Install xz-utils (Linux) or xz (macOS via Homebrew)")
			os.Exit(1)
		}
	}

	// Enable LOCAL INFILE in DSN
	dsn := ensureLocalInfileEnabled(importDBConnection)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Configure pool for parallel loading
	db.SetMaxOpenConns(importMaxOpenConns)
	db.SetMaxIdleConns(importMaxIdleConns)

	// Test connection
	ctx := context.Background()
	spin := u.NewSpinner("Connecting to database")
	spin.Start()
	if err := db.PingContext(ctx); err != nil {
		spin.Error("connection failed: " + err.Error())
		os.Exit(1)
	}
	spin.Success("connected!")

	// Create schema if needed
	spinTables := u.NewSpinner("Creating tables")
	spinTables.Start()
	if err := createTablesIfNotExist(ctx, db); err != nil {
		spinTables.Error("failed: " + err.Error())
		os.Exit(1)
	}
	spinTables.Success("tables ready")

	// Disable checks for bulk loading
	if err := disableChecks(ctx, db); err != nil {
		fmt.Fprintf(os.Stderr, "Error disabling checks: %v\n", err)
		os.Exit(1)
	}

	// Load all tables in parallel
	u.Section("Loading data...")
	startTime := time.Now()
	results, loadErr := loadTablesParallel(ctx, db, importInputDir, u)
	loadDuration := time.Since(startTime)

	// Stop early if any table failed
	if loadErr != nil {
		fmt.Fprintln(os.Stderr, u.Error("Import stopped due to error"))
		printImportSummary(u, results, loadDuration)
		os.Exit(1)
	}

	// Re-enable checks
	if err := enableChecks(ctx, db); err != nil {
		fmt.Fprintf(os.Stderr, "Error re-enabling checks: %v\n", err)
		os.Exit(1)
	}

	// Create indexes
	u.Section("Creating indexes...")
	if err := createIndexes(ctx, db, u); err != nil {
		fmt.Fprintln(os.Stderr, u.Error("Error creating indexes: "+err.Error()))
		os.Exit(1)
	}

	// Print summary
	printImportSummary(u, results, loadDuration)
}

// createTablesIfNotExist creates tables using CREATE TABLE IF NOT EXISTS
func createTablesIfNotExist(ctx context.Context, db *sql.DB) error {
	content, err := schemaFS.ReadFile("schemas/schema_no_indexes.sql")
	if err != nil {
		return fmt.Errorf("failed to read schema: %w", err)
	}

	// Extract and modify CREATE TABLE statements
	lines := strings.Split(string(content), "\n")
	var currentStmt strings.Builder
	inCreateTable := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Skip comments, empty lines, DROP/USE/CREATE DATABASE statements
		if trimmed == "" || strings.HasPrefix(trimmed, "--") {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(trimmed), "DROP ") ||
			strings.HasPrefix(strings.ToUpper(trimmed), "USE ") ||
			strings.HasPrefix(strings.ToUpper(trimmed), "CREATE DATABASE") {
			continue
		}

		// Track CREATE TABLE blocks
		if strings.HasPrefix(strings.ToUpper(trimmed), "CREATE TABLE") {
			inCreateTable = true
			// Add IF NOT EXISTS
			modified := strings.Replace(trimmed, "CREATE TABLE", "CREATE TABLE IF NOT EXISTS", 1)
			currentStmt.WriteString(modified)
			currentStmt.WriteString("\n")
			continue
		}

		if inCreateTable {
			currentStmt.WriteString(line)
			currentStmt.WriteString("\n")

			// Check if statement ends
			if strings.HasSuffix(trimmed, ";") {
				stmt := currentStmt.String()
				if _, err := db.ExecContext(ctx, stmt); err != nil {
					return fmt.Errorf("failed to create table: %w", err)
				}
				currentStmt.Reset()
				inCreateTable = false
			}
		}
	}

	return nil
}

// createIndexes creates indexes and foreign keys after data load
func createIndexes(ctx context.Context, db *sql.DB, u *ui.UI) error {
	content, err := schemaFS.ReadFile("schemas/schema_indexes.sql")
	if err != nil {
		return fmt.Errorf("failed to read index schema: %w", err)
	}

	// Split into statements and execute
	statements := splitSQLStatements(string(content))

	// Count actual statements (excluding comments and USE)
	var validStmts []string
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" || strings.HasPrefix(stmt, "--") {
			continue
		}
		if strings.HasPrefix(strings.ToUpper(stmt), "USE ") {
			continue
		}
		validStmts = append(validStmts, stmt)
	}

	total := len(validStmts)
	progress := u.NewIndexProgress(total)

	for i, stmt := range validStmts {
		progress.Update(i + 1)

		if _, err := db.ExecContext(ctx, stmt); err != nil {
			// Ignore "already exists" errors for indexes and constraints
			errStr := err.Error()
			if strings.Contains(errStr, "Duplicate") ||
				strings.Contains(errStr, "already exists") {
				continue
			}
			return fmt.Errorf("failed to create index: %w", err)
		}
	}
	progress.Complete()

	return nil
}

// loadTablesParallel loads all tables concurrently with fail-fast behavior
func loadTablesParallel(ctx context.Context, db *sql.DB, inputDir string, u *ui.UI) ([]loadResult, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make([]loadResult, len(tablesToLoad))
	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup

	for i, table := range tablesToLoad {
		wg.Add(1)
		go func(idx int, tbl tableConfig) {
			defer wg.Done()

			// Check if cancelled before starting
			select {
			case <-ctx.Done():
				return
			default:
			}

			result := loadTable(ctx, db, inputDir, tbl, u)
			results[idx] = result

			if result.err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = result.err
				}
				mu.Unlock()
				cancel() // Immediately cancel all other goroutines
			}
		}(i, table)
	}

	wg.Wait()
	return results, firstErr
}

// loadTable loads a single table from CSV (supports sharded files)
func loadTable(ctx context.Context, db *sql.DB, inputDir string, tbl tableConfig, u *ui.UI) loadResult {
	start := time.Now()
	result := loadResult{table: tbl.name}

	// Check for sharded files first (transactions_001.csv, etc.)
	shardedFiles := findShardedFiles(inputDir, tbl.csvFile)
	if len(shardedFiles) > 0 {
		u.PrintShardLoading(tbl.name, len(shardedFiles))
		result.rows, result.err = loadShardedFiles(ctx, db, shardedFiles, tbl)
		result.duration = time.Since(start)

		u.PrintTableLoadResult(tbl.name, result.rows, result.duration, len(shardedFiles), result.err)
		return result
	}

	// Fall back to single file (prefer .csv.xz, fall back to .csv)
	csvPath := filepath.Join(inputDir, tbl.csvFile+".csv")
	xzPath := filepath.Join(inputDir, tbl.csvFile+".csv.xz")

	var filePath string
	var isCompressed bool

	if _, err := os.Stat(xzPath); err == nil {
		filePath = xzPath
		isCompressed = true
	} else if _, err := os.Stat(csvPath); err == nil {
		filePath = csvPath
		isCompressed = false
	} else {
		result.err = fmt.Errorf("file not found: %s or %s", csvPath, xzPath)
		u.PrintSkipped(tbl.name, "no file")
		return result
	}

	// Load the data
	if isCompressed {
		result.rows, result.err = loadCompressedFile(ctx, db, filePath, tbl)
	} else {
		result.rows, result.err = loadPlainFile(ctx, db, filePath, tbl)
	}

	result.duration = time.Since(start)

	// Print result
	u.PrintTableLoadResult(tbl.name, result.rows, result.duration, 1, result.err)

	return result
}

// findShardedFiles finds all shard files matching the pattern basename_*.csv or basename_*.csv.xz
func findShardedFiles(inputDir, basename string) []string {
	var files []string

	// Check for compressed shards first
	xzPattern := filepath.Join(inputDir, basename+"_*.csv.xz")
	if matches, err := filepath.Glob(xzPattern); err == nil && len(matches) > 0 {
		files = matches
	}

	// If no compressed shards, check for uncompressed
	if len(files) == 0 {
		csvPattern := filepath.Join(inputDir, basename+"_*.csv")
		if matches, err := filepath.Glob(csvPattern); err == nil {
			files = matches
		}
	}

	// Sort for consistent ordering (_001, _002, etc.)
	if len(files) > 0 {
		sortStrings(files)
	}

	return files
}

// sortStrings sorts a string slice in place
func sortStrings(s []string) {
	for i := 0; i < len(s)-1; i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// loadShardedFiles loads all shard files for a table in order
func loadShardedFiles(ctx context.Context, db *sql.DB, files []string, tbl tableConfig) (int64, error) {
	var totalRows int64

	for i, filePath := range files {
		var rows int64
		var err error

		isCompressed := strings.HasSuffix(filePath, ".xz")
		if isCompressed {
			rows, err = loadCompressedFile(ctx, db, filePath, tbl)
		} else {
			rows, err = loadPlainFile(ctx, db, filePath, tbl)
		}

		if err != nil {
			return totalRows, fmt.Errorf("shard %d (%s): %w", i+1, filepath.Base(filePath), err)
		}

		totalRows += rows
	}

	return totalRows, nil
}

// loadPlainFile loads an uncompressed CSV file
func loadPlainFile(ctx context.Context, db *sql.DB, filePath string, tbl tableConfig) (int64, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to get absolute path: %w", err)
	}

	mysql.RegisterLocalFile(absPath)
	defer mysql.DeregisterLocalFile(absPath)

	loadSQL := fmt.Sprintf(tbl.loadSQL, absPath)
	res, err := db.ExecContext(ctx, loadSQL)
	if err != nil {
		printManualLoadCommand(filePath, tbl, false)
		return 0, fmt.Errorf("LOAD DATA failed: %w", err)
	}

	rows, _ := res.RowsAffected()
	return rows, nil
}

// loadCompressedFile decompresses an xz file to a temp file, then loads it
func loadCompressedFile(ctx context.Context, db *sql.DB, xzPath string, tbl tableConfig) (int64, error) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("loadgen_%s_*.csv", tbl.name))
	if err != nil {
		return 0, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	// Decompress xz to temp file
	xzCmd := exec.CommandContext(ctx, "xz", "-d", "-c", xzPath)
	xzCmd.Stdout = tmpFile
	xzCmd.Stderr = os.Stderr

	if err := xzCmd.Run(); err != nil {
		tmpFile.Close()
		printManualLoadCommand(xzPath, tbl, true)
		return 0, fmt.Errorf("xz decompression failed: %w", err)
	}
	tmpFile.Close()

	// Load from temp file - use inline loading to show correct xz path on error
	absPath, err := filepath.Abs(tmpPath)
	if err != nil {
		return 0, fmt.Errorf("failed to get absolute path: %w", err)
	}

	mysql.RegisterLocalFile(absPath)
	defer mysql.DeregisterLocalFile(absPath)

	loadSQL := fmt.Sprintf(tbl.loadSQL, absPath)
	res, err := db.ExecContext(ctx, loadSQL)
	if err != nil {
		printManualLoadCommand(xzPath, tbl, true)
		return 0, fmt.Errorf("LOAD DATA failed: %w", err)
	}

	rows, _ := res.RowsAffected()
	return rows, nil
}

// Helper functions

func ensureLocalInfileEnabled(dsn string) string {
	if strings.Contains(dsn, "allowAllFiles") {
		return dsn
	}
	if strings.Contains(dsn, "?") {
		return dsn + "&allowAllFiles=true"
	}
	return dsn + "?allowAllFiles=true"
}

func disableChecks(ctx context.Context, db *sql.DB) error {
	queries := []string{
		"SET FOREIGN_KEY_CHECKS = 0",
		"SET UNIQUE_CHECKS = 0",
	}
	for _, q := range queries {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

func enableChecks(ctx context.Context, db *sql.DB) error {
	queries := []string{
		"SET UNIQUE_CHECKS = 1",
		"SET FOREIGN_KEY_CHECKS = 1",
	}
	for _, q := range queries {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return err
		}
	}
	return nil
}

func maskDSN(dsn string) string {
	// Mask password between : and @
	if colonIdx := strings.Index(dsn, ":"); colonIdx > 0 {
		rest := dsn[colonIdx:]
		if atIdx := strings.Index(rest, "@"); atIdx > 0 {
			return dsn[:colonIdx+1] + "***" + rest[atIdx:]
		}
	}
	return dsn
}

// parseDSN extracts connection details from a DSN string
// Format: user:pass@tcp(host:port)/dbname
func parseDSN(dsn string) (user, pass, host, port, dbname string) {
	// Remove any query params
	if idx := strings.Index(dsn, "?"); idx > 0 {
		dsn = dsn[:idx]
	}

	// Extract user:pass
	if atIdx := strings.Index(dsn, "@"); atIdx > 0 {
		userPass := dsn[:atIdx]
		if colonIdx := strings.Index(userPass, ":"); colonIdx > 0 {
			user = userPass[:colonIdx]
			pass = userPass[colonIdx+1:]
		} else {
			user = userPass
		}
		dsn = dsn[atIdx+1:]
	}

	// Extract tcp(host:port)
	if strings.HasPrefix(dsn, "tcp(") {
		endParen := strings.Index(dsn, ")")
		if endParen > 0 {
			hostPort := dsn[4:endParen]
			if colonIdx := strings.Index(hostPort, ":"); colonIdx > 0 {
				host = hostPort[:colonIdx]
				port = hostPort[colonIdx+1:]
			} else {
				host = hostPort
				port = "3306"
			}
			dsn = dsn[endParen+1:]
		}
	}

	// Extract dbname (after /)
	if strings.HasPrefix(dsn, "/") {
		dbname = dsn[1:]
	}

	return
}

// printManualLoadCommand prints a user-friendly command for manual debugging
func printManualLoadCommand(filePath string, tbl tableConfig, isCompressed bool) {
	user, pass, host, port, dbname := parseDSN(importDBConnection)

	absPath, _ := filepath.Abs(filePath)

	fmt.Println("\n    To debug manually, run:")
	fmt.Println("    ─────────────────────────────────────────────")

	if isCompressed {
		// Stream decompressed data directly via /dev/stdin
		loadSQL := fmt.Sprintf(tbl.loadSQL, "/dev/stdin")
		fmt.Printf("    xz -d -c %s | mariadb -u%s -p%s -h %s -P %s --local-infile=1 %s -e \"\n", absPath, user, pass, host, port, dbname)
		fmt.Printf("    SET FOREIGN_KEY_CHECKS = 0;\n")
		fmt.Printf("    %s;\n", loadSQL)
		fmt.Println("    \"")
	} else {
		loadSQL := fmt.Sprintf(tbl.loadSQL, absPath)
		fmt.Printf("    mariadb -u%s -p%s -h %s -P %s --local-infile=1 %s <<'EOF'\n", user, pass, host, port, dbname)
		fmt.Printf("    SET FOREIGN_KEY_CHECKS = 0;\n")
		fmt.Printf("    %s;\n", loadSQL)
		fmt.Println("    EOF")
	}
	fmt.Println("    ─────────────────────────────────────────────")
}

func validateInputDir(dir string) error {
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return fmt.Errorf("input directory does not exist: %s", dir)
	}
	if err != nil {
		return fmt.Errorf("cannot access directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", dir)
	}

	// Check for at least one expected file (including sharded files)
	for _, tbl := range tablesToLoad {
		csvPath := filepath.Join(dir, tbl.csvFile+".csv")
		xzPath := filepath.Join(dir, tbl.csvFile+".csv.xz")
		if _, err := os.Stat(csvPath); err == nil {
			return nil
		}
		if _, err := os.Stat(xzPath); err == nil {
			return nil
		}
		// Check for sharded files
		if shards := findShardedFiles(dir, tbl.csvFile); len(shards) > 0 {
			return nil
		}
	}

	return fmt.Errorf("no CSV files found in %s", dir)
}

func hasCompressedFiles(dir string) bool {
	for _, tbl := range tablesToLoad {
		xzPath := filepath.Join(dir, tbl.csvFile+".csv.xz")
		if _, err := os.Stat(xzPath); err == nil {
			return true
		}
		// Check for sharded compressed files
		xzPattern := filepath.Join(dir, tbl.csvFile+"_*.csv.xz")
		if matches, err := filepath.Glob(xzPattern); err == nil && len(matches) > 0 {
			return true
		}
	}
	return false
}

func splitSQLStatements(content string) []string {
	var statements []string
	var current strings.Builder

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}
		current.WriteString(line)
		current.WriteString("\n")
		if strings.HasSuffix(trimmed, ";") {
			statements = append(statements, current.String())
			current.Reset()
		}
	}

	return statements
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%.1fm", d.Minutes())
}

func formatNumber(n int64) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1000000)
}

func printImportSummary(u *ui.UI, results []loadResult, totalDuration time.Duration) {
	var totalRows int64
	var failures int

	for _, r := range results {
		if r.err != nil {
			failures++
		} else {
			totalRows += r.rows
		}
	}

	items := []ui.KV{
		{Key: "Total rows", Value: formatNumber(totalRows)},
		{Key: "Total time", Value: formatDuration(totalDuration)},
	}

	if failures > 0 {
		items = append(items, ui.KV{Key: "Failed", Value: fmt.Sprintf("%d tables", failures)})
		items = append(items, ui.KV{Key: "Status", Value: "Failed"})
	} else {
		items = append(items, ui.KV{Key: "Status", Value: "Success"})
	}

	fmt.Println(u.SummaryBox("Import Summary", items))

	if failures > 0 {
		os.Exit(1)
	}
}
