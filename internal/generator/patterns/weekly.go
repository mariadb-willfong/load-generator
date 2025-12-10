package patterns

import (
	"time"
)

// WeeklyPattern provides activity multipliers based on day of week.
// Models banking behavior differences between weekdays and weekends.
type WeeklyPattern struct {
	// Daily multipliers indexed by time.Weekday (0=Sunday, 6=Saturday)
	dailyMultipliers [7]float64
}

// NewWeeklyPattern creates a pattern with default retail banking behavior.
// Weekdays have higher activity than weekends, with Monday/Friday slightly higher.
func NewWeeklyPattern() *WeeklyPattern {
	wp := &WeeklyPattern{}

	wp.dailyMultipliers = [7]float64{
		0.40, // Sunday - lowest activity
		1.20, // Monday - high (catching up from weekend)
		1.00, // Tuesday - normal
		1.00, // Wednesday - normal
		1.00, // Thursday - normal
		1.30, // Friday - highest (payday activities, weekend prep)
		0.60, // Saturday - moderate (some personal banking)
	}

	return wp
}

// NewATMWeeklyPattern creates a pattern for ATM usage.
// ATM usage is more even across the week, with weekend evening spikes.
func NewATMWeeklyPattern() *WeeklyPattern {
	wp := &WeeklyPattern{}

	wp.dailyMultipliers = [7]float64{
		0.70, // Sunday - people need cash for weekend activities
		0.90, // Monday - lower (used cash over weekend)
		1.00, // Tuesday - normal
		1.10, // Wednesday - slight increase mid-week
		1.10, // Thursday - building to weekend
		1.30, // Friday - highest (weekend cash needs)
		0.90, // Saturday - shopping day
	}

	return wp
}

// NewOnlineWeeklyPattern creates a pattern for online banking.
// Online banking peaks on weekday evenings and weekend afternoons.
func NewOnlineWeeklyPattern() *WeeklyPattern {
	wp := &WeeklyPattern{}

	wp.dailyMultipliers = [7]float64{
		0.80, // Sunday - afternoon bill management
		1.00, // Monday - checking weekend transactions
		0.90, // Tuesday - normal
		0.90, // Wednesday - normal
		1.00, // Thursday - pre-payday checks
		1.10, // Friday - payday activity
		0.70, // Saturday - lower priority
	}

	return wp
}

// NewBusinessWeeklyPattern creates a pattern for business accounts.
// Business activity is heavily concentrated on weekdays with minimal weekends.
func NewBusinessWeeklyPattern() *WeeklyPattern {
	wp := &WeeklyPattern{}

	wp.dailyMultipliers = [7]float64{
		0.05, // Sunday - essentially closed
		1.20, // Monday - high (week start, catching up)
		1.10, // Tuesday - normal business day
		1.00, // Wednesday - normal
		1.10, // Thursday - pre-Friday preparations
		1.30, // Friday - week-end processing, payroll prep
		0.10, // Saturday - minimal weekend support
	}

	return wp
}

// NewCorporatePayrollPattern creates a pattern for payroll-related activity.
// Payroll processing is heavily weighted toward end of month and Fridays.
func NewCorporatePayrollPattern() *WeeklyPattern {
	wp := &WeeklyPattern{}

	wp.dailyMultipliers = [7]float64{
		0.00, // Sunday - no payroll processing
		1.00, // Monday
		0.90, // Tuesday
		0.90, // Wednesday
		1.20, // Thursday - pre-Friday prep
		1.50, // Friday - highest (many pay on Friday)
		0.00, // Saturday - no payroll processing
	}

	return wp
}

// GetMultiplier returns the activity multiplier for a given weekday.
func (wp *WeeklyPattern) GetMultiplier(weekday time.Weekday) float64 {
	return wp.dailyMultipliers[weekday]
}

// GetMultiplierForDate returns the activity multiplier for a specific date.
func (wp *WeeklyPattern) GetMultiplierForDate(t time.Time) float64 {
	return wp.dailyMultipliers[t.Weekday()]
}

// IsWeekend returns true if the weekday is Saturday or Sunday.
func (wp *WeeklyPattern) IsWeekend(weekday time.Weekday) bool {
	return weekday == time.Saturday || weekday == time.Sunday
}

// IsBusinessDay returns true if it's a weekday (Monday-Friday).
func (wp *WeeklyPattern) IsBusinessDay(weekday time.Weekday) bool {
	return !wp.IsWeekend(weekday)
}

// GetHighActivityDays returns weekdays with above-average activity (multiplier >= 1.0).
func (wp *WeeklyPattern) GetHighActivityDays() []time.Weekday {
	var result []time.Weekday
	for day := time.Sunday; day <= time.Saturday; day++ {
		if wp.dailyMultipliers[day] >= 1.0 {
			result = append(result, day)
		}
	}
	return result
}

// ShouldProcessOnDay determines if activity should occur on a given day
// based on the multiplier as a probability factor.
func (wp *WeeklyPattern) ShouldProcessOnDay(weekday time.Weekday, baseProb float64, rngValue float64) bool {
	multiplier := wp.dailyMultipliers[weekday]
	probability := baseProb * multiplier
	if probability > 1.0 {
		probability = 1.0
	}
	return rngValue < probability
}

// ExpectedDailyTransactions distributes a weekly target across days according to the pattern.
func (wp *WeeklyPattern) ExpectedDailyTransactions(weeklyTarget int) [7]float64 {
	var result [7]float64

	// Calculate total multiplier sum for normalization
	var totalMultiplier float64
	for _, m := range wp.dailyMultipliers {
		totalMultiplier += m
	}

	// Distribute according to pattern
	for day := 0; day < 7; day++ {
		result[day] = float64(weeklyTarget) * (wp.dailyMultipliers[day] / totalMultiplier)
	}

	return result
}

// CombinedPattern combines a WeeklyPattern with a DailyPattern to get
// a composite multiplier for any specific time.
type CombinedPattern struct {
	weekly *WeeklyPattern
	daily  *DailyPattern
}

// NewCombinedPattern creates a combined weekly+daily pattern.
func NewCombinedPattern(weekly *WeeklyPattern, daily *DailyPattern) *CombinedPattern {
	return &CombinedPattern{
		weekly: weekly,
		daily:  daily,
	}
}

// GetMultiplier returns the combined multiplier for a specific time.
// The weekly and daily multipliers are multiplied together.
func (cp *CombinedPattern) GetMultiplier(t time.Time) float64 {
	weeklyMult := cp.weekly.GetMultiplierForDate(t)
	dailyMult := cp.daily.GetMultiplierForTime(t)
	return weeklyMult * dailyMult
}

// IsActive returns true if the time is during active banking hours
// on a non-weekend day.
func (cp *CombinedPattern) IsActive(t time.Time) bool {
	// Check if it's an active hour (6 AM - 10 PM)
	if !cp.daily.IsActiveHour(t.Hour()) {
		return false
	}

	// Weekend activity is lower but not zero
	weekday := t.Weekday()
	if cp.weekly.IsWeekend(weekday) {
		// Weekends: only active 9 AM - 6 PM
		hour := t.Hour()
		return hour >= 9 && hour <= 18
	}

	return true
}

// AdjustedRate returns the base rate adjusted by both weekly and daily multipliers.
func (cp *CombinedPattern) AdjustedRate(t time.Time, baseRate float64) float64 {
	return baseRate * cp.GetMultiplier(t)
}
