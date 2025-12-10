package generator

import (
	"fmt"
	"time"

	"github.com/willfong/load-generator/internal/data"
	"github.com/willfong/load-generator/internal/models"
	"github.com/willfong/load-generator/internal/utils"
)

// StreamingAuditGenerator generates audit logs and writes them directly
// to a CSV file, minimizing memory usage for large datasets.
type StreamingAuditGenerator struct {
	rng     *utils.Random
	refData *data.ReferenceData
	config  StreamingAuditConfig

	// IP address pools for realistic distribution
	ipPools map[string][]string

	// Streaming output
	writer   *CSVWriter
	workerID int

	// Progress reporting
	progressChan chan<- workerProgress
	count        int64

	// ID tracking
	currentID int64
	endID     int64
}

// StreamingAuditConfig holds settings for streaming audit log generation
type StreamingAuditConfig struct {
	// Reference data
	Customers []GeneratedCustomer
	Accounts  []GeneratedAccount
	ATMs      []GeneratedATM

	// Error injection rates
	FailedLoginRate    float64
	LockedAccountRate  float64
	SessionTimeoutRate float64

	// Session parameters
	AvgSessionsPerCustomerPerMonth int
	AvgBalanceChecksPerSession     int

	// Time range for session logs
	StartDate time.Time
	EndDate   time.Time

	// Worker configuration
	WorkerID    int
	WorkerCount int
	StartID     int64
	EndID       int64

	// Output configuration
	OutputDir string
	Compress  bool

	// Progress channel
	ProgressChan chan<- workerProgress
}

// AuditLogHeaders returns the CSV headers for audit logs
func AuditLogHeaders() []string {
	return []string{
		"id", "timestamp", "customer_id", "employee_id", "system_id",
		"action", "outcome", "channel", "branch_id", "atm_id",
		"ip_address", "user_agent", "account_id", "transaction_id", "beneficiary_id",
		"description", "failure_reason", "metadata", "session_id", "risk_score", "request_id",
	}
}

// NewStreamingAuditGenerator creates a new streaming audit generator
func NewStreamingAuditGenerator(rng *utils.Random, refData *data.ReferenceData, config StreamingAuditConfig) (*StreamingAuditGenerator, error) {
	// Create shard writer
	writer, err := NewShardedCSVWriter(CSVWriterConfig{
		OutputDir: config.OutputDir,
		Filename:  "audit_logs",
		Headers:   AuditLogHeaders(),
		Compress:  config.Compress,
	}, config.WorkerID+1, config.WorkerCount) // 1-indexed shard numbers

	if err != nil {
		return nil, fmt.Errorf("failed to create shard writer: %w", err)
	}

	sag := &StreamingAuditGenerator{
		rng:          rng,
		refData:      refData,
		config:       config,
		ipPools:      make(map[string][]string),
		writer:       writer,
		workerID:     config.WorkerID,
		progressChan: config.ProgressChan,
		currentID:    config.StartID,
		endID:        config.EndID,
	}

	sag.initializeIPPools()

	return sag, nil
}

