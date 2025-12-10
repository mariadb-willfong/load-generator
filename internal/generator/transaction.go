package generator

import (
	"fmt"
	"time"

	"github.com/willfong/load-generator/internal/data"
	"github.com/willfong/load-generator/internal/generator/patterns"
	"github.com/willfong/load-generator/internal/models"
	"github.com/willfong/load-generator/internal/utils"
)

// TransactionGenerator creates historical transactions for accounts.
type TransactionGenerator struct {
	rng     *utils.Random
	refData *data.ReferenceData
	config  TransactionGeneratorConfig

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

	// Merchant account IDs for purchase destinations
	merchantAccountIDs []int64
	// Employer account IDs for salary sources
	employerAccountIDs []int64
	// Utility account IDs for bill payments
	utilityAccountIDs []int64
}

// TransactionGeneratorConfig holds settings for transaction generation
type TransactionGeneratorConfig struct {
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

	// Reference data for generating transaction context
	Branches   []GeneratedBranch
	ATMs       []GeneratedATM
	Accounts   []GeneratedAccount
	Businesses []GeneratedBusiness
}

// NewTransactionGenerator creates a new transaction generator
func NewTransactionGenerator(rng *utils.Random, refData *data.ReferenceData, config TransactionGeneratorConfig) *TransactionGenerator {
	tg := &TransactionGenerator{
		rng:     rng,
		refData: refData,
		config:  config,

		retailPattern:   patterns.NewDefaultFullPattern(),
		atmPattern:      patterns.NewATMFullPattern(),
		onlinePattern:   patterns.NewOnlineFullPattern(),
		businessPattern: patterns.NewBusinessFullPattern(),

		activityDist: patterns.NewParetoDistribution(config.ParetoRatio),
		amounts:      patterns.NewTransactionTypeAmounts(),

		branches: config.Branches,
		atms:     config.ATMs,
	}

	// Categorize business accounts by type
	for _, acc := range config.Accounts {
		switch acc.Account.Type {
		case models.AccountTypeMerchant:
			tg.merchantAccountIDs = append(tg.merchantAccountIDs, acc.Account.ID)
		case models.AccountTypePayroll:
			tg.employerAccountIDs = append(tg.employerAccountIDs, acc.Account.ID)
		}
	}

	// Add utility company accounts from businesses
	for _, biz := range config.Businesses {
		if biz.BusinessType == BusinessTypeUtility {
			for _, acc := range config.Accounts {
				if acc.Account.CustomerID == biz.Customer.ID {
					tg.utilityAccountIDs = append(tg.utilityAccountIDs, acc.Account.ID)
					break
				}
			}
		}
	}

	return tg
}

// GeneratedTransaction holds a transaction with metadata
type GeneratedTransaction struct {
	Transaction models.Transaction
	Account     GeneratedAccount
}

// GenerateTransactionsForAccounts creates historical transactions for all accounts
// Returns transactions sorted chronologically and the next available transaction ID
func (g *TransactionGenerator) GenerateTransactionsForAccounts(accounts []GeneratedAccount, startID int64) ([]GeneratedTransaction, int64) {
	// Group accounts by customer for coordinated generation
	customerAccounts := make(map[int64][]GeneratedAccount)
	for _, acc := range accounts {
		customerAccounts[acc.Account.CustomerID] = append(customerAccounts[acc.Account.CustomerID], acc)
	}

	// Track running balances
	balances := make(map[int64]int64)
	for _, acc := range accounts {
		balances[acc.Account.ID] = acc.Account.Balance
	}

	// Estimate capacity: avg 15 txns/month × num accounts × months of history
	months := int(g.config.EndDate.Sub(g.config.StartDate).Hours() / (24 * 30))
	estimatedCapacity := len(accounts) * g.config.TransactionsPerCustomerPerMonth * months
	transactions := make([]GeneratedTransaction, 0, estimatedCapacity)

	currentID := startID

	// Generate month by month
	currentMonth := g.config.StartDate
	for currentMonth.Before(g.config.EndDate) {
		monthEnd := currentMonth.AddDate(0, 1, 0)
		if monthEnd.After(g.config.EndDate) {
			monthEnd = g.config.EndDate
		}

		monthTxns := g.generateMonthTransactions(accounts, customerAccounts, balances, currentMonth, monthEnd, &currentID)
		transactions = append(transactions, monthTxns...)

		currentMonth = currentMonth.AddDate(0, 1, 0)
	}

	return transactions, currentID
}

