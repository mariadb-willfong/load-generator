package patterns

import (
	"math"
	"sort"
)

// ActivityDistribution assigns transaction frequencies to accounts
// following realistic patterns like Pareto (80/20 rule).
type ActivityDistribution struct {
	// paretoRatio is the fraction of accounts that generate most activity
	// e.g., 0.2 means 20% of accounts generate ~80% of transactions
	paretoRatio float64

	// paretoIntensity controls how steep the distribution is
	// higher values = more concentrated activity among top accounts
	paretoIntensity float64
}

// NewParetoDistribution creates a Pareto-based activity distribution.
// With ratio=0.2, approximately 20% of accounts will generate 80% of activity.
func NewParetoDistribution(ratio float64) *ActivityDistribution {
	if ratio <= 0 || ratio > 1 {
		ratio = 0.2 // Default 80/20 rule
	}

	// Calculate intensity to achieve the desired ratio
	// Using Pareto principle: ratio^intensity accounts generate (1-ratio)^intensity transactions
	intensity := math.Log(0.8) / math.Log(ratio)

	return &ActivityDistribution{
		paretoRatio:     ratio,
		paretoIntensity: intensity,
	}
}

// NewUniformDistribution creates an even distribution where all accounts
// have similar activity levels.
func NewUniformDistribution() *ActivityDistribution {
	return &ActivityDistribution{
		paretoRatio:     1.0,
		paretoIntensity: 1.0,
	}
}

// NewHighlySkewedDistribution creates a distribution where 10% of accounts
// generate 90% of activity. Useful for modeling merchant accounts.
func NewHighlySkewedDistribution() *ActivityDistribution {
	return NewParetoDistribution(0.1)
}

// GenerateActivityScore generates an activity score (0.0-1.0) for an account
// based on its position in the Pareto distribution.
// Higher scores = more active accounts.
// The percentile should be uniformly distributed (e.g., from RNG).
func (ad *ActivityDistribution) GenerateActivityScore(percentile float64) float64 {
	if percentile < 0 {
		percentile = 0
	}
	if percentile > 1 {
		percentile = 1
	}

	// Invert the Pareto CDF to get activity score
	// Higher percentile (closer to 1.0) = higher activity
	// Using power law: score = percentile^(1/intensity)
	score := math.Pow(percentile, 1.0/ad.paretoIntensity)

	return score
}

// TransactionsPerMonth calculates expected monthly transactions for an account
// based on its activity score.
// baseTransactions is the average transactions per month.
func (ad *ActivityDistribution) TransactionsPerMonth(activityScore float64, baseTransactions int) int {
	if activityScore < 0 {
		activityScore = 0
	}
	if activityScore > 1 {
		activityScore = 1
	}

	// Scale transactions based on activity score
	// High-activity accounts (score ~1.0) get 2-3x average
	// Low-activity accounts (score ~0.1) get 0.2x average
	multiplier := 0.2 + (activityScore * 2.8) // Range: 0.2 to 3.0

	return int(float64(baseTransactions) * multiplier)
}

// TransactionsPerDay converts monthly transaction count to daily rate
// accounting for weekday/weekend differences.
func (ad *ActivityDistribution) TransactionsPerDay(monthlyCount int, isWeekend bool) float64 {
	// Assume 22 business days, 8 weekend days
	// Weekends have ~40% of weekday activity
	weekdayTransactions := float64(monthlyCount) / 22.0 * 0.7
	weekendTransactions := float64(monthlyCount) / 8.0 * 0.3

	if isWeekend {
		return weekendTransactions
	}
	return weekdayTransactions
}

// AccountActivityTier categorizes accounts by activity level.
type AccountActivityTier string

const (
	TierHighActivity   AccountActivityTier = "high"   // Top 10% - daily transactions
	TierMediumActivity AccountActivityTier = "medium" // Next 30% - weekly transactions
	TierLowActivity    AccountActivityTier = "low"    // Bottom 60% - monthly transactions
)

// GetTier returns the activity tier for an account based on its activity score.
func (ad *ActivityDistribution) GetTier(activityScore float64) AccountActivityTier {
	switch {
	case activityScore >= 0.9:
		return TierHighActivity
	case activityScore >= 0.6:
		return TierMediumActivity
	default:
		return TierLowActivity
	}
}

