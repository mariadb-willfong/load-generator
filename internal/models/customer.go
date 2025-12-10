package models

import (
	"time"
)

// CustomerSegment represents the customer's banking tier
type CustomerSegment string

const (
	SegmentRegular   CustomerSegment = "regular"
	SegmentPremium   CustomerSegment = "premium"
	SegmentPrivate   CustomerSegment = "private"    // High net worth
	SegmentBusiness  CustomerSegment = "business"   // Small business
	SegmentCorporate CustomerSegment = "corporate"  // Large enterprise
)

// CustomerStatus represents the customer's account status
type CustomerStatus string

const (
	CustomerStatusActive   CustomerStatus = "active"
	CustomerStatusInactive CustomerStatus = "inactive"
	CustomerStatusSuspended CustomerStatus = "suspended"
	CustomerStatusClosed   CustomerStatus = "closed"
)

// Customer represents a bank customer with all their personal information
type Customer struct {
	// Primary identifier
	ID int64 `db:"id" json:"id"`

	// Personal Information (PII)
	FirstName   string `db:"first_name" json:"first_name"`
	LastName    string `db:"last_name" json:"last_name"`
	Email       string `db:"email" json:"email"`
	Phone       string `db:"phone" json:"phone"`
	DateOfBirth time.Time `db:"date_of_birth" json:"date_of_birth"`

	// Address
	AddressLine1 string `db:"address_line1" json:"address_line1"`
	AddressLine2 string `db:"address_line2" json:"address_line2"`
	City         string `db:"city" json:"city"`
	State        string `db:"state" json:"state"`
	PostalCode   string `db:"postal_code" json:"postal_code"`
	Country      string `db:"country" json:"country"` // ISO 3166-1 alpha-2

	// Geographic/Timezone
	Timezone   string `db:"timezone" json:"timezone"` // IANA timezone (e.g., "America/New_York")
	HomeBranch int64  `db:"home_branch_id" json:"home_branch_id"`

	// Banking Profile
	Segment       CustomerSegment `db:"segment" json:"segment"`
	Status        CustomerStatus  `db:"status" json:"status"`
	ActivityScore float64         `db:"activity_score" json:"activity_score"` // 0.0-1.0, affects transaction frequency

	// Authentication (for online banking simulation)
	Username     string `db:"username" json:"username"`
	PasswordHash string `db:"password_hash" json:"password_hash"`
	PIN          string `db:"pin" json:"pin"` // For ATM simulation (hashed)

	// Metadata
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// IsBusinessCustomer returns true if this is a business/corporate customer
func (c *Customer) IsBusinessCustomer() bool {
	return c.Segment == SegmentBusiness || c.Segment == SegmentCorporate
}

// IsHighNetWorth returns true if this is a premium/private banking customer
func (c *Customer) IsHighNetWorth() bool {
	return c.Segment == SegmentPremium || c.Segment == SegmentPrivate
}
