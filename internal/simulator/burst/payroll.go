package burst

import (
	"sync"
	"time"
)

// PayrollBurst implements end-of-month payroll surge simulation.
// On payroll days (typically 25th-28th of each month), there's a massive
// spike in transactions as employers process salary payments and employees
// immediately start making transactions.
type PayrollBurst struct {
	config BurstConfig
	mu     sync.RWMutex

	// Payroll processing windows (days of month)
	payrollDays []int

	// Track which timezones have already triggered this pay period
	triggeredThisMonth map[string]time.Time

	// Time parsing cache
	locationCache map[string]*time.Location
	cacheMu       sync.RWMutex
}

// NewPayrollBurst creates a new payroll burst provider
func NewPayrollBurst(cfg BurstConfig) *PayrollBurst {
	return &PayrollBurst{
		config: cfg,
		// Most common payroll days globally
		payrollDays:        []int{25, 26, 27, 28, 29, 30, 31},
		triggeredThisMonth: make(map[string]time.Time),
		locationCache:      make(map[string]*time.Location),
	}
}

// Type implements BurstProvider
func (pb *PayrollBurst) Type() BurstType {
	return BurstTypePayroll
}

// Configure implements BurstProvider
func (pb *PayrollBurst) Configure(cfg BurstConfig) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.config = cfg
}

// SetPayrollDays allows customization of which days trigger payroll bursts
func (pb *PayrollBurst) SetPayrollDays(days []int) {
	pb.mu.Lock()
	defer pb.mu.Unlock()
	pb.payrollDays = days
}

// CheckBurst implements BurstProvider
// Returns a burst event if it's a payroll day and processing time
func (pb *PayrollBurst) CheckBurst(timezone string) *BurstEvent {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	if !pb.config.Enabled {
		return nil
	}

	// Get local time in the timezone
	localTime := pb.getLocalTime(timezone)
	if localTime.IsZero() {
		return nil
	}

	day := localTime.Day()
	hour := localTime.Hour()

	// Check if it's a payroll day
	isPayrollDay := false
	for _, pd := range pb.payrollDays {
		if day == pd {
			isPayrollDay = true
			break
		}
	}

	if !isPayrollDay {
		return nil
	}

	// Payroll processing typically happens in the morning (9-10 AM)
	// When the burst triggers at start of business hours
	if hour != 9 {
		return nil
	}

	// Check if we already triggered this month for this timezone
	thisMonth := time.Date(localTime.Year(), localTime.Month(), 1, 0, 0, 0, 0, localTime.Location())
	if lastTriggered, ok := pb.triggeredThisMonth[timezone]; ok {
		lastMonth := time.Date(lastTriggered.Year(), lastTriggered.Month(), 1, 0, 0, 0, 0, lastTriggered.Location())
		if lastMonth.Equal(thisMonth) {
			return nil // Already triggered this month
		}
	}

	// Mark as triggered for this month
	pb.triggeredThisMonth[timezone] = localTime

	// Clean up old entries
	pb.cleanupOldTriggers()

	// Payroll burst is significant - it represents batch processing of
	// thousands of salary payments followed by increased consumer activity
	duration := pb.config.Duration
	if duration == 0 {
		// Default: 8 hours for the main burst (all-day elevated activity)
		duration = 8 * time.Hour
	}

	// Payroll creates a massive spike:
	// - Batch salary credits (high write volume)
	// - Immediate balance checks (employees checking if paid)
	// - Immediate bill payments (people paying bills right after payday)
	// - ATM withdrawals (people getting cash for the month)
	extraSessions := int(float64(50) * (pb.config.Multiplier - 1))
	if extraSessions < 0 {
		extraSessions = 0
	}

	now := time.Now()
	return &BurstEvent{
		Type:            BurstTypePayroll,
		StartTime:       now,
		EndTime:         now.Add(duration),
		Multiplier:      pb.config.Multiplier,
		Timezone:        timezone,
		SessionIncrease: extraSessions,
	}
}

// getLocalTime returns the current time in the specified timezone
func (pb *PayrollBurst) getLocalTime(timezone string) time.Time {
	pb.cacheMu.RLock()
	loc, ok := pb.locationCache[timezone]
	pb.cacheMu.RUnlock()

	if !ok {
		var err error
		loc, err = time.LoadLocation(timezone)
		if err != nil {
			return time.Time{}
		}

		pb.cacheMu.Lock()
		pb.locationCache[timezone] = loc
		pb.cacheMu.Unlock()
	}

	return time.Now().In(loc)
}

// cleanupOldTriggers removes trigger records older than 60 days
func (pb *PayrollBurst) cleanupOldTriggers() {
	cutoff := time.Now().AddDate(0, -2, 0)
	for tz, triggered := range pb.triggeredThisMonth {
		if triggered.Before(cutoff) {
			delete(pb.triggeredThisMonth, tz)
		}
	}
}

// IsPayrollPeriod returns true if the current day is a payroll day
func (pb *PayrollBurst) IsPayrollPeriod(timezone string) bool {
	pb.mu.RLock()
	defer pb.mu.RUnlock()

	localTime := pb.getLocalTime(timezone)
	if localTime.IsZero() {
		return false
	}

	day := localTime.Day()
	for _, pd := range pb.payrollDays {
		if day == pd {
			return true
		}
	}
	return false
}

// GetPayrollDays returns the configured payroll days
func (pb *PayrollBurst) GetPayrollDays() []int {
	pb.mu.RLock()
	defer pb.mu.RUnlock()
	days := make([]int, len(pb.payrollDays))
	copy(days, pb.payrollDays)
	return days
}