// GetTierTransactionRange returns the typical transaction count range for a tier.
// Returns (min, max) transactions per month.
func (ad *ActivityDistribution) GetTierTransactionRange(tier AccountActivityTier) (int, int) {
	switch tier {
	case TierHighActivity:
		return 50, 200 // Very active: 50-200 transactions/month
	case TierMediumActivity:
		return 15, 50 // Moderate: 15-50 transactions/month
	case TierLowActivity:
		return 2, 15 // Infrequent: 2-15 transactions/month
	default:
		return 5, 20
	}
}

// DistributeTransactions distributes a total transaction count across N accounts
// according to the Pareto distribution.
// Returns a slice of transaction counts per account (sorted high to low).
func (ad *ActivityDistribution) DistributeTransactions(totalTransactions int, numAccounts int) []int {
	if numAccounts <= 0 {
		return nil
	}

	result := make([]int, numAccounts)

	// Generate activity scores for each account
	scores := make([]float64, numAccounts)
	for i := 0; i < numAccounts; i++ {
		// Percentile from 0 to 1, evenly distributed
		percentile := float64(i+1) / float64(numAccounts+1)
		scores[i] = ad.GenerateActivityScore(percentile)
	}

	// Calculate total score for normalization
	var totalScore float64
	for _, s := range scores {
		totalScore += s
	}

	// Distribute transactions proportionally
	var distributed int
	for i, score := range scores {
		count := int(float64(totalTransactions) * (score / totalScore))
		result[i] = count
		distributed += count
	}

	// Distribute remainder to highest-activity accounts
	remainder := totalTransactions - distributed
	for i := len(result) - 1; remainder > 0 && i >= 0; i-- {
		result[i]++
		remainder--
	}

	// Sort high to low
	sort.Sort(sort.Reverse(sort.IntSlice(result)))

	return result
}

// GetCumulativeShare returns what percentage of transactions are generated
// by the top X% of accounts.
func (ad *ActivityDistribution) GetCumulativeShare(topPercentage float64) float64 {
	if topPercentage <= 0 {
		return 0
	}
	if topPercentage >= 1 {
		return 1
	}

	// Using Pareto distribution property
	return 1 - math.Pow(1-topPercentage, ad.paretoIntensity)
}

// AmountDistribution provides amount ranges for different transaction types.
type AmountDistribution struct {
	// Amount ranges in cents
	minAmount int64
	maxAmount int64

	// Distribution shape: "uniform", "normal", "exponential", "bimodal"
	shape string

	// For normal distribution: mean and standard deviation (as fractions of range)
	normalMean   float64 // Fraction of range where mean is (0.0-1.0)
	normalStdDev float64 // Standard deviation as fraction of range
}

// NewAmountRange creates a uniform amount distribution.
func NewAmountRange(minCents, maxCents int64) *AmountDistribution {
	return &AmountDistribution{
		minAmount: minCents,
		maxAmount: maxCents,
		shape:     "uniform",
	}
}

// NewNormalAmountRange creates a normal (Gaussian) distribution of amounts.
// meanFraction is where the mean falls in the range (0.3 = toward low end).
// stdDevFraction is standard deviation as fraction of range (0.2 = moderate spread).
func NewNormalAmountRange(minCents, maxCents int64, meanFraction, stdDevFraction float64) *AmountDistribution {
	return &AmountDistribution{
		minAmount:    minCents,
		maxAmount:    maxCents,
		shape:        "normal",
		normalMean:   meanFraction,
		normalStdDev: stdDevFraction,
	}
}

// NewExponentialAmountRange creates an exponential distribution (many small, few large).
// This models purchase behavior well where small transactions dominate.
func NewExponentialAmountRange(minCents, maxCents int64) *AmountDistribution {
	return &AmountDistribution{
		minAmount: minCents,
		maxAmount: maxCents,
		shape:     "exponential",
	}
}

