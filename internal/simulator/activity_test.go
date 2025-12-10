package simulator

import (
	"testing"

	"github.com/willfong/load-generator/internal/models"
	"github.com/willfong/load-generator/internal/utils"
)

func TestActivityCalculator_New(t *testing.T) {
	ac := NewActivityCalculator(8, 16)

	if ac == nil {
		t.Fatal("expected non-nil ActivityCalculator")
	}
	if ac.timezone == nil {
		t.Error("expected timezone manager to be initialized")
	}
	if len(ac.payrollDays) == 0 {
		t.Error("expected payroll days to be initialized")
	}
	if len(ac.billPaymentDays) == 0 {
		t.Error("expected bill payment days to be initialized")
	}
}

func TestActivityCalculator_SetBurstMultipliers(t *testing.T) {
	ac := NewActivityCalculator(8, 16)

	ac.SetPayrollBurst(3.0)
	if ac.payrollBurst != 3.0 {
		t.Errorf("expected payroll burst 3.0, got %.2f", ac.payrollBurst)
	}

	ac.SetLunchBurst(2.5)
	if ac.lunchBurst != 2.5 {
		t.Errorf("expected lunch burst 2.5, got %.2f", ac.lunchBurst)
	}
}

func TestActivityCalculator_GetTimezoneManager(t *testing.T) {
	ac := NewActivityCalculator(8, 16)
	tm := ac.GetTimezoneManager()

	if tm == nil {
		t.Error("expected non-nil timezone manager")
	}
	if tm.activeStart != 8 || tm.activeEnd != 16 {
		t.Error("timezone manager should have same active hours")
	}
}

func TestActivityCalculator_SegmentModifier(t *testing.T) {
	ac := NewActivityCalculator(8, 16)

	tests := []struct {
		segment  models.CustomerSegment
		minMod   float64
		maxMod   float64
	}{
		{models.SegmentCorporate, 1.4, 1.6},
		{models.SegmentBusiness, 1.2, 1.4},
		{models.SegmentPrivate, 1.1, 1.3},
		{models.SegmentPremium, 1.0, 1.2},
		{models.SegmentRegular, 0.9, 1.1},
	}

	for _, tt := range tests {
		t.Run(string(tt.segment), func(t *testing.T) {
			mod := ac.getSegmentModifier(tt.segment)
			if mod < tt.minMod || mod > tt.maxMod {
				t.Errorf("segment %s: expected modifier in [%.1f, %.1f], got %.2f",
					tt.segment, tt.minMod, tt.maxMod, mod)
			}
		})
	}
}

func TestActivityCalculator_CalculateActivityProbability(t *testing.T) {
	ac := NewActivityCalculator(8, 16)

	customer := &models.Customer{
		Timezone:      "America/New_York",
		Segment:       models.SegmentRegular,
		ActivityScore: 0.5, // Average activity
	}

	prob := ac.CalculateActivityProbability(customer)

	// Probability should be between 0 and 1
	if prob < 0 || prob > 1 {
		t.Errorf("probability should be in [0, 1], got %.4f", prob)
	}
}

func TestActivityCalculator_HighActivityScore(t *testing.T) {
	ac := NewActivityCalculator(8, 16)

	lowActivity := &models.Customer{
		Timezone:      "America/New_York",
		Segment:       models.SegmentRegular,
		ActivityScore: 0.1,
	}

	highActivity := &models.Customer{
		Timezone:      "America/New_York",
		Segment:       models.SegmentRegular,
		ActivityScore: 0.9,
	}

	lowProb := ac.CalculateActivityProbability(lowActivity)
	highProb := ac.CalculateActivityProbability(highActivity)

	// Higher activity score should generally lead to higher probability
	// (other factors being equal)
	if highProb <= lowProb {
		t.Errorf("high activity score (%.4f) should have higher probability than low (%.4f)",
			highProb, lowProb)
	}
}

func TestActivityCalculator_CorporateVsRegular(t *testing.T) {
	ac := NewActivityCalculator(8, 16)

	regular := &models.Customer{
		Timezone:      "America/New_York",
		Segment:       models.SegmentRegular,
		ActivityScore: 0.5,
	}

	corporate := &models.Customer{
		Timezone:      "America/New_York",
		Segment:       models.SegmentCorporate,
		ActivityScore: 0.5,
	}

	regularProb := ac.CalculateActivityProbability(regular)
	corporateProb := ac.CalculateActivityProbability(corporate)

	// Corporate segment should have higher multiplier
	if corporateProb <= regularProb {
		t.Errorf("corporate (%.4f) should have higher probability than regular (%.4f)",
			corporateProb, regularProb)
	}
}

