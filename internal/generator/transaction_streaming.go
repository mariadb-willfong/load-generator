package generator

import (
	"fmt"
	"time"

	"github.com/willfong/load-generator/internal/data"
	"github.com/willfong/load-generator/internal/generator/patterns"
	"github.com/willfong/load-generator/internal/models"
	"github.com/willfong/load-generator/internal/utils"
)

// StreamingTransactionGenerator generates transactions and writes them directly
// to a CSV file, minimizing memory usage for large datasets.
type StreamingTransactionGenerator struct {
	rng     *utils.Random
	refData *data.ReferenceData
	config  StreamingTransactionConfig

	// Patterns for realistic distribution
	retailPattern   *patterns.FullPattern
	atmPattern      *patterns.FullPattern
	onlinePattern   *patterns.FullPattern
	businessPattern *patterns.FullPattern

	// Activity distribution
	activityDist *patterns.ActivityDistribution

	// Amount distributions
	amounts *patterns.TransactionTypeAmounts

	// Reference data
	branches []GeneratedBranch
	atms     []GeneratedATM

	// Account lookups for counterparty transactions
	accountsByID map[int64]GeneratedAccount

	// Merchant account IDs for purchase destinations
	merchantAccountIDs []int64
	// Employer account IDs for salary sources
	employerAccountIDs []int64
	// Utility account IDs for bill payments
	utilityAccountIDs []int64

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

// StreamingTransactionConfig holds settings for streaming transaction generation
type StreamingTransactionConfig struct {
	// Time range for historical transactions
	StartDate time.Time
	EndDate   time.Time

	// Average transactions per customer per month
	TransactionsPerCustomerPerMonth int

	// Pareto ratio (e.g., 0.2 = 20% accounts generate 80% volume)
	ParetoRatio float64

	// Day of month for payroll processing (1-31)
	PayrollDay int

	// Error injection rates (0.0-1.0)
	DeclinedTransactionRate float64
	InsufficientFundsRate   float64

	// Reference data
	Branches   []GeneratedBranch
	ATMs       []GeneratedATM
	AllAccounts []GeneratedAccount // All accounts for counterparty lookups
	Businesses []GeneratedBusiness

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

// TransactionHeaders returns the CSV headers for transactions
func TransactionHeaders() []string {
	return []string{
		"id", "reference_number", "account_id", "counterparty_account_id", "beneficiary_id",
		"type", "status", "channel", "amount", "currency", "balance_after",
		"description", "metadata", "branch_id", "atm_id", "linked_transaction_id",
		"timestamp", "posted_at", "value_date", "failure_reason",
	}
}

// NewStreamingTransactionGenerator creates a new streaming transaction generator
func NewStreamingTransactionGenerator(rng *utils.Random, refData *data.ReferenceData, config StreamingTransactionConfig) (*StreamingTransactionGenerator, error) {
	// Create shard writer
	writer, err := NewShardedCSVWriter(CSVWriterConfig{
		OutputDir: config.OutputDir,
		Filename:  "transactions",
		Headers:   TransactionHeaders(),
		Compress:  config.Compress,
	}, config.WorkerID+1, config.WorkerCount) // 1-indexed shard numbers

	if err != nil {
		return nil, fmt.Errorf("failed to create shard writer: %w", err)
	}

	// Build account lookup map
	accountsByID := make(map[int64]GeneratedAccount)
	for _, acc := range config.AllAccounts {
		accountsByID[acc.Account.ID] = acc
	}

	stg := &StreamingTransactionGenerator{
		rng:     rng,
		refData: refData,
		config:  config,

		retailPattern:   patterns.NewDefaultFullPattern(),
		atmPattern:      patterns.NewATMFullPattern(),
		onlinePattern:   patterns.NewOnlineFullPattern(),
		businessPattern: patterns.NewBusinessFullPattern(),

		activityDist: patterns.NewParetoDistribution(config.ParetoRatio),
		amounts:      patterns.NewTransactionTypeAmounts(),

		branches:     config.Branches,
		atms:         config.ATMs,
		accountsByID: accountsByID,

		writer:       writer,
		workerID:     config.WorkerID,
		progressChan: config.ProgressChan,
		currentID:    config.StartID,
		endID:        config.EndID,
	}

	// Categorize business accounts by type
	for _, acc := range config.AllAccounts {
		switch acc.Account.Type {
		case models.AccountTypeMerchant:
			stg.merchantAccountIDs = append(stg.merchantAccountIDs, acc.Account.ID)
		case models.AccountTypePayroll:
			stg.employerAccountIDs = append(stg.employerAccountIDs, acc.Account.ID)
		}
	}

	// Add utility company accounts from businesses
	for _, biz := range config.Businesses {
		if biz.BusinessType == BusinessTypeUtility {
			for _, acc := range config.AllAccounts {
				if acc.Account.CustomerID == biz.Customer.ID {
					stg.utilityAccountIDs = append(stg.utilityAccountIDs, acc.Account.ID)
					break
				}
			}
		}
	}

	return stg, nil
}

// GenerateAndStream generates transactions for the assigned accounts and streams them to CSV.
// Returns the number of transactions generated.
func (g *StreamingTransactionGenerator) GenerateAndStream(accounts []GeneratedAccount) (int64, error) {
	defer g.writer.Close()

	// Group accounts by customer for coordinated generation
	customerAccounts := make(map[int64][]GeneratedAccount)
	for _, acc := range accounts {
		customerAccounts[acc.Account.CustomerID] = append(customerAccounts[acc.Account.CustomerID], acc)
	}

	// Track running balances for accounts in this worker
	balances := make(map[int64]int64)
	for _, acc := range accounts {
		balances[acc.Account.ID] = acc.Account.Balance
	}

	// Generate month by month
	currentMonth := g.config.StartDate
	for currentMonth.Before(g.config.EndDate) {
		monthEnd := currentMonth.AddDate(0, 1, 0)
		if monthEnd.After(g.config.EndDate) {
			monthEnd = g.config.EndDate
		}

		if err := g.generateMonthTransactions(accounts, customerAccounts, balances, currentMonth, monthEnd); err != nil {
			return g.count, err
		}

		currentMonth = currentMonth.AddDate(0, 1, 0)
	}

	return g.count, nil
}

// generateMonthTransactions generates and streams transactions for a single month
func (g *StreamingTransactionGenerator) generateMonthTransactions(
	accounts []GeneratedAccount,
	customerAccounts map[int64][]GeneratedAccount,
	balances map[int64]int64,
	monthStart, monthEnd time.Time,
) error {
	for _, account := range accounts {
		// Skip closed accounts or accounts opened after this month
		if account.Account.OpenedAt.After(monthEnd) {
			continue
		}

		// Determine transaction count based on activity score and account type
		txnCount := g.calculateMonthlyTransactionCount(account)

		// Generate and write transactions for this account this month
		if err := g.generateAccountMonthTransactions(
			account, customerAccounts, balances, monthStart, monthEnd, txnCount,
		); err != nil {
			return err
		}
	}

	return nil
}

// calculateMonthlyTransactionCount determines how many transactions an account should have
func (g *StreamingTransactionGenerator) calculateMonthlyTransactionCount(account GeneratedAccount) int {
	baseCount := g.config.TransactionsPerCustomerPerMonth

	// Adjust by activity score (from Pareto distribution)
	activityScore := account.Customer.Customer.ActivityScore
	adjustedCount := g.activityDist.TransactionsPerMonth(activityScore, baseCount)

	// Adjust by account type
	switch account.Account.Type {
	case models.AccountTypeChecking:
		adjustedCount = int(float64(adjustedCount) * 1.2)
	case models.AccountTypeSavings:
		adjustedCount = int(float64(adjustedCount) * 0.3)
	case models.AccountTypeCreditCard:
		adjustedCount = int(float64(adjustedCount) * 1.5)
	case models.AccountTypeBusiness:
		adjustedCount = int(float64(adjustedCount) * 2.0)
	case models.AccountTypeMerchant:
		adjustedCount = int(float64(adjustedCount) * 5.0)
	case models.AccountTypePayroll:
		adjustedCount = int(float64(adjustedCount) * 0.5)
	}

	if adjustedCount < 1 {
		adjustedCount = 1
	}

	variance := g.rng.IntRange(-adjustedCount/4, adjustedCount/4)
	return adjustedCount + variance
}

// generateAccountMonthTransactions generates and writes transactions for one account in one month
func (g *StreamingTransactionGenerator) generateAccountMonthTransactions(
	account GeneratedAccount,
	customerAccounts map[int64][]GeneratedAccount,
	balances map[int64]int64,
	monthStart, monthEnd time.Time,
	targetCount int,
) error {
	pattern := g.selectPattern(account)
	timestamps := g.generateTimestamps(monthStart, monthEnd, targetCount, pattern, account)

	for _, ts := range timestamps {
		txnType, channel := g.selectTransactionType(account, ts)
		amount := g.generateAmount(txnType, account)

		status := models.TxStatusCompleted
		var failureReason *string
		if g.shouldDecline(txnType, balances[account.Account.ID], amount) {
			status = models.TxStatusDeclined
			reason := "insufficient_funds"
			failureReason = &reason
			amount = 0
		}

		var counterpartyID *int64
		var beneficiaryID *int64
		counterpartyID, beneficiaryID = g.selectCounterparty(txnType, account, customerAccounts)

		balanceAfter := balances[account.Account.ID]
		if status == models.TxStatusCompleted && amount > 0 {
			if isDebitType(txnType) {
				balanceAfter -= amount
			} else {
				balanceAfter += amount
			}
			balances[account.Account.ID] = balanceAfter
		}

		description := g.generateDescription(txnType, channel, account)
		branchID, atmID := g.selectLocation(channel, account)

		txn := models.Transaction{
			ID:                    g.currentID,
			ReferenceNumber:       g.generateReferenceNumber(g.currentID, ts),
			AccountID:             account.Account.ID,
			CounterpartyAccountID: counterpartyID,
			BeneficiaryID:         beneficiaryID,
			Type:                  txnType,
			Status:                status,
			Channel:               channel,
			Amount:                amount,
			Currency:              account.Account.Currency,
			BalanceAfter:          balanceAfter,
			Description:           description,
			Metadata:              "{}",
			BranchID:              branchID,
			ATMID:                 atmID,
			Timestamp:             ts,
			PostedAt:              ts.Add(time.Duration(g.rng.IntRange(0, 60)) * time.Second),
			ValueDate:             ts,
			FailureReason:         failureReason,
		}

		g.currentID++

		// Write transaction immediately
		if err := g.writeTransaction(txn); err != nil {
			return err
		}

		// Generate counterparty transaction for internal transfers
		if counterpartyID != nil && status == models.TxStatusCompleted {
			if err := g.generateAndWriteCounterpartyTransaction(txn, *counterpartyID, balances); err != nil {
				return err
			}
		}
	}

	return nil
}

// writeTransaction formats and writes a transaction to CSV
func (g *StreamingTransactionGenerator) writeTransaction(t models.Transaction) error {
	row := []string{
		FormatInt64(t.ID),
		t.ReferenceNumber,
		FormatInt64(t.AccountID),
		FormatInt64Ptr(t.CounterpartyAccountID),
		FormatInt64Ptr(t.BeneficiaryID),
		string(t.Type),
		string(t.Status),
		string(t.Channel),
		FormatInt64(t.Amount),
		string(t.Currency),
		FormatInt64(t.BalanceAfter),
		t.Description,
		t.Metadata,
		FormatInt64Ptr(t.BranchID),
		FormatInt64Ptr(t.ATMID),
		FormatInt64Ptr(t.LinkedTransactionID),
		FormatTime(t.Timestamp),
		FormatTime(t.PostedAt),
		FormatDate(t.ValueDate),
		formatStringPtr(t.FailureReason),
	}

	if err := g.writer.WriteRow(row); err != nil {
		return err
	}

	g.count++

	// Report progress every 1000 transactions
	if g.progressChan != nil && g.count%1000 == 0 {
		select {
		case g.progressChan <- workerProgress{workerID: g.workerID, count: g.count}:
		default:
			// Non-blocking send
		}
	}

	return nil
}

// generateAndWriteCounterpartyTransaction creates and writes the other side of a transfer
func (g *StreamingTransactionGenerator) generateAndWriteCounterpartyTransaction(
	original models.Transaction,
	counterpartyID int64,
	balances map[int64]int64,
) error {
	var counterType models.TransactionType
	if isDebitType(original.Type) {
		counterType = models.TxTypeTransferIn
	} else {
		counterType = models.TxTypeTransferOut
	}

	// Update counterparty balance (only if we track it in this worker)
	balanceAfter := balances[counterpartyID]
	if _, exists := balances[counterpartyID]; exists {
		if isDebitType(counterType) {
			balanceAfter -= original.Amount
		} else {
			balanceAfter += original.Amount
		}
		balances[counterpartyID] = balanceAfter
	}

	linkedID := original.ID
	counterTxn := models.Transaction{
		ID:                    g.currentID,
		ReferenceNumber:       original.ReferenceNumber,
		AccountID:             counterpartyID,
		CounterpartyAccountID: &original.AccountID,
		Type:                  counterType,
		Status:                original.Status,
		Channel:               original.Channel,
		Amount:                original.Amount,
		Currency:              original.Currency,
		BalanceAfter:          balanceAfter,
		Description:           "Transfer from " + original.ReferenceNumber,
		Metadata:              "{}",
		LinkedTransactionID:   &linkedID,
		Timestamp:             original.Timestamp,
		PostedAt:              original.PostedAt,
		ValueDate:             original.ValueDate,
	}
	g.currentID++

	return g.writeTransaction(counterTxn)
}

// selectPattern chooses the appropriate time pattern for an account
func (g *StreamingTransactionGenerator) selectPattern(account GeneratedAccount) *patterns.FullPattern {
	if account.Customer.Customer.IsBusinessCustomer() {
		return g.businessPattern
	}

	switch account.Account.Type {
	case models.AccountTypeMerchant, models.AccountTypeBusiness, models.AccountTypePayroll:
		return g.businessPattern
	default:
		return g.retailPattern
	}
}

// generateTimestamps creates realistic timestamps distributed across a month
func (g *StreamingTransactionGenerator) generateTimestamps(
	start, end time.Time,
	count int,
	pattern *patterns.FullPattern,
	account GeneratedAccount,
) []time.Time {
	timestamps := make([]time.Time, 0, count)
	duration := end.Sub(start)

	for i := 0; i < count; i++ {
		offset := time.Duration(g.rng.Float64() * float64(duration))
		ts := start.Add(offset)

		hour, minute := patterns.NewDailyPattern().TimeInActiveWindow(g.rng.Float64())
		ts = time.Date(ts.Year(), ts.Month(), ts.Day(), hour, minute, g.rng.IntRange(0, 59), 0, time.UTC)

		if tz, err := time.LoadLocation(account.Customer.Customer.Timezone); err == nil {
			ts = ts.In(tz)
		}

		if g.rng.Float64() < pattern.GetMultiplier(ts) {
			timestamps = append(timestamps, ts)
		} else {
			i--
		}
	}

	return timestamps
}

// selectTransactionType chooses an appropriate transaction type for the account
func (g *StreamingTransactionGenerator) selectTransactionType(account GeneratedAccount, ts time.Time) (models.TransactionType, models.TransactionChannel) {
	monthlyPattern := patterns.NewMonthlyPattern()
	if monthlyPattern.IsPayrollDay(ts.Day()) && account.Account.Type == models.AccountTypePayroll {
		return models.TxTypePayrollBatch, models.ChannelInternal
	}

	switch account.Account.Type {
	case models.AccountTypeChecking:
		return g.selectCheckingTransactionType(ts)
	case models.AccountTypeSavings:
		return g.selectSavingsTransactionType()
	case models.AccountTypeCreditCard:
		return g.selectCreditCardTransactionType()
	case models.AccountTypeBusiness:
		return g.selectBusinessTransactionType(ts)
	case models.AccountTypeMerchant:
		return models.TxTypeDeposit, models.ChannelPOS
	case models.AccountTypePayroll:
		return g.selectPayrollTransactionType(ts)
	default:
		return models.TxTypeDeposit, models.ChannelOnline
	}
}

func (g *StreamingTransactionGenerator) selectCheckingTransactionType(ts time.Time) (models.TransactionType, models.TransactionChannel) {
	r := g.rng.Float64()
	monthlyPattern := patterns.NewMonthlyPattern()

	if monthlyPattern.IsPayrollDay(ts.Day()) && r < 0.15 {
		return models.TxTypeSalary, models.ChannelACH
	}
	if monthlyPattern.IsStartOfMonth(ts) && r < 0.25 {
		return models.TxTypeBillPayment, models.ChannelOnline
	}

	switch {
	case r < 0.20:
		return models.TxTypeWithdrawal, models.ChannelATM
	case r < 0.35:
		return models.TxTypePurchase, models.ChannelPOS
	case r < 0.50:
		return models.TxTypeTransferOut, models.ChannelOnline
	case r < 0.60:
		return models.TxTypeBillPayment, models.ChannelOnline
	case r < 0.75:
		return models.TxTypeDeposit, models.ChannelBranch
	case r < 0.85:
		return models.TxTypeTransferIn, models.ChannelOnline
	case r < 0.95:
		return models.TxTypeSalary, models.ChannelACH
	default:
		return models.TxTypeFee, models.ChannelInternal
	}
}

func (g *StreamingTransactionGenerator) selectSavingsTransactionType() (models.TransactionType, models.TransactionChannel) {
	r := g.rng.Float64()
	switch {
	case r < 0.40:
		return models.TxTypeTransferIn, models.ChannelOnline
	case r < 0.70:
		return models.TxTypeTransferOut, models.ChannelOnline
	case r < 0.85:
		return models.TxTypeInterestCredit, models.ChannelInternal
	case r < 0.95:
		return models.TxTypeDeposit, models.ChannelBranch
	default:
		return models.TxTypeFee, models.ChannelInternal
	}
}

func (g *StreamingTransactionGenerator) selectCreditCardTransactionType() (models.TransactionType, models.TransactionChannel) {
	r := g.rng.Float64()
	switch {
	case r < 0.65:
		return models.TxTypePurchase, models.ChannelPOS
	case r < 0.80:
		return models.TxTypePurchase, models.ChannelOnline
	case r < 0.90:
		return models.TxTypeDeposit, models.ChannelOnline
	case r < 0.95:
		return models.TxTypeRefund, models.ChannelPOS
	default:
		return models.TxTypeInterestDebit, models.ChannelInternal
	}
}

func (g *StreamingTransactionGenerator) selectBusinessTransactionType(ts time.Time) (models.TransactionType, models.TransactionChannel) {
	r := g.rng.Float64()
	switch {
	case r < 0.30:
		return models.TxTypeDeposit, models.ChannelACH
	case r < 0.50:
		return models.TxTypeTransferOut, models.ChannelWire
	case r < 0.65:
		return models.TxTypeBillPayment, models.ChannelOnline
	case r < 0.80:
		return models.TxTypeTransferIn, models.ChannelACH
	case r < 0.90:
		return models.TxTypeWithdrawal, models.ChannelBranch
	default:
		return models.TxTypeFee, models.ChannelInternal
	}
}

func (g *StreamingTransactionGenerator) selectPayrollTransactionType(ts time.Time) (models.TransactionType, models.TransactionChannel) {
	monthlyPattern := patterns.NewMonthlyPattern()
	if monthlyPattern.IsPayrollDay(ts.Day()) {
		return models.TxTypePayrollBatch, models.ChannelInternal
	}
	r := g.rng.Float64()
	if r < 0.7 {
		return models.TxTypeTransferIn, models.ChannelInternal
	}
	return models.TxTypeFee, models.ChannelInternal
}

// generateAmount creates a realistic transaction amount
func (g *StreamingTransactionGenerator) generateAmount(txnType models.TransactionType, account GeneratedAccount) int64 {
	var dist *patterns.AmountDistribution

	switch txnType {
	case models.TxTypeWithdrawal:
		dist = g.amounts.ATMWithdrawal
	case models.TxTypePurchase:
		r := g.rng.Float64()
		switch {
		case r < 0.5:
			dist = g.amounts.SmallPurchase
		case r < 0.85:
			dist = g.amounts.MediumPurchase
		default:
			dist = g.amounts.LargePurchase
		}
	case models.TxTypeBillPayment:
		dist = g.amounts.BillPayment
	case models.TxTypeSalary:
		dist = g.amounts.Salary
	case models.TxTypeTransferIn, models.TxTypeTransferOut:
		dist = g.amounts.InternalTransfer
	case models.TxTypePayrollBatch:
		return g.rng.Int64Range(50000000, 500000000)
	case models.TxTypeInterestCredit, models.TxTypeInterestDebit:
		balance := account.Account.Balance
		if balance < 0 {
			balance = -balance
		}
		rate := float64(account.Account.InterestRate) / 10000 / 12
		return int64(float64(balance) * rate)
	case models.TxTypeFee:
		return g.rng.Int64Range(500, 5000)
	case models.TxTypeRefund:
		dist = g.amounts.MediumPurchase
	case models.TxTypeCashback:
		return g.rng.Int64Range(100, 2000)
	default:
		dist = g.amounts.MediumPurchase
	}

	if dist == nil {
		return g.rng.Int64Range(1000, 10000)
	}

	return dist.GenerateAmount(g.rng.Float64(), g.rng.NormalFloat64())
}

func (g *StreamingTransactionGenerator) shouldDecline(txnType models.TransactionType, balance, amount int64) bool {
	if !isDebitType(txnType) {
		return false
	}
	if g.rng.Probability(g.config.DeclinedTransactionRate) {
		return true
	}
	if g.rng.Probability(g.config.InsufficientFundsRate) && balance < amount {
		return true
	}
	return false
}

func (g *StreamingTransactionGenerator) selectCounterparty(
	txnType models.TransactionType,
	account GeneratedAccount,
	customerAccounts map[int64][]GeneratedAccount,
) (*int64, *int64) {
	switch txnType {
	case models.TxTypeTransferIn, models.TxTypeTransferOut:
		accounts := customerAccounts[account.Account.CustomerID]
		for _, acc := range accounts {
			if acc.Account.ID != account.Account.ID {
				id := acc.Account.ID
				return &id, nil
			}
		}
	case models.TxTypePurchase:
		if len(g.merchantAccountIDs) > 0 {
			id := g.merchantAccountIDs[g.rng.IntN(len(g.merchantAccountIDs))]
			return &id, nil
		}
	case models.TxTypeBillPayment:
		if len(g.utilityAccountIDs) > 0 {
			id := g.utilityAccountIDs[g.rng.IntN(len(g.utilityAccountIDs))]
			return &id, nil
		}
	case models.TxTypeSalary:
		if len(g.employerAccountIDs) > 0 {
			id := g.employerAccountIDs[g.rng.IntN(len(g.employerAccountIDs))]
			return &id, nil
		}
	}
	return nil, nil
}

func (g *StreamingTransactionGenerator) selectLocation(channel models.TransactionChannel, account GeneratedAccount) (*int64, *int64) {
	switch channel {
	case models.ChannelATM:
		if len(g.atms) > 0 {
			atm := g.atms[g.rng.IntN(len(g.atms))]
			return nil, &atm.ATM.ID
		}
	case models.ChannelBranch:
		if len(g.branches) > 0 {
			branch := g.branches[g.rng.IntN(len(g.branches))]
			return &branch.Branch.ID, nil
		}
	}
	return nil, nil
}

func (g *StreamingTransactionGenerator) generateDescription(
	txnType models.TransactionType,
	channel models.TransactionChannel,
	account GeneratedAccount,
) string {
	switch txnType {
	case models.TxTypeWithdrawal:
		return fmt.Sprintf("ATM Withdrawal - %s", g.pickLocation(account))
	case models.TxTypePurchase:
		return fmt.Sprintf("POS Purchase - %s", g.pickMerchantName())
	case models.TxTypeBillPayment:
		return fmt.Sprintf("Bill Payment - %s", g.pickUtilityName())
	case models.TxTypeSalary:
		return "Direct Deposit - Payroll"
	case models.TxTypeTransferIn:
		return "Transfer from linked account"
	case models.TxTypeTransferOut:
		return "Transfer to linked account"
	case models.TxTypeDeposit:
		if channel == models.ChannelBranch {
			return "Branch Deposit"
		}
		return "Mobile Deposit"
	case models.TxTypeInterestCredit:
		return "Interest Payment"
	case models.TxTypeInterestDebit:
		return "Interest Charge"
	case models.TxTypeFee:
		return g.pickFeeName()
	case models.TxTypeRefund:
		return "Refund - " + g.pickMerchantName()
	case models.TxTypeCashback:
		return "Cashback Reward"
	case models.TxTypePayrollBatch:
		return "Payroll Disbursement"
	case models.TxTypeLoanPayment:
		return "Loan Payment"
	default:
		return "Transaction"
	}
}

func (g *StreamingTransactionGenerator) pickLocation(account GeneratedAccount) string {
	locations := []string{
		"Main Street", "Downtown", "Airport Terminal", "Mall",
		"University", "Hospital", "Train Station", "Shopping Center",
	}
	city := account.Account.Currency
	return fmt.Sprintf("%s - %s", locations[g.rng.IntN(len(locations))], city)
}

func (g *StreamingTransactionGenerator) pickMerchantName() string {
	merchants := []string{
		"AMAZON", "WALMART", "TARGET", "STARBUCKS", "UBER",
		"NETFLIX", "SPOTIFY", "APPLE", "GOOGLE", "DOORDASH",
		"COSTCO", "WHOLE FOODS", "CVS PHARMACY", "SHELL GAS",
		"MCDONALDS", "SUBWAY", "HOME DEPOT", "BEST BUY",
	}
	return merchants[g.rng.IntN(len(merchants))]
}

func (g *StreamingTransactionGenerator) pickUtilityName() string {
	utilities := []string{
		"Electric Company", "Gas & Power", "Water Services",
		"Internet Provider", "Phone Company", "Insurance Co",
		"City Services", "Waste Management",
	}
	return utilities[g.rng.IntN(len(utilities))]
}

func (g *StreamingTransactionGenerator) pickFeeName() string {
	fees := []string{
		"Monthly Maintenance Fee", "ATM Fee", "Wire Transfer Fee",
		"Overdraft Fee", "Paper Statement Fee", "Foreign Transaction Fee",
	}
	return fees[g.rng.IntN(len(fees))]
}

func (g *StreamingTransactionGenerator) generateReferenceNumber(id int64, ts time.Time) string {
	return fmt.Sprintf("TXN%s%012d", ts.Format("20060102"), id)
}

// ShardFile returns the path to the shard file created by this generator
func (g *StreamingTransactionGenerator) ShardFile() string {
	return g.writer.Path()
}

// Count returns the number of transactions written
func (g *StreamingTransactionGenerator) Count() int64 {
	return g.count
}