// GenerateAmount generates an amount based on the distribution.
// rngValue should be uniformly distributed [0, 1).
// rngNormal should be normally distributed (for normal distribution type).
func (ad *AmountDistribution) GenerateAmount(rngValue float64, rngNormal float64) int64 {
	rangeSize := float64(ad.maxAmount - ad.minAmount)

	var fraction float64

	switch ad.shape {
	case "uniform":
		fraction = rngValue

	case "normal":
		// Use normal distribution, clamp to [0, 1]
		mean := ad.normalMean
		stdDev := ad.normalStdDev
		fraction = mean + rngNormal*stdDev
		if fraction < 0 {
			fraction = 0
		}
		if fraction > 1 {
			fraction = 1
		}

	case "exponential":
		// Exponential distribution: many small, few large
		// Use -ln(1-x) transformation, scaled to range
		if rngValue >= 0.9999 {
			rngValue = 0.9999
		}
		fraction = -math.Log(1-rngValue) / 5.0 // Scale factor of 5 for reasonable spread
		if fraction > 1 {
			fraction = 1
		}

	default:
		fraction = rngValue
	}

	amount := ad.minAmount + int64(rangeSize*fraction)

	// Round to nice values for realism
	amount = roundToNiceAmount(amount)

	// Ensure within bounds
	if amount < ad.minAmount {
		amount = ad.minAmount
	}
	if amount > ad.maxAmount {
		amount = ad.maxAmount
	}

	return amount
}

// roundToNiceAmount rounds to common monetary amounts.
// Small amounts round to cents, larger amounts to dollars or tens.
func roundToNiceAmount(cents int64) int64 {
	switch {
	case cents < 1000: // Under $10: round to 5 cents
		return (cents / 5) * 5
	case cents < 10000: // $10-100: round to 25 cents
		return (cents / 25) * 25
	case cents < 100000: // $100-1000: round to $1
		return (cents / 100) * 100
	case cents < 1000000: // $1000-10000: round to $5
		return (cents / 500) * 500
	default: // $10000+: round to $10
		return (cents / 1000) * 1000
	}
}

// TransactionTypeAmounts returns standard amount distributions for transaction types.
type TransactionTypeAmounts struct {
	// Common transaction amount ranges
	SmallPurchase   *AmountDistribution // Coffee, snacks: $2-$20
	MediumPurchase  *AmountDistribution // Groceries, meals: $20-$150
	LargePurchase   *AmountDistribution // Electronics, clothes: $100-$1000
	ATMWithdrawal   *AmountDistribution // Typical ATM: $20-$500
	BillPayment     *AmountDistribution // Utilities, subscriptions: $50-$500
	RentMortgage    *AmountDistribution // Housing: $800-$3000
	Salary          *AmountDistribution // Paychecks: $1500-$10000
	InternalTransfer *AmountDistribution // Between accounts: $100-$5000
}

// NewTransactionTypeAmounts creates standard amount distributions.
func NewTransactionTypeAmounts() *TransactionTypeAmounts {
	return &TransactionTypeAmounts{
		// Small purchases: exponential (many small, few at max)
		SmallPurchase: NewExponentialAmountRange(200, 2000), // $2-$20

		// Medium purchases: normal around $50
		MediumPurchase: NewNormalAmountRange(2000, 15000, 0.3, 0.25), // $20-$150, mean ~$50

		// Large purchases: normal around $300
		LargePurchase: NewNormalAmountRange(10000, 100000, 0.3, 0.3), // $100-$1000, mean ~$300

		// ATM: bimodal ($20-$60 common, occasionally $200-$500)
		ATMWithdrawal: NewNormalAmountRange(2000, 50000, 0.2, 0.25), // $20-$500, mean ~$100

		// Bills: normal around $100
		BillPayment: NewNormalAmountRange(5000, 50000, 0.3, 0.3), // $50-$500, mean ~$150

		// Rent/Mortgage: normal around $1500
		RentMortgage: NewNormalAmountRange(80000, 300000, 0.4, 0.25), // $800-$3000, mean ~$1500

		// Salary: normal around $4000
		Salary: NewNormalAmountRange(150000, 1000000, 0.35, 0.3), // $1500-$10000, mean ~$4000

		// Transfers: exponential (many small, few large)
		InternalTransfer: NewExponentialAmountRange(10000, 500000), // $100-$5000
	}
}
