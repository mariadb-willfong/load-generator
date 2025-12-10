package simulator

import (
	"time"

	"github.com/willfong/load-generator/internal/models"
	"github.com/willfong/load-generator/internal/utils"
)

// ActivityCalculator determines when and how actively a customer should interact
// with banking services. It combines multiple factors:
// - Timezone-based activity windows
// - Intraday patterns (morning rush, lunch peak)
// - Weekly patterns (weekday vs weekend)
// - Monthly patterns (payroll spikes, bill payment cycles)
// - Customer segment behavior
type ActivityCalculator struct {
	timezone *TimezoneManager

	// Configuration
	payrollDays     []int   // Days of month when payroll spikes occur (e.g., 25, 26, 27, 28)
	billPaymentDays []int   // Days when bill payments spike (e.g., 1, 2, 15)
	payrollBurst    float64 // Multiplier during payroll (e.g., 2.0 = double activity)
	lunchBurst      float64 // Multiplier during lunch hours

	// Session type distribution (configurable)
	atmSessionRatio      float64
	onlineSessionRatio   float64
	businessSessionRatio float64
}

// NewActivityCalculator creates a new activity calculator
func NewActivityCalculator(activeStart, activeEnd int) *ActivityCalculator {
	return &ActivityCalculator{
		timezone:             NewTimezoneManager(activeStart, activeEnd),
		payrollDays:          []int{25, 26, 27, 28}, // Last week of month
		billPaymentDays:      []int{1, 2, 15},       // Beginning and mid-month
		payrollBurst:         2.0,
		lunchBurst:           1.5,
		atmSessionRatio:      0.3, // Default 30% ATM
		onlineSessionRatio:   0.5, // Default 50% Online
		businessSessionRatio: 0.2, // Default 20% Business
	}
}

// SetSessionTypeRatios configures the session type distribution
func (ac *ActivityCalculator) SetSessionTypeRatios(atm, online, business float64) {
	ac.atmSessionRatio = atm
	ac.onlineSessionRatio = online
	ac.businessSessionRatio = business
}

// SetPayrollBurst configures the multiplier for payroll day activity
func (ac *ActivityCalculator) SetPayrollBurst(multiplier float64) {
	ac.payrollBurst = multiplier
}

// SetLunchBurst configures the multiplier for lunch hour activity
func (ac *ActivityCalculator) SetLunchBurst(multiplier float64) {
	ac.lunchBurst = multiplier
}

// GetTimezoneManager returns the underlying timezone manager
func (ac *ActivityCalculator) GetTimezoneManager() *TimezoneManager {
	return ac.timezone
}

// ShouldBeActive determines if a customer should be active right now.
// Uses probabilistic decision based on combined activity factors.
// Returns true if the customer should execute actions this cycle.
func (ac *ActivityCalculator) ShouldBeActive(customer *models.Customer, rng *utils.Random) bool {
	probability := ac.CalculateActivityProbability(customer)

	// Random check against probability
	return rng.Float64() < probability
}

// CalculateActivityProbability computes the overall probability (0.0-1.0)
// that the customer would be active at this moment.
func (ac *ActivityCalculator) CalculateActivityProbability(customer *models.Customer) float64 {
	// Start with timezone-based probability (includes hourly and weekday)
	baseProbability := ac.timezone.GetCombinedActivityProbability(customer.Timezone)

	// Apply monthly pattern modifiers
	monthlyMod := ac.getMonthlyModifier(customer.Timezone)

	// Apply customer segment modifier
	segmentMod := ac.getSegmentModifier(customer.Segment)

	// Apply customer's personal activity score (0.0 - 1.0)
	// Higher activity score = more likely to transact
	activityMod := 0.5 + (customer.ActivityScore * 0.5) // Normalize to 0.5-1.0 range

	// Combine all factors
	combined := baseProbability * monthlyMod * segmentMod * activityMod

	// Cap at 1.0
	if combined > 1.0 {
		combined = 1.0
	}

	return combined
}

// getMonthlyModifier returns a multiplier based on day of month
func (ac *ActivityCalculator) getMonthlyModifier(timezone string) float64 {
	dayOfMonth := ac.timezone.GetLocalTime(timezone).Day()
	modifier := 1.0

	// Check if it's a payroll day
	for _, day := range ac.payrollDays {
		if dayOfMonth == day {
			modifier *= ac.payrollBurst
			break
		}
	}

	// Check if it's a bill payment day (smaller boost)
	for _, day := range ac.billPaymentDays {
		if dayOfMonth == day {
			modifier *= 1.3 // 30% increase for bill payment days
			break
		}
	}

	return modifier
}

// getSegmentModifier returns activity multiplier based on customer segment
func (ac *ActivityCalculator) getSegmentModifier(segment models.CustomerSegment) float64 {
	switch segment {
	case models.SegmentCorporate:
		return 1.5 // Corporate accounts are very active
	case models.SegmentBusiness:
		return 1.3 // Business accounts are moderately more active
	case models.SegmentPrivate:
		return 1.2 // Private banking clients (high net worth)
	case models.SegmentPremium:
		return 1.1 // Premium customers slightly more active
	case models.SegmentRegular:
		return 1.0 // Baseline activity
	default:
		return 1.0
	}
}

