package models

import (
	"time"
)

// BranchType represents the type of banking location
type BranchType string

const (
	BranchTypeFull       BranchType = "full"       // Full-service branch
	BranchTypeLimited    BranchType = "limited"    // Limited service
	BranchTypeATMOnly    BranchType = "atm_only"   // ATM location only
	BranchTypeHeadquarter BranchType = "hq"        // Headquarters
	BranchTypeRegional   BranchType = "regional"   // Regional office
)

// BranchStatus represents the operational status
type BranchStatus string

const (
	BranchStatusOpen       BranchStatus = "open"
	BranchStatusClosed     BranchStatus = "closed"
	BranchStatusRenovation BranchStatus = "renovation"
	BranchStatusRelocating BranchStatus = "relocating"
)

// Branch represents a bank branch or office location
type Branch struct {
	// Primary identifier
	ID int64 `db:"id" json:"id"`

	// Branch code (like sort code in UK, routing prefix)
	BranchCode string `db:"branch_code" json:"branch_code"`
	Name       string `db:"name" json:"name"`

	// Type and status
	Type   BranchType   `db:"type" json:"type"`
	Status BranchStatus `db:"status" json:"status"`

	// Location
	AddressLine1 string  `db:"address_line1" json:"address_line1"`
	AddressLine2 string  `db:"address_line2" json:"address_line2"`
	City         string  `db:"city" json:"city"`
	State        string  `db:"state" json:"state"`
	PostalCode   string  `db:"postal_code" json:"postal_code"`
	Country      string  `db:"country" json:"country"` // ISO 3166-1 alpha-2
	Latitude     float64 `db:"latitude" json:"latitude"`
	Longitude    float64 `db:"longitude" json:"longitude"`

	// Timezone for this branch (affects operating hours)
	Timezone string `db:"timezone" json:"timezone"`

	// Operating hours (stored as JSON or separate fields)
	// Format: "09:00-17:00" for each day
	MondayHours    string `db:"monday_hours" json:"monday_hours"`
	TuesdayHours   string `db:"tuesday_hours" json:"tuesday_hours"`
	WednesdayHours string `db:"wednesday_hours" json:"wednesday_hours"`
	ThursdayHours  string `db:"thursday_hours" json:"thursday_hours"`
	FridayHours    string `db:"friday_hours" json:"friday_hours"`
	SaturdayHours  string `db:"saturday_hours" json:"saturday_hours"`
	SundayHours    string `db:"sunday_hours" json:"sunday_hours"`

	// Contact
	Phone string `db:"phone" json:"phone"`
	Email string `db:"email" json:"email"`

	// Capacity/load modeling
	CustomerCapacity int `db:"customer_capacity" json:"customer_capacity"` // For activity distribution
	ATMCount         int `db:"atm_count" json:"atm_count"`

	// Metadata
	OpenedAt  time.Time  `db:"opened_at" json:"opened_at"`
	ClosedAt  *time.Time `db:"closed_at" json:"closed_at"`
	UpdatedAt time.Time  `db:"updated_at" json:"updated_at"`
}

// ATMStatus represents the operational status of an ATM
type ATMStatus string

const (
	ATMStatusOnline      ATMStatus = "online"
	ATMStatusOffline     ATMStatus = "offline"
	ATMStatusMaintenance ATMStatus = "maintenance"
	ATMStatusOutOfCash   ATMStatus = "out_of_cash"
)

// ATM represents an automated teller machine
type ATM struct {
	// Primary identifier
	ID int64 `db:"id" json:"id"`

	// ATM identifier (displayed on machine)
	ATMID string `db:"atm_id" json:"atm_id"`

	// Associated branch (if any)
	BranchID *int64 `db:"branch_id" json:"branch_id"`

	// Status
	Status ATMStatus `db:"status" json:"status"`

	// Location
	LocationName string  `db:"location_name" json:"location_name"` // e.g., "Main Street Mall"
	AddressLine1 string  `db:"address_line1" json:"address_line1"`
	City         string  `db:"city" json:"city"`
	State        string  `db:"state" json:"state"`
	PostalCode   string  `db:"postal_code" json:"postal_code"`
	Country      string  `db:"country" json:"country"`
	Latitude     float64 `db:"latitude" json:"latitude"`
	Longitude    float64 `db:"longitude" json:"longitude"`

	// Timezone
	Timezone string `db:"timezone" json:"timezone"`

	// Capabilities
	SupportsDeposit  bool `db:"supports_deposit" json:"supports_deposit"`
	SupportsTransfer bool `db:"supports_transfer" json:"supports_transfer"`
	Is24Hours        bool `db:"is_24_hours" json:"is_24_hours"`

	// For load modeling
	AvgDailyTransactions int `db:"avg_daily_transactions" json:"avg_daily_transactions"`

	// Metadata
	InstalledAt time.Time `db:"installed_at" json:"installed_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// IsOperational returns true if the ATM can process transactions
func (a *ATM) IsOperational() bool {
	return a.Status == ATMStatusOnline
}
