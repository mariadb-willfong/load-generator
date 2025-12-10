package generator

import (
	"fmt"
	"time"

	"github.com/willfong/load-generator/internal/data"
	"github.com/willfong/load-generator/internal/models"
	"github.com/willfong/load-generator/internal/utils"
)

// AuditGenerator creates audit log entries for historical activity.
// It generates audit trails for transactions, login sessions, and other banking events.
type AuditGenerator struct {
	rng     *utils.Random
	refData *data.ReferenceData
	config  AuditGeneratorConfig

	// IP address pools for realistic distribution
	ipPools map[string][]string
}

// AuditGeneratorConfig holds settings for audit log generation
type AuditGeneratorConfig struct {
	// Reference data for generating context
	Transactions []GeneratedTransaction
	Customers    []GeneratedCustomer
	Accounts     []GeneratedAccount
	Branches     []GeneratedBranch
	ATMs         []GeneratedATM

	// Error injection rates
	FailedLoginRate      float64 // Rate of failed login attempts (0.0-1.0)
	LockedAccountRate    float64 // Rate of account lockouts after failures
	SessionTimeoutRate   float64 // Rate of sessions that timeout vs logout

	// Session parameters
	AvgSessionsPerCustomerPerMonth int // Average login sessions per customer per month
	AvgBalanceChecksPerSession     int // Average balance inquiries per session
}

// GeneratedAuditLog holds an audit log entry with metadata
type GeneratedAuditLog struct {
	AuditLog models.AuditLog
}

// NewAuditGenerator creates a new audit generator
func NewAuditGenerator(rng *utils.Random, refData *data.ReferenceData, config AuditGeneratorConfig) *AuditGenerator {
	ag := &AuditGenerator{
		rng:     rng,
		refData: refData,
		config:  config,
		ipPools: make(map[string][]string),
	}

	// Generate IP pools for each country/region
	ag.initializeIPPools()

	return ag
}

// initializeIPPools creates realistic IP address pools per region
func (g *AuditGenerator) initializeIPPools() {
	// Common IP prefixes by region (simplified simulation)
	regionPrefixes := map[string][][2]int{
		"NA": {{24, 31}, {65, 72}, {96, 99}, {192, 192}},   // North America
		"EU": {{77, 79}, {88, 89}, {109, 109}, {176, 178}}, // Europe
		"AS": {{14, 14}, {27, 27}, {49, 49}, {103, 103}},   // Asia
		"SA": {{179, 179}, {186, 189}, {200, 201}},         // South America
		"OC": {{101, 101}, {110, 110}, {120, 120}},         // Oceania
		"AF": {{41, 41}, {105, 105}, {154, 154}},           // Africa
	}

	for region, prefixes := range regionPrefixes {
		ips := make([]string, 0, 100)
		for i := 0; i < 100; i++ {
			prefix := prefixes[g.rng.IntN(len(prefixes))]
			first := g.rng.IntRange(prefix[0], prefix[1])
			second := g.rng.IntRange(0, 255)
			third := g.rng.IntRange(0, 255)
			fourth := g.rng.IntRange(1, 254)
			ips = append(ips, fmt.Sprintf("%d.%d.%d.%d", first, second, third, fourth))
		}
		g.ipPools[region] = ips
	}
}

// GenerateAuditLogs creates audit logs for all historical activity.
// Returns audit logs sorted chronologically and the next available ID.
func (g *AuditGenerator) GenerateAuditLogs(startID int64) ([]GeneratedAuditLog, int64) {
	// Estimate capacity: transactions * 2 (initiated + completed) + sessions + balance checks
	estimatedCapacity := len(g.config.Transactions)*2 + len(g.config.Customers)*12*g.config.AvgSessionsPerCustomerPerMonth
	auditLogs := make([]GeneratedAuditLog, 0, estimatedCapacity)

	currentID := startID

	// 1. Generate transaction audit logs (most of the volume)
	txnLogs := g.generateTransactionAuditLogs(&currentID)
	auditLogs = append(auditLogs, txnLogs...)

	// 2. Generate session audit logs (logins, logouts, balance checks)
	sessionLogs := g.generateSessionAuditLogs(&currentID)
	auditLogs = append(auditLogs, sessionLogs...)

	return auditLogs, currentID
}