func TestActivityCalculator_ShouldBeActive_Deterministic(t *testing.T) {
	// Use a wide active window (0-24) to ensure we're always in business hours
	ac := NewActivityCalculator(0, 24)
	rng := utils.NewRandom(42)

	customer := &models.Customer{
		Timezone:      "UTC", // Use UTC for predictability
		Segment:       models.SegmentRegular,
		ActivityScore: 0.5,
	}

	// Run multiple times and count active decisions
	activeCount := 0
	totalRuns := 1000

	for i := 0; i < totalRuns; i++ {
		if ac.ShouldBeActive(customer, rng) {
			activeCount++
		}
	}

	// With 24-hour active window and 0.5 activity score, we should see
	// a reasonable mix of active and inactive decisions
	// (based on intraday patterns and activity score)
	if activeCount == 0 {
		t.Errorf("expected some active decisions, got 0 out of %d", totalRuns)
	}
	if activeCount == totalRuns {
		t.Errorf("expected some inactive decisions, got %d out of %d", activeCount, totalRuns)
	}
}

func TestActivityCalculator_MakeActivityDecision(t *testing.T) {
	ac := NewActivityCalculator(8, 16)
	rng := utils.NewRandom(42)

	customer := &models.Customer{
		Timezone:      "America/New_York",
		Segment:       models.SegmentRegular,
		ActivityScore: 0.5,
	}

	decision := ac.MakeActivityDecision(customer, rng)

	// Decision should have all fields populated
	if decision.Probability < 0 || decision.Probability > 1 {
		t.Errorf("probability should be in [0, 1], got %.4f", decision.Probability)
	}

	if decision.ThinkTimeMultiplier <= 0 {
		t.Error("think time multiplier should be positive")
	}

	// Session type should be valid
	if decision.RecommendedType != SessionTypeATM &&
		decision.RecommendedType != SessionTypeOnline &&
		decision.RecommendedType != SessionTypeBusiness {
		t.Errorf("invalid session type: %v", decision.RecommendedType)
	}
}

func TestActivityCalculator_GetThinkTimeMultiplier(t *testing.T) {
	ac := NewActivityCalculator(8, 16)

	// Multiplier should always be positive
	tz := "America/New_York"
	mult := ac.GetThinkTimeMultiplier(tz)

	if mult <= 0 || mult > 1.5 {
		t.Errorf("think time multiplier should be in (0, 1.5], got %.2f", mult)
	}
}

func TestActivityCalculator_GetRecommendedSessionType_Business(t *testing.T) {
	ac := NewActivityCalculator(8, 16)
	rng := utils.NewRandom(42)

	businessCustomer := &models.Customer{
		Timezone:      "America/New_York",
		Segment:       models.SegmentBusiness,
		ActivityScore: 0.5,
	}

	corporateCustomer := &models.Customer{
		Timezone:      "America/New_York",
		Segment:       models.SegmentCorporate,
		ActivityScore: 0.5,
	}

	// Business and corporate customers should get business session type
	for i := 0; i < 10; i++ {
		sessType := ac.GetRecommendedSessionType(businessCustomer, rng)
		if sessType != SessionTypeBusiness {
			t.Errorf("business customer should get business session, got %v", sessType)
		}

		sessType = ac.GetRecommendedSessionType(corporateCustomer, rng)
		if sessType != SessionTypeBusiness {
			t.Errorf("corporate customer should get business session, got %v", sessType)
		}
	}
}

func TestActivityCalculator_GlobalActivitySnapshot(t *testing.T) {
	ac := NewActivityCalculator(8, 16)

	snapshot := ac.GetGlobalActivitySnapshot()

	// Timestamp should be recent
	if snapshot.Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}

	// Regional activity should have entries for major regions
	regions := []string{"Americas", "Europe", "Asia", "Pacific"}
	for _, region := range regions {
		if _, ok := snapshot.RegionalActivity[region]; !ok {
			t.Errorf("expected activity for region %s", region)
		}
	}

	// All activity levels should be between 0 and 1
	for region, level := range snapshot.RegionalActivity {
		if level < 0 || level > 1 {
			t.Errorf("activity level for %s should be in [0, 1], got %.4f", region, level)
		}
	}
}
