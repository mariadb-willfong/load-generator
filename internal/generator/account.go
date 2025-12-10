package generator

import (
	"fmt"
	"time"

	"github.com/willfong/load-generator/internal/data"
	"github.com/willfong/load-generator/internal/models"
	"github.com/willfong/load-generator/internal/utils"
)

// AccountGenerator creates bank accounts for customers.
type AccountGenerator struct {
	rng     *utils.Random
	refData *data.ReferenceData
	config  AccountGeneratorConfig
}

// AccountGeneratorConfig holds settings for account generation
type AccountGeneratorConfig struct {
	// Branches for assigning account branch
	Branches []GeneratedBranch
}

// NewAccountGenerator creates a new account generator
func NewAccountGenerator(rng *utils.Random, refData *data.ReferenceData, config AccountGeneratorConfig) *AccountGenerator {
	return &AccountGenerator{
		rng:     rng,
		refData: refData,
		config:  config,
	}
}

// GeneratedAccount holds a generated account with metadata
type GeneratedAccount struct {
	Account  models.Account
	Country  *data.Country
	Customer GeneratedCustomer
}

// GenerateAccountsForCustomers creates accounts for retail customers
// Returns accounts and the next available account ID
func (g *AccountGenerator) GenerateAccountsForCustomers(customers []GeneratedCustomer, startID int64) ([]GeneratedAccount, int64) {
	accounts := make([]GeneratedAccount, 0, len(customers)*2) // Estimate 2 accounts per customer
	currentID := startID

	for _, customer := range customers {
		customerAccounts := g.generateAccountsForCustomer(customer, &currentID)
		accounts = append(accounts, customerAccounts...)
	}

	return accounts, currentID
}

// GenerateAccountsForBusinesses creates accounts for business entities
func (g *AccountGenerator) GenerateAccountsForBusinesses(businesses []GeneratedBusiness, startID int64) ([]GeneratedAccount, int64) {
	accounts := make([]GeneratedAccount, 0, len(businesses)*2)
	currentID := startID

	for _, business := range businesses {
		businessAccounts := g.generateAccountsForBusiness(business, &currentID)
		accounts = append(accounts, businessAccounts...)
	}

	return accounts, currentID
}

// generateAccountsForCustomer creates 1-3 accounts for a retail customer
func (g *AccountGenerator) generateAccountsForCustomer(customer GeneratedCustomer, currentID *int64) []GeneratedAccount {
	accounts := make([]GeneratedAccount, 0, 3)

	// Everyone gets a checking account
	checking := g.generateAccount(*currentID, customer, models.AccountTypeChecking)
	accounts = append(accounts, checking)
	*currentID++

	// 70% get a savings account
	if g.rng.Probability(0.7) {
		savings := g.generateAccount(*currentID, customer, models.AccountTypeSavings)
		accounts = append(accounts, savings)
		*currentID++
	}

	// Based on segment, add more account types
	switch customer.Customer.Segment {
	case models.SegmentPremium, models.SegmentPrivate:
		// High net worth: investment account (50%), credit card (80%)
		if g.rng.Probability(0.5) {
			investment := g.generateAccount(*currentID, customer, models.AccountTypeInvestment)
			accounts = append(accounts, investment)
			*currentID++
		}
		if g.rng.Probability(0.8) {
			creditCard := g.generateAccount(*currentID, customer, models.AccountTypeCreditCard)
			accounts = append(accounts, creditCard)
			*currentID++
		}

	case models.SegmentRegular:
		// Regular: credit card (40%), occasional loan (10%)
		if g.rng.Probability(0.4) {
			creditCard := g.generateAccount(*currentID, customer, models.AccountTypeCreditCard)
			accounts = append(accounts, creditCard)
			*currentID++
		}
		if g.rng.Probability(0.1) {
			loan := g.generateAccount(*currentID, customer, models.AccountTypeLoan)
			accounts = append(accounts, loan)
			*currentID++
		}
	}

	return accounts
}

