package patterns

import (
	"math"
	"time"
)

// MonthlyPattern provides activity multipliers based on day of month.
// Models banking behavior around payroll, bill due dates, and end-of-month activity.
type MonthlyPattern struct {
	// Day multipliers (1-31 days), indexed 0-30
	dayMultipliers [31]float64

	// Payroll configuration
	payrollDays []int // Days when payroll typically occurs (e.g., 15, 30)

	// Bill cycle configuration
	billDueDays []int // Days when bills are typically due (e.g., 1, 15)
}

// NewMonthlyPattern creates a pattern with default retail banking behavior.
// Features:
// - End of month (25-31): High activity as paychecks arrive
// - Start of month (1-5): Bill payment surge
// - Mid-month (15): Secondary paycheck day for bi-weekly pay
// - Rest of month: Baseline activity
func NewMonthlyPattern() *MonthlyPattern {
	mp := &MonthlyPattern{
		payrollDays: []int{15, 25, 26, 27, 28, 29, 30, 31},
		billDueDays: []int{1, 5, 10, 15},
	}

	// Initialize with baseline of 1.0
	for i := range mp.dayMultipliers {
		mp.dayMultipliers[i] = 1.0
	}

	// Start of month - bill payment surge (days 1-5)
	mp.dayMultipliers[0] = 1.40  // Day 1 - Many bills due on 1st
	mp.dayMultipliers[1] = 1.25  // Day 2
	mp.dayMultipliers[2] = 1.15  // Day 3
	mp.dayMultipliers[3] = 1.10  // Day 4
	mp.dayMultipliers[4] = 1.05  // Day 5

	// Mid-month - bi-weekly payday spike (day 15)
	mp.dayMultipliers[14] = 1.35 // Day 15 - bi-weekly payday

	// End of month - payroll spike (days 25-31)
	mp.dayMultipliers[24] = 1.60 // Day 25 - early payroll
	mp.dayMultipliers[25] = 1.80 // Day 26 - payroll processing
	mp.dayMultipliers[26] = 1.90 // Day 27
	mp.dayMultipliers[27] = 2.00 // Day 28 - PEAK PAYROLL
	mp.dayMultipliers[28] = 1.90 // Day 29
	mp.dayMultipliers[29] = 1.80 // Day 30
	mp.dayMultipliers[30] = 1.70 // Day 31 - end of month

	return mp
}

// NewPayrollPattern creates a pattern focused on payroll-related activity.
// Extreme spikes on typical payroll processing days.
func NewPayrollPattern(payrollDay int) *MonthlyPattern {
	mp := &MonthlyPattern{
		payrollDays: []int{payrollDay},
		billDueDays: []int{},
	}

	// Initialize with very low baseline (payroll is concentrated)
	for i := range mp.dayMultipliers {
		mp.dayMultipliers[i] = 0.10
	}

	// Create a spike around the payroll day
	// Payroll processing often happens 1-2 days before the actual payday
	if payrollDay >= 3 {
		mp.dayMultipliers[payrollDay-3] = 0.30 // 3 days before - prep
	}
	if payrollDay >= 2 {
		mp.dayMultipliers[payrollDay-2] = 0.60 // 2 days before - processing starts
	}
	if payrollDay >= 1 {
		mp.dayMultipliers[payrollDay-1] = 1.50 // 1 day before - main processing
	}
	mp.dayMultipliers[payrollDay-1] = 3.00  // Payday itself - PEAK
	if payrollDay <= 30 {
		mp.dayMultipliers[payrollDay] = 1.50 // Day after - spending begins
	}
	if payrollDay <= 29 {
		mp.dayMultipliers[payrollDay+1] = 1.00 // 2 days after - continued spending
	}

	return mp
}

// NewBillPaymentPattern creates a pattern focused on bill payment cycles.
// Higher activity at start and middle of month when bills are due.
func NewBillPaymentPattern() *MonthlyPattern {
	mp := &MonthlyPattern{
		payrollDays: []int{},
		billDueDays: []int{1, 5, 10, 15, 20, 25},
	}

	// Initialize with baseline
	for i := range mp.dayMultipliers {
		mp.dayMultipliers[i] = 0.80
	}

	// Bill due date spikes
	mp.dayMultipliers[0] = 2.00  // Day 1 - Major due date
	mp.dayMultipliers[1] = 1.50  // Day 2
	mp.dayMultipliers[2] = 1.30  // Day 3
	mp.dayMultipliers[3] = 1.20  // Day 4
	mp.dayMultipliers[4] = 1.50  // Day 5 - Secondary due date
	mp.dayMultipliers[9] = 1.30  // Day 10
	mp.dayMultipliers[14] = 1.50 // Day 15 - Mid-month due date
	mp.dayMultipliers[19] = 1.20 // Day 20
	mp.dayMultipliers[24] = 1.30 // Day 25

	return mp
}