// generateMonthTransactions generates transactions for a single month
func (g *TransactionGenerator) generateMonthTransactions(
	accounts []GeneratedAccount,
	customerAccounts map[int64][]GeneratedAccount,
	balances map[int64]int64,
	monthStart, monthEnd time.Time,
	currentID *int64,
) []GeneratedTransaction {
	transactions := make([]GeneratedTransaction, 0)

	// Generate transactions for each account
	for _, account := range accounts {
		// Skip closed accounts or accounts opened after this month
		if account.Account.OpenedAt.After(monthEnd) {
			continue
		}

		// Determine transaction count based on activity score and account type
		txnCount := g.calculateMonthlyTransactionCount(account)

		// Generate transactions distributed across the month
		accountTxns := g.generateAccountMonthTransactions(
			account, customerAccounts, balances, monthStart, monthEnd, txnCount, currentID,
		)
		transactions = append(transactions, accountTxns...)
	}

	return transactions
}

// calculateMonthlyTransactionCount determines how many transactions an account should have
func (g *TransactionGenerator) calculateMonthlyTransactionCount(account GeneratedAccount) int {
	baseCount := g.config.TransactionsPerCustomerPerMonth

	// Adjust by activity score (from Pareto distribution)
	activityScore := account.Customer.Customer.ActivityScore
	adjustedCount := g.activityDist.TransactionsPerMonth(activityScore, baseCount)

	// Adjust by account type
	switch account.Account.Type {
	case models.AccountTypeChecking:
		adjustedCount = int(float64(adjustedCount) * 1.2) // Primary transaction account
	case models.AccountTypeSavings:
		adjustedCount = int(float64(adjustedCount) * 0.3) // Less activity
	case models.AccountTypeCreditCard:
		adjustedCount = int(float64(adjustedCount) * 1.5) // Many purchases
	case models.AccountTypeBusiness:
		adjustedCount = int(float64(adjustedCount) * 2.0) // High volume
	case models.AccountTypeMerchant:
		adjustedCount = int(float64(adjustedCount) * 5.0) // Very high volume
	case models.AccountTypePayroll:
		adjustedCount = int(float64(adjustedCount) * 0.5) // Concentrated payroll events
	default:
		// Use base adjustment
	}

	// Minimum 1 transaction per month for active accounts
	if adjustedCount < 1 {
		adjustedCount = 1
	}

	// Add some randomness
	variance := g.rng.IntRange(-adjustedCount/4, adjustedCount/4)
	return adjustedCount + variance
}