// generateTransactionAuditLogs creates audit entries for each transaction
func (g *AuditGenerator) generateTransactionAuditLogs(currentID *int64) []GeneratedAuditLog {
	logs := make([]GeneratedAuditLog, 0, len(g.config.Transactions)*2)

	for _, txn := range g.config.Transactions {
		// Transaction initiated event
		initiatedLog := g.createTransactionInitiatedLog(txn, currentID)
		logs = append(logs, initiatedLog)

		// Transaction completed/failed/declined event
		completedLog := g.createTransactionCompletedLog(txn, currentID)
		logs = append(logs, completedLog)
	}

	return logs
}

// createTransactionInitiatedLog creates the "initiated" audit entry for a transaction
func (g *AuditGenerator) createTransactionInitiatedLog(txn GeneratedTransaction, currentID *int64) GeneratedAuditLog {
	t := txn.Transaction
	c := txn.Account.Customer.Customer

	// Determine channel
	channel := channelToAuditChannel(t.Channel)

	// Generate session ID for grouping
	sessionID := fmt.Sprintf("SES%s%08d", t.Timestamp.Format("20060102"), c.ID)

	// Get IP and user agent based on channel
	ipAddress, userAgent := g.getChannelContext(channel, c)

	log := models.AuditLog{
		ID:            *currentID,
		Timestamp:     t.Timestamp.Add(-time.Duration(g.rng.IntRange(1, 30)) * time.Second),
		CustomerID:    &c.ID,
		Action:        models.AuditTransactionInitiated,
		Outcome:       models.OutcomeSuccess, // Initiation always succeeds
		Channel:       channel,
		BranchID:      t.BranchID,
		ATMID:         t.ATMID,
		IPAddress:     ipAddress,
		UserAgent:     userAgent,
		AccountID:     &t.AccountID,
		TransactionID: &t.ID,
		Description:   fmt.Sprintf("Transaction initiated: %s %s", t.Type, t.ReferenceNumber),
		SessionID:     sessionID,
		RequestID:     fmt.Sprintf("REQ%d", *currentID),
	}
	*currentID++

	return GeneratedAuditLog{AuditLog: log}
}

// createTransactionCompletedLog creates the "completed/failed/declined" audit entry
func (g *AuditGenerator) createTransactionCompletedLog(txn GeneratedTransaction, currentID *int64) GeneratedAuditLog {
	t := txn.Transaction
	c := txn.Account.Customer.Customer

	channel := channelToAuditChannel(t.Channel)
	sessionID := fmt.Sprintf("SES%s%08d", t.Timestamp.Format("20060102"), c.ID)
	ipAddress, userAgent := g.getChannelContext(channel, c)

	// Determine action and outcome based on transaction status
	var action models.AuditAction
	var outcome models.AuditOutcome
	var failureReason string

	switch t.Status {
	case models.TxStatusCompleted:
		action = models.AuditTransactionCompleted
		outcome = models.OutcomeSuccess
	case models.TxStatusDeclined:
		action = models.AuditTransactionDeclined
		outcome = models.OutcomeDenied
		if t.FailureReason != nil {
			failureReason = *t.FailureReason
		} else {
			failureReason = "transaction_declined"
		}
	case models.TxStatusFailed:
		action = models.AuditTransactionFailed
		outcome = models.OutcomeFailure
		if t.FailureReason != nil {
			failureReason = *t.FailureReason
		} else {
			failureReason = "transaction_failed"
		}
	default:
		action = models.AuditTransactionCompleted
		outcome = models.OutcomeSuccess
	}

	log := models.AuditLog{
		ID:            *currentID,
		Timestamp:     t.Timestamp,
		CustomerID:    &c.ID,
		Action:        action,
		Outcome:       outcome,
		Channel:       channel,
		BranchID:      t.BranchID,
		ATMID:         t.ATMID,
		IPAddress:     ipAddress,
		UserAgent:     userAgent,
		AccountID:     &t.AccountID,
		TransactionID: &t.ID,
		Description:   fmt.Sprintf("Transaction %s: %s %s", outcome, t.Type, t.ReferenceNumber),
		FailureReason: failureReason,
		SessionID:     sessionID,
		RequestID:     fmt.Sprintf("REQ%d", *currentID),
	}
	*currentID++

	return GeneratedAuditLog{AuditLog: log}
}

