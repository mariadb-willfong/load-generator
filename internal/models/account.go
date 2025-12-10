package models

import (
	"time"
)

// AccountType represents the type of bank account
type AccountType string

const (
	AccountTypeChecking   AccountType = "checking"
	AccountTypeSavings    AccountType = "savings"
	AccountTypeCreditCard AccountType = "credit_card"
	AccountTypeLoan       AccountType = "loan"
	AccountTypeMortgage   AccountType = "mortgage"
	AccountTypeInvestment AccountType = "investment"
	AccountTypeBusiness   AccountType = "business"   // Business checking
	AccountTypeMerchant   AccountType = "merchant"   // For receiving payments
	AccountTypePayroll    AccountType = "payroll"    // Corporate payroll account
)

// AccountStatus represents the current status of an account
type AccountStatus string

const (
	AccountStatusActive    AccountStatus = "active"
	AccountStatusDormant   AccountStatus = "dormant"
	AccountStatusFrozen    AccountStatus = "frozen"
	AccountStatusClosed    AccountStatus = "closed"
	AccountStatusPending   AccountStatus = "pending"
)

// Currency represents ISO 4217 currency codes
type Currency string

const (
	CurrencyUSD Currency = "USD"
	CurrencyEUR Currency = "EUR"
	CurrencyGBP Currency = "GBP"
	CurrencyJPY Currency = "JPY"
	CurrencyCHF Currency = "CHF"
	CurrencyCAD Currency = "CAD"
	CurrencyAUD Currency = "AUD"
	CurrencyINR Currency = "INR"
	CurrencyCNY Currency = "CNY"
	CurrencySGD Currency = "SGD"
	CurrencyHKD Currency = "HKD"
	CurrencyBRL Currency = "BRL"
	CurrencyMXN Currency = "MXN"
)

// Account represents a bank account
type Account struct {
	// Primary identifier
	ID int64 `db:"id" json:"id"`

	// Account number (formatted like real bank accounts)
	AccountNumber string `db:"account_number" json:"account_number"`

	// Owner relationship
	CustomerID int64 `db:"customer_id" json:"customer_id"`

	// Account details
	Type     AccountType   `db:"type" json:"type"`
	Status   AccountStatus `db:"status" json:"status"`
	Currency Currency      `db:"currency" json:"currency"`

	// Balance - stored as cents (int64) for precision
	// For credit cards/loans, negative balance = amount owed
	Balance int64 `db:"balance" json:"balance"`

	// Credit/overdraft limits (in cents)
	CreditLimit    int64 `db:"credit_limit" json:"credit_limit"`       // For credit cards
	OverdraftLimit int64 `db:"overdraft_limit" json:"overdraft_limit"` // For checking

	// Daily transaction limits (in cents)
	DailyWithdrawLimit  int64 `db:"daily_withdraw_limit" json:"daily_withdraw_limit"`
	DailyTransferLimit  int64 `db:"daily_transfer_limit" json:"daily_transfer_limit"`

	// Interest rates (stored as basis points, e.g., 250 = 2.50%)
	InterestRate int `db:"interest_rate" json:"interest_rate"`

	// Branch association
	BranchID int64 `db:"branch_id" json:"branch_id"`

	// Metadata
	OpenedAt  time.Time  `db:"opened_at" json:"opened_at"`
	ClosedAt  *time.Time `db:"closed_at" json:"closed_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
}

// AvailableBalance returns the balance plus any credit/overdraft limits
func (a *Account) AvailableBalance() int64 {
	available := a.Balance

	if a.Type == AccountTypeCreditCard {
		// Credit card: available = limit - balance (balance is negative when owed)
		available = a.CreditLimit + a.Balance
	} else if a.Type == AccountTypeChecking {
		// Checking: include overdraft protection
		available = a.Balance + a.OverdraftLimit
	}

	return available
}

// CanWithdraw checks if the account can support a withdrawal of the given amount
func (a *Account) CanWithdraw(amount int64) bool {
	if a.Status != AccountStatusActive {
		return false
	}
	return a.AvailableBalance() >= amount
}

// IsLiability returns true if this is a credit product (credit card, loan, mortgage)
func (a *Account) IsLiability() bool {
	return a.Type == AccountTypeCreditCard ||
		a.Type == AccountTypeLoan ||
		a.Type == AccountTypeMortgage
}
