package simulator

import (
	"context"
	"sync"
	"time"

	"github.com/willfong/load-generator/internal/config"
	"github.com/willfong/load-generator/internal/database"
	"github.com/willfong/load-generator/internal/models"
	"github.com/willfong/load-generator/internal/utils"
)

// Scheduler handles timezone-aware session scheduling.
// It implements "follow the sun" traffic distribution by favoring customers
// in currently active timezones while respecting realistic activity patterns.
type Scheduler struct {
	queries  *database.Queries
	config   config.SimulateConfig
	activity *ActivityCalculator

	// Customer cache for weighted selection
	customersByTZ map[string][]int64 // timezone -> customer IDs
	allCustomerIDs []int64
	cacheMu        sync.RWMutex
	cacheTime      time.Time

	// Rate control
	targetTPS      float64 // Target transactions per second
	burstEnabled   bool
	burstMultiplier float64

	// Statistics
	stats SchedulerStats
	statsMu sync.Mutex
}

// SchedulerStats tracks scheduling statistics
type SchedulerStats struct {
	TotalScheduled      int64
	ScheduledByTimezone map[string]int64
	BurstTriggered      int64
	SkippedInactive     int64
}

// NewScheduler creates a new scheduler
func NewScheduler(queries *database.Queries, cfg config.SimulateConfig) *Scheduler {
	// Create activity calculator and configure session type distribution
	activity := NewActivityCalculator(cfg.ActiveHourStart, cfg.ActiveHourEnd)
	activity.SetSessionTypeRatios(cfg.ATMSessionRatio, cfg.OnlineSessionRatio, cfg.BusinessSessionRatio)
	activity.SetPayrollBurst(cfg.BurstMultiplier)

	return &Scheduler{
		queries:         queries,
		config:          cfg,
		activity:        activity,
		customersByTZ:   make(map[string][]int64),
		burstEnabled:    cfg.EnablePayrollBurst || cfg.EnableLunchBurst,
		burstMultiplier: cfg.BurstMultiplier,
		stats: SchedulerStats{
			ScheduledByTimezone: make(map[string]int64),
		},
	}
}

// GetActivityCalculator returns the underlying activity calculator
func (s *Scheduler) GetActivityCalculator() *ActivityCalculator {
	return s.activity
}

// RefreshCustomerCache loads customer IDs grouped by timezone.
// This should be called periodically to handle new customers.
func (s *Scheduler) RefreshCustomerCache(ctx context.Context) error {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	// Get customer timezone distribution from database
	customers, err := s.queries.GetAllCustomerTimezones(ctx)
	if err != nil {
		return err
	}

	// Reset cache
	s.customersByTZ = make(map[string][]int64)
	s.allCustomerIDs = make([]int64, 0, len(customers))

	// Group by timezone
	for _, c := range customers {
		s.customersByTZ[c.Timezone] = append(s.customersByTZ[c.Timezone], c.ID)
		s.allCustomerIDs = append(s.allCustomerIDs, c.ID)
	}

	s.cacheTime = time.Now()
	return nil
}

// SelectCustomer chooses a customer for the next session using weighted selection.
// Customers in active timezones are more likely to be selected.
func (s *Scheduler) SelectCustomer(ctx context.Context, rng *utils.Random) (*models.Customer, error) {
	s.cacheMu.RLock()
	hasCache := len(s.allCustomerIDs) > 0
	s.cacheMu.RUnlock()

	// If no cache, fall back to random selection from DB
	if !hasCache {
		return s.queries.GetRandomCustomer(ctx)
	}

	// Calculate weighted selection across timezones
	customerID := s.selectWeightedCustomerID(rng)

	// Fetch full customer record
	return s.queries.GetCustomerByID(ctx, customerID)
}

// selectWeightedCustomerID picks a customer ID favoring active timezones
func (s *Scheduler) selectWeightedCustomerID(rng *utils.Random) int64 {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	// Build weighted selection
	type tzWeight struct {
		timezone string
		weight   float64
		cumulative float64
	}

	weights := make([]tzWeight, 0, len(s.customersByTZ))
	totalWeight := 0.0

	for tz, customers := range s.customersByTZ {
		// Weight = activity probability * number of customers in timezone
		prob := s.activity.CalculateActivityProbability(&models.Customer{Timezone: tz})
		weight := prob * float64(len(customers))

		// Ensure minimum weight so no timezone is completely ignored
		if weight < 0.1 {
			weight = 0.1
		}

		totalWeight += weight
		weights = append(weights, tzWeight{
			timezone:   tz,
			weight:     weight,
			cumulative: totalWeight,
		})
	}

	// Select timezone based on weights
	r := rng.Float64() * totalWeight
	var selectedTZ string
	for _, tw := range weights {
		if r <= tw.cumulative {
			selectedTZ = tw.timezone
			break
		}
	}

	// If nothing selected (shouldn't happen), use first timezone
	if selectedTZ == "" && len(weights) > 0 {
		selectedTZ = weights[0].timezone
	}

	// Select random customer from chosen timezone
	customers := s.customersByTZ[selectedTZ]
	if len(customers) == 0 {
		// Fallback to any customer
		return s.allCustomerIDs[rng.IntN(len(s.allCustomerIDs))]
	}

	// Track stats
	s.statsMu.Lock()
	s.stats.TotalScheduled++
	s.stats.ScheduledByTimezone[selectedTZ]++
	s.statsMu.Unlock()

	return customers[rng.IntN(len(customers))]
}