// generateSessionAuditLogs creates audit logs for login sessions
func (g *AuditGenerator) generateSessionAuditLogs(currentID *int64) []GeneratedAuditLog {
	logs := make([]GeneratedAuditLog, 0)

	// Find time range from transactions
	if len(g.config.Transactions) == 0 {
		return logs
	}

	startDate := g.config.Transactions[0].Transaction.Timestamp
	endDate := g.config.Transactions[len(g.config.Transactions)-1].Transaction.Timestamp

	// For each customer, generate sessions
	for _, customer := range g.config.Customers {
		customerLogs := g.generateCustomerSessionLogs(customer, startDate, endDate, currentID)
		logs = append(logs, customerLogs...)
	}

	return logs
}

// generateCustomerSessionLogs creates session logs for a single customer
func (g *AuditGenerator) generateCustomerSessionLogs(
	customer GeneratedCustomer,
	startDate, endDate time.Time,
	currentID *int64,
) []GeneratedAuditLog {
	logs := make([]GeneratedAuditLog, 0)

	// Calculate number of sessions for this customer
	months := int(endDate.Sub(startDate).Hours() / (24 * 30))
	if months < 1 {
		months = 1
	}

	avgSessions := g.config.AvgSessionsPerCustomerPerMonth
	if avgSessions <= 0 {
		avgSessions = 3
	}

	// Adjust by activity score
	sessionCount := int(float64(months*avgSessions) * customer.Customer.ActivityScore)
	if sessionCount < 1 {
		sessionCount = 1
	}

	// Generate sessions distributed across the time range
	for i := 0; i < sessionCount; i++ {
		// Random timestamp in the range
		duration := endDate.Sub(startDate)
		offset := time.Duration(g.rng.Float64() * float64(duration))
		sessionTime := startDate.Add(offset)

		// Adjust to business hours
		hour := g.rng.IntRange(7, 22)
		minute := g.rng.IntRange(0, 59)
		sessionTime = time.Date(
			sessionTime.Year(), sessionTime.Month(), sessionTime.Day(),
			hour, minute, g.rng.IntRange(0, 59), 0, time.UTC,
		)

		sessionLogs := g.generateSingleSession(customer, sessionTime, currentID)
		logs = append(logs, sessionLogs...)
	}

	return logs
}