// generateAccountsForBusiness creates accounts for a business entity
func (g *AccountGenerator) generateAccountsForBusiness(business GeneratedBusiness, currentID *int64) []GeneratedAccount {
	accounts := make([]GeneratedAccount, 0, 3)

	// Convert business to customer format for account generation
	customer := GeneratedCustomer{
		Customer: business.Customer,
		Country:  business.Country,
	}

	// All businesses get a business checking account
	bizChecking := g.generateAccount(*currentID, customer, models.AccountTypeBusiness)
	accounts = append(accounts, bizChecking)
	*currentID++

	// Based on business type, add specialized accounts
	switch business.BusinessType {
	case BusinessTypeEmployer:
		// Employers get a payroll account
		payroll := g.generateAccount(*currentID, customer, models.AccountTypePayroll)
		accounts = append(accounts, payroll)
		*currentID++

	case BusinessTypeMerchant:
		// Merchants get a merchant account
		merchant := g.generateAccount(*currentID, customer, models.AccountTypeMerchant)
		accounts = append(accounts, merchant)
		*currentID++
	}

	// Most businesses have a savings account (60%)
	if g.rng.Probability(0.6) {
		savings := g.generateAccount(*currentID, customer, models.AccountTypeSavings)
		accounts = append(accounts, savings)
		*currentID++
	}

	return accounts
}

// generateAccount creates a single account
func (g *AccountGenerator) generateAccount(id int64, customer GeneratedCustomer, accountType models.AccountType) GeneratedAccount {
	// Get currency from customer's country
	currency := g.getCurrency(customer.Country.Currency)

	// Generate account number
	accountNumber := g.generateAccountNumber(customer.Country.Code, id)

	// Calculate balance based on account type and customer segment
	balance := g.calculateBalance(accountType, customer.Customer.Segment, currency)

	// Calculate limits
	creditLimit, overdraftLimit := g.calculateLimits(accountType, customer.Customer.Segment, currency)

	// Calculate daily limits
	dailyWithdraw, dailyTransfer := g.calculateDailyLimits(accountType, customer.Customer.Segment, currency)

	// Calculate interest rate
	interestRate := g.calculateInterestRate(accountType)

	// Get branch
	branchID := g.pickBranch(customer.Country.Code)

	// Account opening date (after customer creation)
	openedAt := g.generateOpenedAt(customer.Customer.CreatedAt)

	account := models.Account{
		ID:                 id,
		AccountNumber:      accountNumber,
		CustomerID:         customer.Customer.ID,
		Type:               accountType,
		Status:             models.AccountStatusActive,
		Currency:           currency,
		Balance:            balance,
		CreditLimit:        creditLimit,
		OverdraftLimit:     overdraftLimit,
		DailyWithdrawLimit: dailyWithdraw,
		DailyTransferLimit: dailyTransfer,
		InterestRate:       interestRate,
		BranchID:           branchID,
		OpenedAt:           openedAt,
		UpdatedAt:          time.Now(),
	}

	return GeneratedAccount{
		Account:  account,
		Country:  customer.Country,
		Customer: customer,
	}
}

// getCurrency converts currency code string to Currency type
func (g *AccountGenerator) getCurrency(code string) models.Currency {
	switch code {
	case "USD":
		return models.CurrencyUSD
	case "EUR":
		return models.CurrencyEUR
	case "GBP":
		return models.CurrencyGBP
	case "JPY":
		return models.CurrencyJPY
	case "CHF":
		return models.CurrencyCHF
	case "CAD":
		return models.CurrencyCAD
	case "AUD":
		return models.CurrencyAUD
	case "INR":
		return models.CurrencyINR
	case "CNY":
		return models.CurrencyCNY
	case "SGD":
		return models.CurrencySGD
	case "HKD":
		return models.CurrencyHKD
	case "BRL":
		return models.CurrencyBRL
	case "MXN":
		return models.CurrencyMXN
	default:
		return models.CurrencyUSD // Default to USD
	}
}

// generateAccountNumber creates a realistic account number
func (g *AccountGenerator) generateAccountNumber(countryCode string, id int64) string {
	// Format: CC-BBBBB-XXXXXXXXXX (country-branch-number)
	branchPart := g.rng.NumericString(5)
	accountPart := fmt.Sprintf("%010d", id)
	return fmt.Sprintf("%s-%s-%s", countryCode, branchPart, accountPart)
}