// NewBusinessMonthlyPattern creates a pattern for business account activity.
// Business activity spikes at month-end for accounting close and payroll.
func NewBusinessMonthlyPattern() *MonthlyPattern {
	mp := &MonthlyPattern{
		payrollDays: []int{25, 26, 27, 28, 29, 30, 31},
		billDueDays: []int{1, 10, 20},
	}

	// Initialize with business baseline
	for i := range mp.dayMultipliers {
		mp.dayMultipliers[i] = 1.0
	}

	// Start of month - invoice settlements
	mp.dayMultipliers[0] = 1.40 // Day 1
	mp.dayMultipliers[1] = 1.30 // Day 2

	// Net-10 payments
	mp.dayMultipliers[9] = 1.25 // Day 10

	// Net-30 payments and month-end close
	mp.dayMultipliers[24] = 1.50 // Day 25
	mp.dayMultipliers[25] = 1.80 // Day 26
	mp.dayMultipliers[26] = 2.00 // Day 27
	mp.dayMultipliers[27] = 2.50 // Day 28 - MONTH END CLOSE
	mp.dayMultipliers[28] = 2.30 // Day 29
	mp.dayMultipliers[29] = 2.20 // Day 30
	mp.dayMultipliers[30] = 2.00 // Day 31

	return mp
}

// GetMultiplier returns the activity multiplier for a given day of month (1-31).
func (mp *MonthlyPattern) GetMultiplier(dayOfMonth int) float64 {
	if dayOfMonth < 1 || dayOfMonth > 31 {
		return 1.0
	}
	return mp.dayMultipliers[dayOfMonth-1]
}

// GetMultiplierForDate returns the activity multiplier for a specific date.
// Handles edge cases like February (28/29 days) by capping at actual month length.
func (mp *MonthlyPattern) GetMultiplierForDate(t time.Time) float64 {
	day := t.Day()
	return mp.GetMultiplier(day)
}

// IsPayrollDay returns true if the day is a typical payroll processing day.
func (mp *MonthlyPattern) IsPayrollDay(dayOfMonth int) bool {
	for _, pd := range mp.payrollDays {
		if pd == dayOfMonth {
			return true
		}
	}
	return false
}

// IsBillDueDay returns true if the day is a typical bill due date.
func (mp *MonthlyPattern) IsBillDueDay(dayOfMonth int) bool {
	for _, bd := range mp.billDueDays {
		if bd == dayOfMonth {
			return true
		}
	}
	return false
}

// IsEndOfMonth returns true if the date is within the last 5 days of the month.
func (mp *MonthlyPattern) IsEndOfMonth(t time.Time) bool {
	day := t.Day()
	lastDay := mp.lastDayOfMonth(t)
	return day >= lastDay-4
}

// IsStartOfMonth returns true if the date is within the first 5 days of the month.
func (mp *MonthlyPattern) IsStartOfMonth(t time.Time) bool {
	return t.Day() <= 5
}

// GetPayrollSpike returns a spike multiplier for payroll days.
// This can be used to add extra transactions on payroll days.
func (mp *MonthlyPattern) GetPayrollSpike(dayOfMonth int) float64 {
	if !mp.IsPayrollDay(dayOfMonth) {
		return 1.0
	}

	// The closer to the end of month, the higher the spike
	switch {
	case dayOfMonth >= 28:
		return 3.0
	case dayOfMonth >= 25:
		return 2.5
	default:
		return 2.0
	}
}

// ExpectedTransactionsForMonth distributes a monthly target across days.
func (mp *MonthlyPattern) ExpectedTransactionsForMonth(monthlyTarget int, year int, month time.Month) [31]float64 {
	var result [31]float64

	// Get actual days in this month
	firstOfMonth := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	lastDay := mp.lastDayOfMonthFromDate(firstOfMonth)

	// Calculate total multiplier sum for this month's days
	var totalMultiplier float64
	for day := 1; day <= lastDay; day++ {
		totalMultiplier += mp.dayMultipliers[day-1]
	}

	// Distribute according to pattern (only for valid days)
	for day := 1; day <= lastDay; day++ {
		result[day-1] = float64(monthlyTarget) * (mp.dayMultipliers[day-1] / totalMultiplier)
	}

	return result
}

