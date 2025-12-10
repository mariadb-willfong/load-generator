package models

import (
	"time"
)

// BeneficiaryType represents the type of external payee
type BeneficiaryType string

const (
	BeneficiaryTypeIndividual BeneficiaryType = "individual"
	BeneficiaryTypeBusiness   BeneficiaryType = "business"
	BeneficiaryTypeUtility    BeneficiaryType = "utility"
	BeneficiaryTypeGovernment BeneficiaryType = "government"
)

// BeneficiaryStatus represents the verification status
type BeneficiaryStatus string

const (
	BeneficiaryStatusPending  BeneficiaryStatus = "pending"
	BeneficiaryStatusVerified BeneficiaryStatus = "verified"
	BeneficiaryStatusBlocked  BeneficiaryStatus = "blocked"
)

// Beneficiary represents an external payee that a customer can send money to
type Beneficiary struct {
	// Primary identifier
	ID int64 `db:"id" json:"id"`

	// Owner - the customer who added this beneficiary
	CustomerID int64 `db:"customer_id" json:"customer_id"`

	// Beneficiary details
	Nickname string          `db:"nickname" json:"nickname"` // User-friendly name
	Name     string          `db:"name" json:"name"`         // Legal/full name
	Type     BeneficiaryType `db:"type" json:"type"`
	Status   BeneficiaryStatus `db:"status" json:"status"`

	// External bank details
	BankName      string `db:"bank_name" json:"bank_name"`
	BankCode      string `db:"bank_code" json:"bank_code"`           // SWIFT/BIC code
	RoutingNumber string `db:"routing_number" json:"routing_number"` // ABA routing (US)
	AccountNumber string `db:"account_number" json:"account_number"`
	IBAN          string `db:"iban" json:"iban"` // International Bank Account Number

	// Address (for wire transfers)
	AddressLine1 string `db:"address_line1" json:"address_line1"`
	AddressLine2 string `db:"address_line2" json:"address_line2"`
	City         string `db:"city" json:"city"`
	State        string `db:"state" json:"state"`
	PostalCode   string `db:"postal_code" json:"postal_code"`
	Country      string `db:"country" json:"country"`

	// Payment details
	Currency      Currency `db:"currency" json:"currency"`
	PaymentMethod string   `db:"payment_method" json:"payment_method"` // ach, wire, etc.

	// For bill payments
	AccountReference string `db:"account_reference" json:"account_reference"` // Customer's account # with the biller

	// Usage tracking
	LastUsedAt    *time.Time `db:"last_used_at" json:"last_used_at"`
	TransferCount int        `db:"transfer_count" json:"transfer_count"`

	// Metadata
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// IsDomestic returns true if the beneficiary is in the same country
func (b *Beneficiary) IsDomestic(customerCountry string) bool {
	return b.Country == customerCountry
}

// IsVerified returns true if the beneficiary has been verified
func (b *Beneficiary) IsVerified() bool {
	return b.Status == BeneficiaryStatusVerified
}