// calculateBalance determines initial balance based on account type and segment
// Returns balance in cents (smallest currency unit)
func (g *AccountGenerator) calculateBalance(accountType models.AccountType, segment models.CustomerSegment, currency models.Currency) int64 {
	// Base balance ranges in cents (USD equivalent)
	var minBalance, maxBalance int64

	switch accountType {
	case models.AccountTypeChecking:
		switch segment {
		case models.SegmentPrivate:
			minBalance, maxBalance = 5000000, 50000000 // $50k - $500k
		case models.SegmentPremium:
			minBalance, maxBalance = 1000000, 10000000 // $10k - $100k
		case models.SegmentCorporate:
			minBalance, maxBalance = 10000000, 100000000 // $100k - $1M
		case models.SegmentBusiness:
			minBalance, maxBalance = 500000, 5000000 // $5k - $50k
		default:
			minBalance, maxBalance = 50000, 1000000 // $500 - $10k
		}

	case models.AccountTypeSavings:
		switch segment {
		case models.SegmentPrivate:
			minBalance, maxBalance = 10000000, 100000000 // $100k - $1M
		case models.SegmentPremium:
			minBalance, maxBalance = 2500000, 25000000 // $25k - $250k
		case models.SegmentCorporate:
			minBalance, maxBalance = 50000000, 500000000 // $500k - $5M
		case models.SegmentBusiness:
			minBalance, maxBalance = 1000000, 10000000 // $10k - $100k
		default:
			minBalance, maxBalance = 100000, 2500000 // $1k - $25k
		}

	case models.AccountTypeCreditCard:
		// Credit cards: negative balance = amount owed
		// Start with partial utilization
		switch segment {
		case models.SegmentPrivate:
			minBalance, maxBalance = -5000000, 0 // -$50k to $0 owed
		case models.SegmentPremium:
			minBalance, maxBalance = -2000000, 0 // -$20k to $0 owed
		default:
			minBalance, maxBalance = -500000, 0 // -$5k to $0 owed
		}

	case models.AccountTypeLoan, models.AccountTypeMortgage:
		// Loans: negative balance = amount owed
		switch accountType {
		case models.AccountTypeMortgage:
			minBalance, maxBalance = -50000000, -10000000 // $100k - $500k owed
		default:
			minBalance, maxBalance = -2500000, -500000 // $5k - $25k owed
		}

	case models.AccountTypeInvestment:
		switch segment {
		case models.SegmentPrivate:
			minBalance, maxBalance = 50000000, 500000000 // $500k - $5M
		case models.SegmentPremium:
			minBalance, maxBalance = 10000000, 100000000 // $100k - $1M
		default:
			minBalance, maxBalance = 1000000, 10000000 // $10k - $100k
		}

	case models.AccountTypeBusiness:
		minBalance, maxBalance = 1000000, 50000000 // $10k - $500k

	case models.AccountTypeMerchant:
		minBalance, maxBalance = 500000, 5000000 // $5k - $50k

	case models.AccountTypePayroll:
		// Payroll accounts have large balances for salary disbursement
		minBalance, maxBalance = 10000000, 500000000 // $100k - $5M

	default:
		minBalance, maxBalance = 100000, 1000000 // $1k - $10k
	}

	return g.rng.Int64Range(minBalance, maxBalance)
}

// calculateLimits determines credit/overdraft limits
func (g *AccountGenerator) calculateLimits(accountType models.AccountType, segment models.CustomerSegment, currency models.Currency) (creditLimit, overdraftLimit int64) {
	switch accountType {
	case models.AccountTypeCreditCard:
		switch segment {
		case models.SegmentPrivate:
			creditLimit = g.rng.Int64Range(10000000, 50000000) // $100k - $500k
		case models.SegmentPremium:
			creditLimit = g.rng.Int64Range(2500000, 10000000) // $25k - $100k
		default:
			creditLimit = g.rng.Int64Range(500000, 2500000) // $5k - $25k
		}

	case models.AccountTypeChecking:
		switch segment {
		case models.SegmentPrivate:
			overdraftLimit = g.rng.Int64Range(500000, 2500000) // $5k - $25k
		case models.SegmentPremium:
			overdraftLimit = g.rng.Int64Range(100000, 500000) // $1k - $5k
		case models.SegmentCorporate, models.SegmentBusiness:
			overdraftLimit = g.rng.Int64Range(1000000, 10000000) // $10k - $100k
		default:
			overdraftLimit = g.rng.Int64Range(50000, 100000) // $500 - $1k
		}
	}

	return
}

// calculateDailyLimits determines daily transaction limits
func (g *AccountGenerator) calculateDailyLimits(accountType models.AccountType, segment models.CustomerSegment, currency models.Currency) (withdrawLimit, transferLimit int64) {
	switch segment {
	case models.SegmentPrivate:
		withdrawLimit = g.rng.Int64Range(500000, 2000000) // $5k - $20k
		transferLimit = g.rng.Int64Range(10000000, 50000000) // $100k - $500k
	case models.SegmentPremium:
		withdrawLimit = g.rng.Int64Range(200000, 500000) // $2k - $5k
		transferLimit = g.rng.Int64Range(5000000, 10000000) // $50k - $100k
	case models.SegmentCorporate:
		withdrawLimit = g.rng.Int64Range(1000000, 5000000) // $10k - $50k
		transferLimit = g.rng.Int64Range(50000000, 200000000) // $500k - $2M
	case models.SegmentBusiness:
		withdrawLimit = g.rng.Int64Range(500000, 2000000) // $5k - $20k
		transferLimit = g.rng.Int64Range(10000000, 50000000) // $100k - $500k
	default:
		withdrawLimit = g.rng.Int64Range(50000, 100000) // $500 - $1k
		transferLimit = g.rng.Int64Range(500000, 2000000) // $5k - $20k
	}

	// Business account types get higher limits
	if accountType == models.AccountTypeBusiness || accountType == models.AccountTypePayroll || accountType == models.AccountTypeMerchant {
		transferLimit *= 2
	}

	return
}