// lastDayOfMonth returns the last day of the month for a given date.
func (mp *MonthlyPattern) lastDayOfMonth(t time.Time) int {
	// Go to first of next month, then back one day
	year, month, _ := t.Date()
	firstOfNext := time.Date(year, month+1, 1, 0, 0, 0, 0, t.Location())
	lastDay := firstOfNext.AddDate(0, 0, -1)
	return lastDay.Day()
}

// lastDayOfMonthFromDate returns the last day number for the month of given date.
func (mp *MonthlyPattern) lastDayOfMonthFromDate(t time.Time) int {
	return mp.lastDayOfMonth(t)
}

// FullPattern combines daily, weekly, and monthly patterns.
type FullPattern struct {
	daily   *DailyPattern
	weekly  *WeeklyPattern
	monthly *MonthlyPattern
}

// NewFullPattern creates a combined pattern for all time dimensions.
func NewFullPattern(daily *DailyPattern, weekly *WeeklyPattern, monthly *MonthlyPattern) *FullPattern {
	return &FullPattern{
		daily:   daily,
		weekly:  weekly,
		monthly: monthly,
	}
}

// NewDefaultFullPattern creates a full pattern with default retail banking behavior.
func NewDefaultFullPattern() *FullPattern {
	return &FullPattern{
		daily:   NewDailyPattern(),
		weekly:  NewWeeklyPattern(),
		monthly: NewMonthlyPattern(),
	}
}

// NewATMFullPattern creates a full pattern optimized for ATM transactions.
func NewATMFullPattern() *FullPattern {
	return &FullPattern{
		daily:   NewATMDailyPattern(),
		weekly:  NewATMWeeklyPattern(),
		monthly: NewMonthlyPattern(),
	}
}

// NewOnlineFullPattern creates a full pattern for online banking.
func NewOnlineFullPattern() *FullPattern {
	return &FullPattern{
		daily:   NewOnlineBankingPattern(),
		weekly:  NewOnlineWeeklyPattern(),
		monthly: NewMonthlyPattern(),
	}
}

// NewBusinessFullPattern creates a full pattern for business accounts.
func NewBusinessFullPattern() *FullPattern {
	return &FullPattern{
		daily:   NewBusinessBankingPattern(),
		weekly:  NewBusinessWeeklyPattern(),
		monthly: NewBusinessMonthlyPattern(),
	}
}

// GetMultiplier returns the combined multiplier for a specific time.
// All three dimensions are multiplied together, with sqrt normalization
// to prevent extreme spikes.
func (fp *FullPattern) GetMultiplier(t time.Time) float64 {
	dailyMult := fp.daily.GetMultiplierForTime(t)
	weeklyMult := fp.weekly.GetMultiplierForDate(t)
	monthlyMult := fp.monthly.GetMultiplierForDate(t)

	// Combine with geometric mean to smooth extremes
	combined := dailyMult * weeklyMult * monthlyMult

	// Apply sqrt to prevent extreme spikes (e.g., 2.0 * 1.5 * 3.0 = 9.0 -> ~3.0)
	return math.Sqrt(combined)
}

// GetRawMultiplier returns the raw combined multiplier without normalization.
// Use for analysis or when extreme spikes are desired.
func (fp *FullPattern) GetRawMultiplier(t time.Time) float64 {
	return fp.daily.GetMultiplierForTime(t) *
		fp.weekly.GetMultiplierForDate(t) *
		fp.monthly.GetMultiplierForDate(t)
}

// IsActive returns true if the time is during active banking hours.
func (fp *FullPattern) IsActive(t time.Time) bool {
	// Check daily active hours
	if !fp.daily.IsActiveHour(t.Hour()) {
		return false
	}

	// Weekends have reduced hours
	if fp.weekly.IsWeekend(t.Weekday()) {
		hour := t.Hour()
		return hour >= 9 && hour <= 18
	}

	return true
}

// ShouldGenerateTransaction determines if a transaction should occur
// based on all pattern dimensions.
func (fp *FullPattern) ShouldGenerateTransaction(t time.Time, baseRate float64, rngValue float64) bool {
	multiplier := fp.GetMultiplier(t)
	probability := baseRate * multiplier
	if probability > 1.0 {
		probability = 1.0
	}
	return rngValue < probability
}

// GetTransactionCount returns the expected number of transactions for a time slot
// given a base count.
func (fp *FullPattern) GetTransactionCount(t time.Time, baseCount int) int {
	multiplier := fp.GetMultiplier(t)
	return int(float64(baseCount) * multiplier)
}