// GetThinkTimeMultiplier returns a multiplier for think time based on
// current load conditions. During peak hours, people tend to be faster.
func (ac *ActivityCalculator) GetThinkTimeMultiplier(timezone string) float64 {
	hour := ac.timezone.GetLocalHour(timezone)

	// During peak hours, think times are shorter (people are rushed)
	switch {
	case hour >= 8 && hour <= 10: // Morning rush
		return 0.7 // 30% faster
	case hour >= 12 && hour <= 13: // Lunch
		return 0.8 // 20% faster
	case hour >= 16 && hour <= 17: // End of day rush
		return 0.7 // 30% faster
	default:
		return 1.0 // Normal pace
	}
}

// IsPayrollPeriod checks if the current time (in customer's timezone)
// falls within a payroll processing period
func (ac *ActivityCalculator) IsPayrollPeriod(timezone string) bool {
	dayOfMonth := ac.timezone.GetLocalTime(timezone).Day()
	for _, day := range ac.payrollDays {
		if dayOfMonth == day {
			return true
		}
	}
	return false
}

// IsLunchHour checks if the current time is during lunch hour
func (ac *ActivityCalculator) IsLunchHour(timezone string) bool {
	hour := ac.timezone.GetLocalHour(timezone)
	return hour >= 12 && hour < 14
}

// GetRecommendedSessionType suggests a session type based on time of day
// and customer segment. This provides more realistic session type distribution.
func (ac *ActivityCalculator) GetRecommendedSessionType(customer *models.Customer, rng *utils.Random) SessionType {
	hour := ac.timezone.GetLocalHour(customer.Timezone)
	isLunch := hour >= 12 && hour < 14

	// Business customers almost always do online banking, not ATM
	if customer.Segment == "business" || customer.Segment == "corporate" {
		return SessionTypeBusiness
	}

	// During lunch hours, ATM usage spikes for cash withdrawals
	if isLunch {
		r := rng.Float64()
		if r < 0.5 { // 50% chance of ATM during lunch
			return SessionTypeATM
		}
	}

	// Morning hours: More online banking as people check accounts
	if hour >= 8 && hour < 10 {
		r := rng.Float64()
		if r < 0.6 { // 60% online during morning
			return SessionTypeOnline
		}
	}

	// Default distribution
	return ac.pickDefaultSessionType(rng)
}

// pickDefaultSessionType chooses session type based on configured ratios
func (ac *ActivityCalculator) pickDefaultSessionType(rng *utils.Random) SessionType {
	r := rng.Float64()
	// Use cumulative probabilities based on configured ratios
	if r < ac.atmSessionRatio {
		return SessionTypeATM
	}
	if r < ac.atmSessionRatio+ac.onlineSessionRatio {
		return SessionTypeOnline
	}
	return SessionTypeBusiness
}

// ActivityDecision captures the result of an activity calculation
type ActivityDecision struct {
	ShouldExecute     bool          // Whether to execute this session
	Probability       float64       // The calculated probability
	ThinkTimeMultiplier float64     // Adjustment for think time
	IsPayrollPeriod   bool          // Payroll spike active
	IsLunchHour       bool          // Lunch burst active
	RecommendedType   SessionType   // Suggested session type
}

// MakeActivityDecision computes a comprehensive activity decision for a customer
func (ac *ActivityCalculator) MakeActivityDecision(customer *models.Customer, rng *utils.Random) ActivityDecision {
	probability := ac.CalculateActivityProbability(customer)

	return ActivityDecision{
		ShouldExecute:       rng.Float64() < probability,
		Probability:         probability,
		ThinkTimeMultiplier: ac.GetThinkTimeMultiplier(customer.Timezone),
		IsPayrollPeriod:     ac.IsPayrollPeriod(customer.Timezone),
		IsLunchHour:         ac.IsLunchHour(customer.Timezone),
		RecommendedType:     ac.GetRecommendedSessionType(customer, rng),
	}
}

// GlobalActivitySnapshot provides a point-in-time view of global activity
type GlobalActivitySnapshot struct {
	Timestamp       time.Time
	ActiveTimezones []string           // Timezones currently in business hours
	IsPayrollDay    bool               // Any timezone is in payroll period
	RegionalActivity map[string]float64 // Activity level per region
}

// GetGlobalActivitySnapshot returns current global activity state
func (ac *ActivityCalculator) GetGlobalActivitySnapshot() GlobalActivitySnapshot {
	now := time.Now()

	// Check major regions for activity
	regions := map[string]string{
		"Americas": "America/New_York",
		"Europe":   "Europe/London",
		"Asia":     "Asia/Tokyo",
		"Pacific":  "Australia/Sydney",
	}

	regionalActivity := make(map[string]float64)
	isPayrollDay := false

	for region, tz := range regions {
		regionalActivity[region] = ac.timezone.GetCombinedActivityProbability(tz)
		if ac.IsPayrollPeriod(tz) {
			isPayrollDay = true
		}
	}

	return GlobalActivitySnapshot{
		Timestamp:        now,
		ActiveTimezones:  ac.timezone.GetActiveTimezones(),
		IsPayrollDay:     isPayrollDay,
		RegionalActivity: regionalActivity,
	}
}