// initializeIPPools creates realistic IP address pools per region
func (g *StreamingAuditGenerator) initializeIPPools() {
	regionPrefixes := map[string][][2]int{
		"NA": {{24, 31}, {65, 72}, {96, 99}, {192, 192}},
		"EU": {{77, 79}, {88, 89}, {109, 109}, {176, 178}},
		"AS": {{14, 14}, {27, 27}, {49, 49}, {103, 103}},
		"SA": {{179, 179}, {186, 189}, {200, 201}},
		"OC": {{101, 101}, {110, 110}, {120, 120}},
		"AF": {{41, 41}, {105, 105}, {154, 154}},
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

// GenerateAndStream generates audit logs for the assigned customers and streams them to CSV.
// This generates session-based audit logs (logins, logouts, balance checks).
// Transaction-based audit logs should be generated inline during transaction streaming.
func (g *StreamingAuditGenerator) GenerateAndStream() (int64, error) {
	defer g.writer.Close()

	// Generate session audit logs for each customer
	for _, customer := range g.config.Customers {
		if err := g.generateCustomerSessionLogs(customer); err != nil {
			return g.count, err
		}
	}

	return g.count, nil
}

// WriteTransactionAuditLogs writes audit logs for a transaction.
// Call this from the transaction streaming generator for each transaction.
func (g *StreamingAuditGenerator) WriteTransactionAuditLogs(txn models.Transaction, customer models.Customer) error {
	// Transaction initiated event
	if err := g.writeTransactionInitiatedLog(txn, customer); err != nil {
		return err
	}

	// Transaction completed/failed/declined event
	return g.writeTransactionCompletedLog(txn, customer)
}

func (g *StreamingAuditGenerator) writeTransactionInitiatedLog(t models.Transaction, c models.Customer) error {
	channel := channelToAuditChannel(t.Channel)
	sessionID := fmt.Sprintf("SES%s%08d", t.Timestamp.Format("20060102"), c.ID)
	ipAddress, userAgent := g.getChannelContext(channel, c)

	log := models.AuditLog{
		ID:            g.currentID,
		Timestamp:     t.Timestamp.Add(-time.Duration(g.rng.IntRange(1, 30)) * time.Second),
		CustomerID:    &c.ID,
		Action:        models.AuditTransactionInitiated,
		Outcome:       models.OutcomeSuccess,
		Channel:       channel,
		BranchID:      t.BranchID,
		ATMID:         t.ATMID,
		IPAddress:     ipAddress,
		UserAgent:     userAgent,
		AccountID:     &t.AccountID,
		TransactionID: &t.ID,
		Description:   fmt.Sprintf("Transaction initiated: %s %s", t.Type, t.ReferenceNumber),
		SessionID:     sessionID,
		RequestID:     fmt.Sprintf("REQ%d", g.currentID),
	}
	g.currentID++

	return g.writeAuditLog(log)
}

func (g *StreamingAuditGenerator) writeTransactionCompletedLog(t models.Transaction, c models.Customer) error {
	channel := channelToAuditChannel(t.Channel)
	sessionID := fmt.Sprintf("SES%s%08d", t.Timestamp.Format("20060102"), c.ID)
	ipAddress, userAgent := g.getChannelContext(channel, c)

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
		ID:            g.currentID,
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
		RequestID:     fmt.Sprintf("REQ%d", g.currentID),
	}
	g.currentID++

	return g.writeAuditLog(log)
}

func (g *StreamingAuditGenerator) generateCustomerSessionLogs(customer GeneratedCustomer) error {
	months := int(g.config.EndDate.Sub(g.config.StartDate).Hours() / (24 * 30))
	if months < 1 {
		months = 1
	}

	avgSessions := g.config.AvgSessionsPerCustomerPerMonth
	if avgSessions <= 0 {
		avgSessions = 3
	}

	sessionCount := int(float64(months*avgSessions) * customer.Customer.ActivityScore)
	if sessionCount < 1 {
		sessionCount = 1
	}

	for i := 0; i < sessionCount; i++ {
		duration := g.config.EndDate.Sub(g.config.StartDate)
		offset := time.Duration(g.rng.Float64() * float64(duration))
		sessionTime := g.config.StartDate.Add(offset)

		hour := g.rng.IntRange(7, 22)
		minute := g.rng.IntRange(0, 59)
		sessionTime = time.Date(
			sessionTime.Year(), sessionTime.Month(), sessionTime.Day(),
			hour, minute, g.rng.IntRange(0, 59), 0, time.UTC,
		)

		if err := g.generateSingleSession(customer, sessionTime); err != nil {
			return err
		}
	}

	return nil
}

func (g *StreamingAuditGenerator) generateSingleSession(customer GeneratedCustomer, sessionTime time.Time) error {
	c := customer.Customer
	customerID := c.ID

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

	isFailedLogin := g.rng.Probability(g.config.FailedLoginRate)

	if isFailedLogin {
		failedAttempts := g.rng.IntRange(1, 3)
		for i := 0; i < failedAttempts; i++ {
			attemptTime := sessionTime.Add(time.Duration(i*10) * time.Second)
			if err := g.writeLoginFailedLog(customerID, attemptTime, channel, atmID, ipAddress, userAgent, sessionID); err != nil {
				return err
			}
		}

		if g.rng.Probability(g.config.LockedAccountRate) && failedAttempts >= 3 {
			lockTime := sessionTime.Add(time.Duration(failedAttempts*10+5) * time.Second)
			if err := g.writeAccountLockedLog(customerID, lockTime, channel, atmID, ipAddress, userAgent, sessionID); err != nil {
				return err
			}
		}
		return nil
	}

	// Successful login
	if err := g.writeLoginSuccessLog(customerID, sessionTime, channel, atmID, ipAddress, userAgent, sessionID); err != nil {
		return err
	}

	if err := g.writeSessionStartedLog(customerID, sessionTime.Add(time.Second), channel, atmID, ipAddress, userAgent, sessionID); err != nil {
		return err
	}

	// Balance inquiries
	avgChecks := g.config.AvgBalanceChecksPerSession
	if avgChecks <= 0 {
		avgChecks = 2
	}
	numChecks := g.rng.IntRange(1, avgChecks*2)

	var customerAccountIDs []int64
	for _, acc := range g.config.Accounts {
		if acc.Account.CustomerID == customerID {
			customerAccountIDs = append(customerAccountIDs, acc.Account.ID)
		}
	}

	for i := 0; i < numChecks && len(customerAccountIDs) > 0; i++ {
		checkTime := sessionTime.Add(time.Duration(30+i*20) * time.Second)
		accountID := customerAccountIDs[g.rng.IntN(len(customerAccountIDs))]
		if err := g.writeBalanceInquiryLog(customerID, accountID, checkTime, channel, atmID, ipAddress, userAgent, sessionID); err != nil {
			return err
		}
	}

	// Session end
	sessionDuration := time.Duration(g.rng.IntRange(60, 1800)) * time.Second
	endTime := sessionTime.Add(sessionDuration)

	if g.rng.Probability(g.config.SessionTimeoutRate) {
		return g.writeSessionTimeoutLog(customerID, endTime, channel, atmID, ipAddress, userAgent, sessionID)
	}

	if err := g.writeLogoutLog(customerID, endTime, channel, atmID, ipAddress, userAgent, sessionID); err != nil {
		return err
	}
	return g.writeSessionEndedLog(customerID, endTime.Add(time.Second), channel, atmID, ipAddress, userAgent, sessionID)
}

func (g *StreamingAuditGenerator) writeAuditLog(a models.AuditLog) error {
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

	if err := g.writer.WriteRow(row); err != nil {
		return err
	}

	g.count++

	if g.progressChan != nil && g.count%1000 == 0 {
		select {
		case g.progressChan <- workerProgress{workerID: g.workerID, count: g.count}:
		default:
		}
	}

	return nil
}

func (g *StreamingAuditGenerator) writeLoginSuccessLog(customerID int64, ts time.Time, channel models.AuditChannel, atmID *int64, ip, ua, sessionID string) error {
	log := models.AuditLog{
		ID:          g.currentID,
		Timestamp:   ts,
		CustomerID:  &customerID,
		Action:      models.AuditLoginSuccess,
		Outcome:     models.OutcomeSuccess,
		Channel:     channel,
		ATMID:       atmID,
		IPAddress:   ip,
		UserAgent:   ua,
		Description: "User logged in successfully",
		SessionID:   sessionID,
		RequestID:   fmt.Sprintf("REQ%d", g.currentID),
	}
	g.currentID++
	return g.writeAuditLog(log)
}

func (g *StreamingAuditGenerator) writeLoginFailedLog(customerID int64, ts time.Time, channel models.AuditChannel, atmID *int64, ip, ua, sessionID string) error {
	reasons := []string{"invalid_password", "invalid_pin", "expired_credentials", "user_not_found"}
	reason := reasons[g.rng.IntN(len(reasons))]

	log := models.AuditLog{
		ID:            g.currentID,
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
		RequestID:     fmt.Sprintf("REQ%d", g.currentID),
	}
	g.currentID++
	return g.writeAuditLog(log)
}

func (g *StreamingAuditGenerator) writeAccountLockedLog(customerID int64, ts time.Time, channel models.AuditChannel, atmID *int64, ip, ua, sessionID string) error {
	log := models.AuditLog{
		ID:            g.currentID,
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
		RequestID:     fmt.Sprintf("REQ%d", g.currentID),
	}
	g.currentID++
	return g.writeAuditLog(log)
}

func (g *StreamingAuditGenerator) writeLogoutLog(customerID int64, ts time.Time, channel models.AuditChannel, atmID *int64, ip, ua, sessionID string) error {
	log := models.AuditLog{
		ID:          g.currentID,
		Timestamp:   ts,
		CustomerID:  &customerID,
		Action:      models.AuditLogout,
		Outcome:     models.OutcomeSuccess,
		Channel:     channel,
		ATMID:       atmID,
		IPAddress:   ip,
		UserAgent:   ua,
		Description: "User logged out",
		SessionID:   sessionID,
		RequestID:   fmt.Sprintf("REQ%d", g.currentID),
	}
	g.currentID++
	return g.writeAuditLog(log)
}

func (g *StreamingAuditGenerator) writeSessionStartedLog(customerID int64, ts time.Time, channel models.AuditChannel, atmID *int64, ip, ua, sessionID string) error {
	log := models.AuditLog{
		ID:          g.currentID,
		Timestamp:   ts,
		CustomerID:  &customerID,
		Action:      models.AuditSessionStarted,
		Outcome:     models.OutcomeSuccess,
		Channel:     channel,
		ATMID:       atmID,
		IPAddress:   ip,
		UserAgent:   ua,
		Description: "Session started",
		SessionID:   sessionID,
		RequestID:   fmt.Sprintf("REQ%d", g.currentID),
	}
	g.currentID++
	return g.writeAuditLog(log)
}

func (g *StreamingAuditGenerator) writeSessionEndedLog(customerID int64, ts time.Time, channel models.AuditChannel, atmID *int64, ip, ua, sessionID string) error {
	log := models.AuditLog{
		ID:          g.currentID,
		Timestamp:   ts,
		CustomerID:  &customerID,
		Action:      models.AuditSessionEnded,
		Outcome:     models.OutcomeSuccess,
		Channel:     channel,
		ATMID:       atmID,
		IPAddress:   ip,
		UserAgent:   ua,
		Description: "Session ended normally",
		SessionID:   sessionID,
		RequestID:   fmt.Sprintf("REQ%d", g.currentID),
	}
	g.currentID++
	return g.writeAuditLog(log)
}

func (g *StreamingAuditGenerator) writeSessionTimeoutLog(customerID int64, ts time.Time, channel models.AuditChannel, atmID *int64, ip, ua, sessionID string) error {
	log := models.AuditLog{
		ID:            g.currentID,
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
		RequestID:     fmt.Sprintf("REQ%d", g.currentID),
	}
	g.currentID++
	return g.writeAuditLog(log)
}

func (g *StreamingAuditGenerator) writeBalanceInquiryLog(customerID, accountID int64, ts time.Time, channel models.AuditChannel, atmID *int64, ip, ua, sessionID string) error {
	log := models.AuditLog{
		ID:          g.currentID,
		Timestamp:   ts,
		CustomerID:  &customerID,
		Action:      models.AuditBalanceInquiry,
		Outcome:     models.OutcomeSuccess,
		Channel:     channel,
		ATMID:       atmID,
		IPAddress:   ip,
		UserAgent:   ua,
		AccountID:   &accountID,
		Description: "Balance inquiry",
		SessionID:   sessionID,
		RequestID:   fmt.Sprintf("REQ%d", g.currentID),
	}
	g.currentID++
	return g.writeAuditLog(log)
}

func (g *StreamingAuditGenerator) getChannelContext(channel models.AuditChannel, customer models.Customer) (string, string) {
	switch channel {
	case models.AuditChannelATM:
		return "10.0.0." + fmt.Sprintf("%d", g.rng.IntRange(1, 254)), ""
	case models.AuditChannelBranch:
		return "192.168." + fmt.Sprintf("%d.%d", g.rng.IntRange(1, 10), g.rng.IntRange(1, 254)), "BranchTellerSystem/3.2"
	case models.AuditChannelMobile:
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

func (g *StreamingAuditGenerator) getCustomerRegion(customer models.Customer) string {
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
		return "AS"
	}
}

// ShardFile returns the path to the shard file created by this generator
func (g *StreamingAuditGenerator) ShardFile() string {
	return g.writer.Path()
}

// Count returns the number of audit logs written
func (g *StreamingAuditGenerator) Count() int64 {
	return g.count
}