// generateSingleSession creates audit logs for one login session
func (g *AuditGenerator) generateSingleSession(
	customer GeneratedCustomer,
	sessionTime time.Time,
	currentID *int64,
) []GeneratedAuditLog {
	logs := make([]GeneratedAuditLog, 0, 5)

	c := customer.Customer
	customerID := c.ID

	// Choose channel (mostly online, some ATM)
	var channel models.AuditChannel
	var atmID *int64

	if g.rng.Probability(0.7) {
		channel = models.AuditChannelOnline
	} else if g.rng.Probability(0.2) {
		channel = models.AuditChannelMobile
	} else {
		channel = models.AuditChannelATM
		if len(g.config.ATMs) > 0 {
			atm := g.config.ATMs[g.rng.IntN(len(g.config.ATMs))]
			atmID = &atm.ATM.ID
		}
	}

	ipAddress, userAgent := g.getChannelContext(channel, c)
	sessionID := fmt.Sprintf("SES%s%08d%04d", sessionTime.Format("20060102150405"), customerID, g.rng.IntN(10000))

	// Should this be a failed login?
	isFailedLogin := g.rng.Probability(g.config.FailedLoginRate)

	if isFailedLogin {
		// Generate 1-3 failed attempts
		failedAttempts := g.rng.IntRange(1, 3)
		for i := 0; i < failedAttempts; i++ {
			attemptTime := sessionTime.Add(time.Duration(i*10) * time.Second)
			failLog := g.createLoginFailedLog(customerID, attemptTime, channel, atmID, ipAddress, userAgent, sessionID, currentID)
			logs = append(logs, failLog)
		}

		// Maybe lock the account
		if g.rng.Probability(g.config.LockedAccountRate) && failedAttempts >= 3 {
			lockTime := sessionTime.Add(time.Duration(failedAttempts*10+5) * time.Second)
			lockLog := g.createAccountLockedLog(customerID, lockTime, channel, atmID, ipAddress, userAgent, sessionID, currentID)
			logs = append(logs, lockLog)
		}

		return logs
	}

	// Successful login
	loginLog := g.createLoginSuccessLog(customerID, sessionTime, channel, atmID, ipAddress, userAgent, sessionID, currentID)
	logs = append(logs, loginLog)

	// Session started
	sessionStartLog := g.createSessionStartedLog(customerID, sessionTime.Add(time.Second), channel, atmID, ipAddress, userAgent, sessionID, currentID)
	logs = append(logs, sessionStartLog)

	// Balance inquiries during session
	avgChecks := g.config.AvgBalanceChecksPerSession
	if avgChecks <= 0 {
		avgChecks = 2
	}
	numChecks := g.rng.IntRange(1, avgChecks*2)

	// Find customer's accounts
	var customerAccountIDs []int64
	for _, acc := range g.config.Accounts {
		if acc.Account.CustomerID == customerID {
			customerAccountIDs = append(customerAccountIDs, acc.Account.ID)
		}
	}

	for i := 0; i < numChecks && len(customerAccountIDs) > 0; i++ {
		checkTime := sessionTime.Add(time.Duration(30+i*20) * time.Second)
		accountID := customerAccountIDs[g.rng.IntN(len(customerAccountIDs))]
		balanceLog := g.createBalanceInquiryLog(customerID, accountID, checkTime, channel, atmID, ipAddress, userAgent, sessionID, currentID)
		logs = append(logs, balanceLog)
	}

	// Session end (logout or timeout)
	sessionDuration := time.Duration(g.rng.IntRange(60, 1800)) * time.Second
	endTime := sessionTime.Add(sessionDuration)

	if g.rng.Probability(g.config.SessionTimeoutRate) {
		timeoutLog := g.createSessionTimeoutLog(customerID, endTime, channel, atmID, ipAddress, userAgent, sessionID, currentID)
		logs = append(logs, timeoutLog)
	} else {
		logoutLog := g.createLogoutLog(customerID, endTime, channel, atmID, ipAddress, userAgent, sessionID, currentID)
		logs = append(logs, logoutLog)
		sessionEndLog := g.createSessionEndedLog(customerID, endTime.Add(time.Second), channel, atmID, ipAddress, userAgent, sessionID, currentID)
		logs = append(logs, sessionEndLog)
	}

	return logs
}

// Helper functions to create specific audit log types

func (g *AuditGenerator) createLoginSuccessLog(customerID int64, ts time.Time, channel models.AuditChannel, atmID *int64, ip, ua, sessionID string, currentID *int64) GeneratedAuditLog {
	log := models.AuditLog{
		ID:         *currentID,
		Timestamp:  ts,
		CustomerID: &customerID,
		Action:     models.AuditLoginSuccess,
		Outcome:    models.OutcomeSuccess,
		Channel:    channel,
		ATMID:      atmID,
		IPAddress:  ip,
		UserAgent:  ua,
		Description: "User logged in successfully",
		SessionID:  sessionID,
		RequestID:  fmt.Sprintf("REQ%d", *currentID),
	}
	*currentID++
	return GeneratedAuditLog{AuditLog: log}
}

