package models

import (
	"time"
)

// TransactionType represents the type of financial transaction
type TransactionType string

const (
	// Credit transactions (money coming in)
	TxTypeDeposit         TransactionType = "deposit"
	TxTypeSalary          TransactionType = "salary"
	TxTypeTransferIn      TransactionType = "transfer_in"
	TxTypeInterestCredit  TransactionType = "interest_credit"
	TxTypeRefund          TransactionType = "refund"
	TxTypeCashback        TransactionType = "cashback"

	// Debit transactions (money going out)
	TxTypeWithdrawal      TransactionType = "withdrawal"
	TxTypePurchase        TransactionType = "purchase"
	TxTypeTransferOut     TransactionType = "transfer_out"
	TxTypeBillPayment     TransactionType = "bill_payment"
	TxTypeInterestDebit   TransactionType = "interest_debit"
	TxTypeFee             TransactionType = "fee"
	TxTypeLoanPayment     TransactionType = "loan_payment"

	// Payroll (corporate accounts)
	TxTypePayrollBatch    TransactionType = "payroll_batch"
)

// TransactionStatus represents the state of a transaction
type TransactionStatus string

const (
	TxStatusPending   TransactionStatus = "pending"
	TxStatusCompleted TransactionStatus = "completed"
	TxStatusFailed    TransactionStatus = "failed"
	TxStatusReversed  TransactionStatus = "reversed"
	TxStatusDeclined  TransactionStatus = "declined"
)

// TransactionChannel represents how the transaction was initiated
type TransactionChannel string

const (
	ChannelOnline   TransactionChannel = "online"   // Web/mobile banking
	ChannelATM      TransactionChannel = "atm"      // ATM transaction
	ChannelBranch   TransactionChannel = "branch"   // In-person at branch
	ChannelPOS      TransactionChannel = "pos"      // Point of sale (card swipe)
	ChannelACH      TransactionChannel = "ach"      // Automated clearing house
	ChannelWire     TransactionChannel = "wire"     // Wire transfer
	ChannelInternal TransactionChannel = "internal" // Internal bank operation
)

// Transaction represents a financial transaction on an account
type Transaction struct {
	// Primary identifier
	ID int64 `db:"id" json:"id"`

	// Reference number (human-readable, for statements)
	ReferenceNumber string `db:"reference_number" json:"reference_number"`

	// Account relationship - the primary account affected
	AccountID int64 `db:"account_id" json:"account_id"`

	// For transfers: the counterparty account (if internal to bank)
	CounterpartyAccountID *int64 `db:"counterparty_account_id" json:"counterparty_account_id"`

	// For external transfers: beneficiary reference
	BeneficiaryID *int64 `db:"beneficiary_id" json:"beneficiary_id"`

	// Transaction details
	Type    TransactionType   `db:"type" json:"type"`
	Status  TransactionStatus `db:"status" json:"status"`
	Channel TransactionChannel `db:"channel" json:"channel"`

	// Amount in cents - always positive, sign determined by type
	// Credit types increase balance, debit types decrease balance
	Amount   int64    `db:"amount" json:"amount"`
	Currency Currency `db:"currency" json:"currency"`

	// Balance after this transaction (running balance)
	BalanceAfter int64 `db:"balance_after" json:"balance_after"`

	// Description/memo visible on statements
	Description string `db:"description" json:"description"`

	// Additional metadata (JSON in database)
	// Could include: merchant name, category, location, etc.
	Metadata string `db:"metadata" json:"metadata"`

	// Location context
	BranchID *int64 `db:"branch_id" json:"branch_id"` // Branch where transaction occurred
	ATMID    *int64 `db:"atm_id" json:"atm_id"`       // ATM ID if ATM transaction

	// For double-entry bookkeeping: link related transactions
	// e.g., transfer creates two transactions with same linked_id
	LinkedTransactionID *int64 `db:"linked_transaction_id" json:"linked_transaction_id"`

	// Timing
	Timestamp   time.Time `db:"timestamp" json:"timestamp"`
	PostedAt    time.Time `db:"posted_at" json:"posted_at"`
	ValueDate   time.Time `db:"value_date" json:"value_date"` // Effective date for interest

	// Error info for failed/declined transactions
	FailureReason *string `db:"failure_reason" json:"failure_reason"`
}

// IsCredit returns true if this transaction adds money to the account
func (t *Transaction) IsCredit() bool {
	switch t.Type {
	case TxTypeDeposit, TxTypeSalary, TxTypeTransferIn,
		TxTypeInterestCredit, TxTypeRefund, TxTypeCashback:
		return true
	default:
		return false
	}
}

// IsDebit returns true if this transaction removes money from the account
func (t *Transaction) IsDebit() bool {
	return !t.IsCredit()
}

// SignedAmount returns the amount with appropriate sign for balance calculation
// Positive for credits, negative for debits
func (t *Transaction) SignedAmount() int64 {
	if t.IsCredit() {
		return t.Amount
	}
	return -t.Amount
}

// IsInternal returns true if this is an internal bank transfer
func (t *Transaction) IsInternal() bool {
	return t.CounterpartyAccountID != nil
}