// ShouldExecuteSession determines if a session should actually run
// based on the customer's current activity probability.
// This provides another layer of filtering after customer selection.
func (s *Scheduler) ShouldExecuteSession(customer *models.Customer, rng *utils.Random) bool {
	decision := s.activity.MakeActivityDecision(customer, rng)

	if !decision.ShouldExecute {
		s.statsMu.Lock()
		s.stats.SkippedInactive++
		s.statsMu.Unlock()
	}

	return decision.ShouldExecute
}

// GetSessionPacing returns the recommended delay between sessions
// based on current global activity and burst conditions.
func (s *Scheduler) GetSessionPacing(customer *models.Customer) time.Duration {
	baseDelay := time.Duration(float64(time.Second) / float64(s.config.NumSessions))

	// Get think time multiplier (faster during peak hours)
	thinkMult := s.activity.GetThinkTimeMultiplier(customer.Timezone)

	// Check for burst conditions
	if s.burstEnabled {
		isPayroll := s.config.EnablePayrollBurst && s.activity.IsPayrollPeriod(customer.Timezone)
		isLunch := s.config.EnableLunchBurst && s.activity.IsLunchHour(customer.Timezone)

		if isPayroll || isLunch {
			s.statsMu.Lock()
			s.stats.BurstTriggered++
			s.statsMu.Unlock()

			// Reduce delay during bursts (more sessions)
			baseDelay = time.Duration(float64(baseDelay) / s.burstMultiplier)
		}
	}

	// Apply think time multiplier
	return time.Duration(float64(baseDelay) * thinkMult)
}

// GetRecommendedSessionType returns the session type based on time and customer
func (s *Scheduler) GetRecommendedSessionType(customer *models.Customer, rng *utils.Random) SessionType {
	return s.activity.GetRecommendedSessionType(customer, rng)
}

// GetStats returns current scheduler statistics
func (s *Scheduler) GetStats() SchedulerStats {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()

	// Copy stats
	statsCopy := SchedulerStats{
		TotalScheduled:      s.stats.TotalScheduled,
		ScheduledByTimezone: make(map[string]int64),
		BurstTriggered:      s.stats.BurstTriggered,
		SkippedInactive:     s.stats.SkippedInactive,
	}
	for k, v := range s.stats.ScheduledByTimezone {
		statsCopy.ScheduledByTimezone[k] = v
	}
	return statsCopy
}

// GetGlobalActivitySummary returns a summary of current global activity
func (s *Scheduler) GetGlobalActivitySummary() string {
	snapshot := s.activity.GetGlobalActivitySnapshot()

	// Count active regions
	activeCount := 0
	for _, level := range snapshot.RegionalActivity {
		if level > 0.3 { // Consider "active" if > 30%
			activeCount++
		}
	}

	if snapshot.IsPayrollDay {
		return "PAYROLL BURST"
	}
	if activeCount >= 3 {
		return "HIGH (global)"
	}
	if activeCount >= 2 {
		return "MEDIUM"
	}
	if activeCount >= 1 {
		return "LOW"
	}
	return "MINIMAL"
}

// ScheduledSession represents a session that has been scheduled
type ScheduledSession struct {
	Customer        *models.Customer
	SessionType     SessionType
	ActivityDecision ActivityDecision
	ScheduledAt     time.Time
}

// ScheduleNextSession performs full scheduling: selects customer, checks activity,
// determines session type, and returns a ready-to-execute session.
func (s *Scheduler) ScheduleNextSession(ctx context.Context, rng *utils.Random) (*ScheduledSession, error) {
	// Select customer with timezone weighting
	customer, err := s.SelectCustomer(ctx, rng)
	if err != nil {
		return nil, err
	}

	// Make activity decision
	decision := s.activity.MakeActivityDecision(customer, rng)

	// Create scheduled session
	return &ScheduledSession{
		Customer:         customer,
		SessionType:      decision.RecommendedType,
		ActivityDecision: decision,
		ScheduledAt:      time.Now(),
	}, nil
}

// CacheStats returns cache statistics for monitoring
type CacheStats struct {
	TotalCustomers     int
	TimezoneCount      int
	CacheAge           time.Duration
	CustomersPerTimezone map[string]int
}

// GetCacheStats returns current cache statistics
func (s *Scheduler) GetCacheStats() CacheStats {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	stats := CacheStats{
		TotalCustomers:       len(s.allCustomerIDs),
		TimezoneCount:        len(s.customersByTZ),
		CacheAge:             time.Since(s.cacheTime),
		CustomersPerTimezone: make(map[string]int),
	}

	for tz, customers := range s.customersByTZ {
		stats.CustomersPerTimezone[tz] = len(customers)
	}

	return stats
}