func (g *AuditGenerator) createLoginFailedLog(customerID int64, ts time.Time, channel models.AuditChannel, atmID *int64, ip, ua, sessionID string, currentID *int64) GeneratedAuditLog {
	reasons := []string{"invalid_password", "invalid_pin", "expired_credentials", "user_not_found"}
	reason := reasons[g.rng.IntN(len(reasons))]

	log := models.AuditLog{
		ID:            *currentID,
		Timestamp:     ts,
		CustomerID:    &customerID,
		Action:        models.AuditLoginFailed,
		Outcome:       models.OutcomeFailure,
		Channel:       channel,
		ATMID:         atmID,
		IPAddress:     ip,
		UserAgent:     ua,
		Description:   "Login attempt failed",
		FailureReason: reason,
		SessionID:     sessionID,
		RequestID:     fmt.Sprintf("REQ%d", *currentID),
	}
	*currentID++
	return GeneratedAuditLog{AuditLog: log}
}

func (g *AuditGenerator) createAccountLockedLog(customerID int64, ts time.Time, channel models.AuditChannel, atmID *int64, ip, ua, sessionID string, currentID *int64) GeneratedAuditLog {
	log := models.AuditLog{
		ID:            *currentID,
		Timestamp:     ts,
		CustomerID:    &customerID,
		Action:        models.AuditAccountLocked,
		Outcome:       models.OutcomeDenied,
		Channel:       channel,
		ATMID:         atmID,
		IPAddress:     ip,
		UserAgent:     ua,
		Description:   "Account locked due to multiple failed login attempts",
		FailureReason: "max_attempts_exceeded",
		SessionID:     sessionID,
		RequestID:     fmt.Sprintf("REQ%d", *currentID),
	}
	*currentID++
	return GeneratedAuditLog{AuditLog: log}
}

func (g *AuditGenerator) createLogoutLog(customerID int64, ts time.Time, channel models.AuditChannel, atmID *int64, ip, ua, sessionID string, currentID *int64) GeneratedAuditLog {
	log := models.AuditLog{
		ID:         *currentID,
		Timestamp:  ts,
		CustomerID: &customerID,
		Action:     models.AuditLogout,
		Outcome:    models.OutcomeSuccess,
		Channel:    channel,
		ATMID:      atmID,
		IPAddress:  ip,
		UserAgent:  ua,
		Description: "User logged out",
		SessionID:  sessionID,
		RequestID:  fmt.Sprintf("REQ%d", *currentID),
	}
	*currentID++
	return GeneratedAuditLog{AuditLog: log}
}

func (g *AuditGenerator) createSessionStartedLog(customerID int64, ts time.Time, channel models.AuditChannel, atmID *int64, ip, ua, sessionID string, currentID *int64) GeneratedAuditLog {
	log := models.AuditLog{
		ID:         *currentID,
		Timestamp:  ts,
		CustomerID: &customerID,
		Action:     models.AuditSessionStarted,
		Outcome:    models.OutcomeSuccess,
		Channel:    channel,
		ATMID:      atmID,
		IPAddress:  ip,
		UserAgent:  ua,
		Description: "Session started",
		SessionID:  sessionID,
		RequestID:  fmt.Sprintf("REQ%d", *currentID),
	}
	*currentID++
	return GeneratedAuditLog{AuditLog: log}
}

func (g *AuditGenerator) createSessionEndedLog(customerID int64, ts time.Time, channel models.AuditChannel, atmID *int64, ip, ua, sessionID string, currentID *int64) GeneratedAuditLog {
	log := models.AuditLog{
		ID:         *currentID,
		Timestamp:  ts,
		CustomerID: &customerID,
		Action:     models.AuditSessionEnded,
		Outcome:    models.OutcomeSuccess,
		Channel:    channel,
		ATMID:      atmID,
		IPAddress:  ip,
		UserAgent:  ua,
		Description: "Session ended normally",
		SessionID:  sessionID,
		RequestID:  fmt.Sprintf("REQ%d", *currentID),
	}
	*currentID++
	return GeneratedAuditLog{AuditLog: log}
}