// generateAccountMonthTransactions generates transactions for one account in one month
func (g *TransactionGenerator) generateAccountMonthTransactions(
	account GeneratedAccount,
	customerAccounts map[int64][]GeneratedAccount,
	balances map[int64]int64,
	monthStart, monthEnd time.Time,
	targetCount int,
	currentID *int64,
) []GeneratedTransaction {
	transactions := make([]GeneratedTransaction, 0, targetCount)

	// Select pattern based on customer segment and account type
	pattern := g.selectPattern(account)

	// Generate transaction timestamps distributed across the month
	timestamps := g.generateTimestamps(monthStart, monthEnd, targetCount, pattern, account)

	for _, ts := range timestamps {
		// Select transaction type based on account type and timing
		txnType, channel := g.selectTransactionType(account, ts)

		// Generate amount
		amount := g.generateAmount(txnType, account)

		// Check if this should be a declined transaction
		status := models.TxStatusCompleted
		var failureReason *string
		if g.shouldDecline(txnType, balances[account.Account.ID], amount) {
			status = models.TxStatusDeclined
			reason := "insufficient_funds"
			failureReason = &reason
			amount = 0 // Declined transactions have no effect
		}

		// Get counterparty if applicable
		var counterpartyID *int64
		var beneficiaryID *int64
		counterpartyID, beneficiaryID = g.selectCounterparty(txnType, account, customerAccounts)

		// Update balance for successful transactions
		balanceAfter := balances[account.Account.ID]
		if status == models.TxStatusCompleted && amount > 0 {
			if isDebitType(txnType) {
				balanceAfter -= amount
			} else {
				balanceAfter += amount
			}
			balances[account.Account.ID] = balanceAfter
		}

		// Generate transaction description
		description := g.generateDescription(txnType, channel, account)

		// Get branch/ATM IDs
		branchID, atmID := g.selectLocation(channel, account)

		txn := models.Transaction{
			ID:                    *currentID,
			ReferenceNumber:       g.generateReferenceNumber(*currentID, ts),
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

		*currentID++

		transactions = append(transactions, GeneratedTransaction{
			Transaction: txn,
			Account:     account,
		})

		// Generate the counterparty side of the transaction for internal transfers
		if counterpartyID != nil && status == models.TxStatusCompleted {
			linkedTxn := g.generateCounterpartyTransaction(txn, *counterpartyID, balances, currentID)
			if linkedTxn != nil {
				// Find the counterparty account for the GeneratedTransaction
				for _, acc := range g.config.Accounts {
					if acc.Account.ID == *counterpartyID {
						transactions = append(transactions, GeneratedTransaction{
							Transaction: *linkedTxn,
							Account:     acc,
						})
						break
					}
				}
			}
		}
	}

	return transactions
}

// selectPattern chooses the appropriate time pattern for an account
func (g *TransactionGenerator) selectPattern(account GeneratedAccount) *patterns.FullPattern {
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
func (g *TransactionGenerator) generateTimestamps(
	start, end time.Time,
	count int,
	pattern *patterns.FullPattern,
	account GeneratedAccount,
) []time.Time {
	timestamps := make([]time.Time, 0, count)
	duration := end.Sub(start)

	for i := 0; i < count; i++ {
		// Generate a random point in the month
		offset := time.Duration(g.rng.Float64() * float64(duration))
		ts := start.Add(offset)

		// Adjust to realistic hours using daily pattern
		hour, minute := patterns.NewDailyPattern().TimeInActiveWindow(g.rng.Float64())
		ts = time.Date(ts.Year(), ts.Month(), ts.Day(), hour, minute, g.rng.IntRange(0, 59), 0, time.UTC)

		// Apply timezone offset for the customer
		if tz, err := time.LoadLocation(account.Customer.Customer.Timezone); err == nil {
			ts = ts.In(tz)
		}

		// Accept based on pattern multiplier (rejection sampling)
		if g.rng.Float64() < pattern.GetMultiplier(ts) {
			timestamps = append(timestamps, ts)
		} else {
			// Retry with another timestamp
			i--
		}
	}

	return timestamps
}

// selectTransactionType chooses an appropriate transaction type for the account
func (g *TransactionGenerator) selectTransactionType(account GeneratedAccount, ts time.Time) (models.TransactionType, models.TransactionChannel) {
	// Check for payroll day
	monthlyPattern := patterns.NewMonthlyPattern()
	if monthlyPattern.IsPayrollDay(ts.Day()) && account.Account.Type == models.AccountTypePayroll {
		return models.TxTypePayrollBatch, models.ChannelInternal
	}

	// Account type specific logic
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
		return models.TxTypeDeposit, models.ChannelPOS // Merchants receive payments

	case models.AccountTypePayroll:
		return g.selectPayrollTransactionType(ts)

	default:
		return models.TxTypeDeposit, models.ChannelOnline
	}
}

// selectCheckingTransactionType chooses transaction type for checking accounts
func (g *TransactionGenerator) selectCheckingTransactionType(ts time.Time) (models.TransactionType, models.TransactionChannel) {
	r := g.rng.Float64()

	// Monthly patterns influence transaction types
	monthlyPattern := patterns.NewMonthlyPattern()

	// Salary deposits around payroll days
	if monthlyPattern.IsPayrollDay(ts.Day()) && r < 0.15 {
		return models.TxTypeSalary, models.ChannelACH
	}

	// Bill payments at start of month
	if monthlyPattern.IsStartOfMonth(ts) && r < 0.25 {
		return models.TxTypeBillPayment, models.ChannelOnline
	}

	// Weighted distribution of transaction types
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

// selectSavingsTransactionType chooses transaction type for savings accounts
func (g *TransactionGenerator) selectSavingsTransactionType() (models.TransactionType, models.TransactionChannel) {
	r := g.rng.Float64()

	switch {
	case r < 0.40:
		return models.TxTypeTransferIn, models.ChannelOnline // Deposits from checking
	case r < 0.70:
		return models.TxTypeTransferOut, models.ChannelOnline // Withdrawals to checking
	case r < 0.85:
		return models.TxTypeInterestCredit, models.ChannelInternal
	case r < 0.95:
		return models.TxTypeDeposit, models.ChannelBranch
	default:
		return models.TxTypeFee, models.ChannelInternal
	}
}

// selectCreditCardTransactionType chooses transaction type for credit cards
func (g *TransactionGenerator) selectCreditCardTransactionType() (models.TransactionType, models.TransactionChannel) {
	r := g.rng.Float64()

	switch {
	case r < 0.65:
		return models.TxTypePurchase, models.ChannelPOS
	case r < 0.80:
		return models.TxTypePurchase, models.ChannelOnline // Online purchases
	case r < 0.90:
		return models.TxTypeDeposit, models.ChannelOnline // Payment
	case r < 0.95:
		return models.TxTypeRefund, models.ChannelPOS
	default:
		return models.TxTypeInterestDebit, models.ChannelInternal
	}
}

// selectBusinessTransactionType chooses transaction type for business accounts
func (g *TransactionGenerator) selectBusinessTransactionType(ts time.Time) (models.TransactionType, models.TransactionChannel) {
	r := g.rng.Float64()

	switch {
	case r < 0.30:
		return models.TxTypeDeposit, models.ChannelACH // Customer payments
	case r < 0.50:
		return models.TxTypeTransferOut, models.ChannelWire // Supplier payments
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

// selectPayrollTransactionType chooses transaction type for payroll accounts
func (g *TransactionGenerator) selectPayrollTransactionType(ts time.Time) (models.TransactionType, models.TransactionChannel) {
	monthlyPattern := patterns.NewMonthlyPattern()

	if monthlyPattern.IsPayrollDay(ts.Day()) {
		return models.TxTypePayrollBatch, models.ChannelInternal
	}

	// Outside payroll days, mainly deposits to fund payroll
	r := g.rng.Float64()
	if r < 0.7 {
		return models.TxTypeTransferIn, models.ChannelInternal
	}
	return models.TxTypeFee, models.ChannelInternal
}

// generateAmount creates a realistic transaction amount
func (g *TransactionGenerator) generateAmount(txnType models.TransactionType, account GeneratedAccount) int64 {
	var dist *patterns.AmountDistribution

	switch txnType {
	case models.TxTypeWithdrawal:
		dist = g.amounts.ATMWithdrawal
	case models.TxTypePurchase:
		// Vary by purchase size
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
		// Large payroll amount
		return g.rng.Int64Range(50000000, 500000000) // $500k - $5M
	case models.TxTypeInterestCredit, models.TxTypeInterestDebit:
		// Calculate based on balance
		balance := account.Account.Balance
		if balance < 0 {
			balance = -balance
		}
		rate := float64(account.Account.InterestRate) / 10000 / 12 // Monthly rate
		return int64(float64(balance) * rate)
	case models.TxTypeFee:
		return g.rng.Int64Range(500, 5000) // $5 - $50
	case models.TxTypeRefund:
		dist = g.amounts.MediumPurchase // Refunds are usually for previous purchases
	case models.TxTypeCashback:
		return g.rng.Int64Range(100, 2000) // $1 - $20
	default:
		dist = g.amounts.MediumPurchase
	}

	if dist == nil {
		return g.rng.Int64Range(1000, 10000)
	}

	return dist.GenerateAmount(g.rng.Float64(), g.rng.NormalFloat64())
}

// shouldDecline determines if a transaction should be declined
func (g *TransactionGenerator) shouldDecline(txnType models.TransactionType, balance, amount int64) bool {
	// Only decline debit transactions
	if !isDebitType(txnType) {
		return false
	}

	// Random decline based on configured rate
	if g.rng.Probability(g.config.DeclinedTransactionRate) {
		return true
	}

	// Insufficient funds check
	if g.rng.Probability(g.config.InsufficientFundsRate) && balance < amount {
		return true
	}

	return false
}

// selectCounterparty selects a counterparty account for transfers
func (g *TransactionGenerator) selectCounterparty(
	txnType models.TransactionType,
	account GeneratedAccount,
	customerAccounts map[int64][]GeneratedAccount,
) (*int64, *int64) {
	switch txnType {
	case models.TxTypeTransferIn, models.TxTypeTransferOut:
		// Internal transfer between customer's accounts
		accounts := customerAccounts[account.Account.CustomerID]
		for _, acc := range accounts {
			if acc.Account.ID != account.Account.ID {
				id := acc.Account.ID
				return &id, nil
			}
		}

	case models.TxTypePurchase:
		// Purchase goes to a merchant
		if len(g.merchantAccountIDs) > 0 {
			id := g.merchantAccountIDs[g.rng.IntN(len(g.merchantAccountIDs))]
			return &id, nil
		}

	case models.TxTypeBillPayment:
		// Bill payment to utility
		if len(g.utilityAccountIDs) > 0 {
			id := g.utilityAccountIDs[g.rng.IntN(len(g.utilityAccountIDs))]
			return &id, nil
		}

	case models.TxTypeSalary:
		// Salary from employer
		if len(g.employerAccountIDs) > 0 {
			id := g.employerAccountIDs[g.rng.IntN(len(g.employerAccountIDs))]
			return &id, nil
		}

	case models.TxTypePayrollBatch:
		// Payroll batch creates many individual salary transactions
		// The counterparty is tracked differently for batch operations
		return nil, nil
	}

	return nil, nil
}

// generateCounterpartyTransaction creates the other side of a transfer
func (g *TransactionGenerator) generateCounterpartyTransaction(
	original models.Transaction,
	counterpartyID int64,
	balances map[int64]int64,
	currentID *int64,
) *models.Transaction {
	// Determine the counterparty transaction type
	var counterType models.TransactionType
	if isDebitType(original.Type) {
		counterType = models.TxTypeTransferIn // Debit on source = Credit on destination
	} else {
		counterType = models.TxTypeTransferOut // Credit on source = Debit on destination
	}

	// Update counterparty balance
	balanceAfter := balances[counterpartyID]
	if isDebitType(counterType) {
		balanceAfter -= original.Amount
	} else {
		balanceAfter += original.Amount
	}
	balances[counterpartyID] = balanceAfter

	linkedID := original.ID
	counterTxn := models.Transaction{
		ID:                    *currentID,
		ReferenceNumber:       original.ReferenceNumber, // Same reference
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
	*currentID++

	return &counterTxn
}

// selectLocation picks a branch or ATM for the transaction
func (g *TransactionGenerator) selectLocation(channel models.TransactionChannel, account GeneratedAccount) (*int64, *int64) {
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

// generateDescription creates a realistic transaction description
func (g *TransactionGenerator) generateDescription(
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

// pickLocation returns a location name for ATM withdrawals
func (g *TransactionGenerator) pickLocation(account GeneratedAccount) string {
	locations := []string{
		"Main Street", "Downtown", "Airport Terminal", "Mall",
		"University", "Hospital", "Train Station", "Shopping Center",
	}
	city := account.Account.Currency // Use currency as proxy for location variety
	return fmt.Sprintf("%s - %s", locations[g.rng.IntN(len(locations))], city)
}

// pickMerchantName returns a realistic merchant name
func (g *TransactionGenerator) pickMerchantName() string {
	merchants := []string{
		"AMAZON", "WALMART", "TARGET", "STARBUCKS", "UBER",
		"NETFLIX", "SPOTIFY", "APPLE", "GOOGLE", "DOORDASH",
		"COSTCO", "WHOLE FOODS", "CVS PHARMACY", "SHELL GAS",
		"MCDONALDS", "SUBWAY", "HOME DEPOT", "BEST BUY",
	}
	return merchants[g.rng.IntN(len(merchants))]
}

// pickUtilityName returns a utility company name
func (g *TransactionGenerator) pickUtilityName() string {
	utilities := []string{
		"Electric Company", "Gas & Power", "Water Services",
		"Internet Provider", "Phone Company", "Insurance Co",
		"City Services", "Waste Management",
	}
	return utilities[g.rng.IntN(len(utilities))]
}

// pickFeeName returns a bank fee description
func (g *TransactionGenerator) pickFeeName() string {
	fees := []string{
		"Monthly Maintenance Fee", "ATM Fee", "Wire Transfer Fee",
		"Overdraft Fee", "Paper Statement Fee", "Foreign Transaction Fee",
	}
	return fees[g.rng.IntN(len(fees))]
}

// generateReferenceNumber creates a unique reference number
func (g *TransactionGenerator) generateReferenceNumber(id int64, ts time.Time) string {
	return fmt.Sprintf("TXN%s%012d", ts.Format("20060102"), id)
}

// isDebitType returns true if the transaction type is a debit
func isDebitType(txnType models.TransactionType) bool {
	switch txnType {
	case models.TxTypeWithdrawal, models.TxTypePurchase, models.TxTypeTransferOut,
		models.TxTypeBillPayment, models.TxTypeInterestDebit, models.TxTypeFee,
		models.TxTypeLoanPayment, models.TxTypePayrollBatch:
		return true
	default:
		return false
	}
}

// WriteTransactionsCSV writes transactions to a CSV file (or .csv.xz if compress=true)
func WriteTransactionsCSV(transactions []GeneratedTransaction, outputDir string, compress bool) error {
	return writeTransactionsCSVInternal(transactions, outputDir, compress, false)
}

// WriteTransactionsCSVWithProgress writes transactions to a CSV file with progress reporting
func WriteTransactionsCSVWithProgress(transactions []GeneratedTransaction, outputDir string, compress bool) error {
	return writeTransactionsCSVInternal(transactions, outputDir, compress, true)
}

// writeTransactionsCSVInternal is the internal implementation with optional progress
func writeTransactionsCSVInternal(transactions []GeneratedTransaction, outputDir string, compress, showProgress bool) error {
	headers := []string{
		"id", "reference_number", "account_id", "counterparty_account_id", "beneficiary_id",
		"type", "status", "channel", "amount", "currency", "balance_after",
		"description", "metadata", "branch_id", "atm_id", "linked_transaction_id",
		"timestamp", "posted_at", "value_date", "failure_reason",
	}

	writer, err := NewCSVWriter(CSVWriterConfig{
		OutputDir: outputDir,
		Filename:  "transactions",
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
			Total: int64(len(transactions)),
			Label: "  Transactions",
		})
	}

	for i, gt := range transactions {
		t := gt.Transaction
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
		if err := writer.WriteRow(row); err != nil {
			return err
		}

		if progress != nil && i%1000 == 0 {
			progress.Set(int64(i + 1))
		}
	}

	if progress != nil {
		progress.Set(int64(len(transactions)))
		progress.Finish()
	}

	return writer.Close()
}

// formatStringPtr formats a *string for CSV
func formatStringPtr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