// calculateInterestRate determines interest rate in basis points
func (g *AccountGenerator) calculateInterestRate(accountType models.AccountType) int {
	switch accountType {
	case models.AccountTypeSavings:
		return g.rng.IntRange(100, 500) // 1.00% - 5.00%
	case models.AccountTypeChecking:
		return g.rng.IntRange(0, 50) // 0.00% - 0.50%
	case models.AccountTypeCreditCard:
		return g.rng.IntRange(1500, 2500) // 15% - 25%
	case models.AccountTypeLoan:
		return g.rng.IntRange(500, 1200) // 5% - 12%
	case models.AccountTypeMortgage:
		return g.rng.IntRange(300, 700) // 3% - 7%
	case models.AccountTypeInvestment:
		return 0 // No fixed interest
	default:
		return g.rng.IntRange(50, 200) // 0.50% - 2.00%
	}
}

// pickBranch selects a branch for the account
func (g *AccountGenerator) pickBranch(countryCode string) int64 {
	if len(g.config.Branches) == 0 {
		return 1
	}

	// Prefer branches in same country
	sameCntry := make([]int64, 0)
	for _, b := range g.config.Branches {
		if b.Country.Code == countryCode {
			sameCntry = append(sameCntry, b.Branch.ID)
		}
	}

	if len(sameCntry) > 0 {
		return sameCntry[g.rng.IntN(len(sameCntry))]
	}

	return g.config.Branches[g.rng.IntN(len(g.config.Branches))].Branch.ID
}

// generateOpenedAt creates an account opening date
func (g *AccountGenerator) generateOpenedAt(customerCreatedAt time.Time) time.Time {
	// Account opened within 30 days of customer creation
	daysAfter := g.rng.IntRange(0, 30)
	return customerCreatedAt.Add(time.Duration(daysAfter) * 24 * time.Hour)
}

// WriteAccountsCSV writes accounts to a CSV file (or .csv.xz if compress=true)
func WriteAccountsCSV(accounts []GeneratedAccount, outputDir string, compress bool) error {
	return writeAccountsCSVInternal(accounts, outputDir, compress, false)
}

// WriteAccountsCSVWithProgress writes accounts with progress reporting
func WriteAccountsCSVWithProgress(accounts []GeneratedAccount, outputDir string, compress bool) error {
	return writeAccountsCSVInternal(accounts, outputDir, compress, true)
}

func writeAccountsCSVInternal(accounts []GeneratedAccount, outputDir string, compress, showProgress bool) error {
	headers := []string{
		"id", "account_number", "customer_id", "type", "status", "currency",
		"balance", "credit_limit", "overdraft_limit",
		"daily_withdraw_limit", "daily_transfer_limit", "interest_rate",
		"branch_id", "opened_at", "closed_at", "updated_at",
	}

	writer, err := NewCSVWriter(CSVWriterConfig{
		OutputDir: outputDir,
		Filename:  "accounts",
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
			Total: int64(len(accounts)),
			Label: "  Accounts",
		})
	}

	for i, ga := range accounts {
		a := ga.Account
		row := []string{
			FormatInt64(a.ID),
			a.AccountNumber,
			FormatInt64(a.CustomerID),
			string(a.Type),
			string(a.Status),
			string(a.Currency),
			FormatInt64(a.Balance),
			FormatInt64(a.CreditLimit),
			FormatInt64(a.OverdraftLimit),
			FormatInt64(a.DailyWithdrawLimit),
			FormatInt64(a.DailyTransferLimit),
			FormatInt(a.InterestRate),
			FormatInt64(a.BranchID),
			FormatTime(a.OpenedAt),
			FormatTimePtr(a.ClosedAt),
			FormatTime(a.UpdatedAt),
		}
		if err := writer.WriteRow(row); err != nil {
			return err
		}

		if progress != nil && (i+1)%100 == 0 {
			progress.Set(int64(i + 1))
		}
	}

	if progress != nil {
		progress.Set(int64(len(accounts)))
		progress.Finish()
	}

	return writer.Close()
}