func (g *AuditGenerator) createSessionTimeoutLog(customerID int64, ts time.Time, channel models.AuditChannel, atmID *int64, ip, ua, sessionID string, currentID *int64) GeneratedAuditLog {
	log := models.AuditLog{
		ID:            *currentID,
		Timestamp:     ts,
		CustomerID:    &customerID,
		Action:        models.AuditSessionTimeout,
		Outcome:       models.OutcomeSuccess,
		Channel:       channel,
		ATMID:         atmID,
		IPAddress:     ip,
		UserAgent:     ua,
		Description:   "Session timed out due to inactivity",
		FailureReason: "inactivity_timeout",
		SessionID:     sessionID,
		RequestID:     fmt.Sprintf("REQ%d", *currentID),
	}
	*currentID++
	return GeneratedAuditLog{AuditLog: log}
}

func (g *AuditGenerator) createBalanceInquiryLog(customerID, accountID int64, ts time.Time, channel models.AuditChannel, atmID *int64, ip, ua, sessionID string, currentID *int64) GeneratedAuditLog {
	log := models.AuditLog{
		ID:         *currentID,
		Timestamp:  ts,
		CustomerID: &customerID,
		Action:     models.AuditBalanceInquiry,
		Outcome:    models.OutcomeSuccess,
		Channel:    channel,
		ATMID:      atmID,
		IPAddress:  ip,
		UserAgent:  ua,
		AccountID:  &accountID,
		Description: "Balance inquiry",
		SessionID:  sessionID,
		RequestID:  fmt.Sprintf("REQ%d", *currentID),
	}
	*currentID++
	return GeneratedAuditLog{AuditLog: log}
}

// getChannelContext returns IP address and user agent based on channel
func (g *AuditGenerator) getChannelContext(channel models.AuditChannel, customer models.Customer) (string, string) {
	switch channel {
	case models.AuditChannelATM:
		// ATM doesn't have user agent, IP is internal
		return "10.0.0." + fmt.Sprintf("%d", g.rng.IntRange(1, 254)), ""

	case models.AuditChannelBranch:
		// Branch teller systems
		return "192.168." + fmt.Sprintf("%d.%d", g.rng.IntRange(1, 10), g.rng.IntRange(1, 254)), "BranchTellerSystem/3.2"

	case models.AuditChannelMobile:
		// Mobile app
		region := g.getCustomerRegion(customer)
		ips := g.ipPools[region]
		ip := ips[g.rng.IntN(len(ips))]

		agents := []string{
			"BankApp/5.2.1 (iOS 17.0; iPhone14,2)",
			"BankApp/5.2.0 (Android 14; Pixel 8)",
			"BankApp/5.1.9 (iOS 16.5; iPhone13,4)",
			"BankApp/5.1.8 (Android 13; Samsung S23)",
		}
		return ip, agents[g.rng.IntN(len(agents))]

	case models.AuditChannelOnline:
		// Web browser
		region := g.getCustomerRegion(customer)
		ips := g.ipPools[region]
		ip := ips[g.rng.IntN(len(ips))]

		agents := []string{
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Safari/17.2",
		}
		return ip, agents[g.rng.IntN(len(agents))]

	default:
		return "0.0.0.0", ""
	}
}

// getCustomerRegion maps customer country to region for IP pools
func (g *AuditGenerator) getCustomerRegion(customer models.Customer) string {
	// Map country codes to regions (simplified)
	switch customer.Country {
	case "US", "CA", "MX":
		return "NA"
	case "BR", "AR", "CL", "CO", "PE":
		return "SA"
	case "GB", "DE", "FR", "IT", "ES", "NL", "BE", "CH", "AT", "SE", "NO", "DK", "FI", "PL", "PT", "IE", "CZ", "GR", "HU", "RO":
		return "EU"
	case "AU", "NZ":
		return "OC"
	case "ZA", "NG", "EG", "KE", "MA":
		return "AF"
	default:
		return "AS" // Default to Asia
	}
}

// channelToAuditChannel converts transaction channel to audit channel
func channelToAuditChannel(txnChannel models.TransactionChannel) models.AuditChannel {
	switch txnChannel {
	case models.ChannelATM:
		return models.AuditChannelATM
	case models.ChannelBranch:
		return models.AuditChannelBranch
	case models.ChannelOnline:
		return models.AuditChannelOnline
	case models.ChannelPOS:
		return models.AuditChannelAPI // POS is an API integration
	case models.ChannelACH, models.ChannelWire:
		return models.AuditChannelSystem
	case models.ChannelInternal:
		return models.AuditChannelSystem
	default:
		return models.AuditChannelOnline
	}
}

// WriteAuditLogsCSV writes audit logs to a CSV file (or .csv.xz if compress=true)
func WriteAuditLogsCSV(auditLogs []GeneratedAuditLog, outputDir string, compress bool) error {
	return writeAuditLogsCSVInternal(auditLogs, outputDir, compress, false)
}

// WriteAuditLogsCSVWithProgress writes audit logs to a CSV file with progress reporting
func WriteAuditLogsCSVWithProgress(auditLogs []GeneratedAuditLog, outputDir string, compress bool) error {
	return writeAuditLogsCSVInternal(auditLogs, outputDir, compress, true)
}

// writeAuditLogsCSVInternal is the internal implementation with optional progress
func writeAuditLogsCSVInternal(auditLogs []GeneratedAuditLog, outputDir string, compress, showProgress bool) error {
	headers := []string{
		"id", "timestamp", "customer_id", "employee_id", "system_id",
		"action", "outcome", "channel", "branch_id", "atm_id",
		"ip_address", "user_agent", "account_id", "transaction_id", "beneficiary_id",
		"description", "failure_reason", "metadata", "session_id", "risk_score", "request_id",
	}

	writer, err := NewCSVWriter(CSVWriterConfig{
		OutputDir: outputDir,
		Filename:  "audit_logs",
		Headers:   headers,
		Compress:  compress,
	})
	if err != nil {
		return err
	}
	defer writer.Close()

	var progress *ProgressReporter
	if showProgress {
		progress = NewProgressReporter(ProgressConfig{
			Total: int64(len(auditLogs)),
			Label: "  Audit logs",
		})
	}

	for i, ga := range auditLogs {
		a := ga.AuditLog
		row := []string{
			FormatInt64(a.ID),
			FormatTime(a.Timestamp),
			FormatInt64Ptr(a.CustomerID),
			FormatInt64Ptr(a.EmployeeID),
			a.SystemID,
			string(a.Action),
			string(a.Outcome),
			string(a.Channel),
			FormatInt64Ptr(a.BranchID),
			FormatInt64Ptr(a.ATMID),
			a.IPAddress,
			a.UserAgent,
			FormatInt64Ptr(a.AccountID),
			FormatInt64Ptr(a.TransactionID),
			FormatInt64Ptr(a.BeneficiaryID),
			a.Description,
			a.FailureReason,
			a.Metadata,
			a.SessionID,
			formatFloat64Ptr(a.RiskScore),
			a.RequestID,
		}
		if err := writer.WriteRow(row); err != nil {
			return err
		}

		if progress != nil && i%1000 == 0 {
			progress.Set(int64(i + 1))
		}
	}

	if progress != nil {
		progress.Set(int64(len(auditLogs)))
		progress.Finish()
	}

	return writer.Close()
}

// formatFloat64Ptr formats a *float64 for CSV
func formatFloat64Ptr(f *float64) string {
	if f == nil {
		return ""
	}
	return FormatFloat64(*f)
}
