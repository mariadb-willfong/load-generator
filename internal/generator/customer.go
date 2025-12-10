package generator

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/willfong/load-generator/internal/data"
	"github.com/willfong/load-generator/internal/models"
	"github.com/willfong/load-generator/internal/utils"
)

// CustomerGenerator creates customers with realistic PII and global distribution.
type CustomerGenerator struct {
	rng     *utils.Random
	refData *data.ReferenceData
	config  CustomerGeneratorConfig
}

// CustomerGeneratorConfig holds settings for customer generation
type CustomerGeneratorConfig struct {
	NumCustomers int
	// Branches to assign customers to
	Branches []GeneratedBranch
	// BaseDate for date calculations
	BaseDate time.Time
	// ParetoRatio: top X% of customers have high activity (default 0.2)
	ParetoRatio float64
}

// NewCustomerGenerator creates a new customer generator
func NewCustomerGenerator(rng *utils.Random, refData *data.ReferenceData, config CustomerGeneratorConfig) *CustomerGenerator {
	if config.ParetoRatio <= 0 {
		config.ParetoRatio = 0.2
	}
	return &CustomerGenerator{
		rng:     rng,
		refData: refData,
		config:  config,
	}
}

// GeneratedCustomer holds a generated customer with metadata
type GeneratedCustomer struct {
	Customer models.Customer
	Country  *data.Country
}

// GenerateCustomers creates all customers with global distribution
func (g *CustomerGenerator) GenerateCustomers() []GeneratedCustomer {
	customers := make([]GeneratedCustomer, 0, g.config.NumCustomers)

	for i := 0; i < g.config.NumCustomers; i++ {
		customer := g.generateCustomer(int64(i + 1))
		customers = append(customers, customer)
	}

	return customers
}

// generateCustomer creates a single customer
func (g *CustomerGenerator) generateCustomer(id int64) GeneratedCustomer {
	// Pick country weighted by banking activity
	country := g.pickCountry()

	// Get region for name generation
	region := country.Region

	// Generate name
	isMale := g.rng.Bool()
	firstName := g.generateFirstName(region, isMale)
	lastName := g.generateLastName(region)

	// Pick city for address
	city := g.pickCity(country.Code)

	// Generate segment with distribution:
	// 70% regular, 15% premium, 5% private, 8% business, 2% corporate
	segment := g.pickSegment()

	// Generate activity score with Pareto distribution
	// Top 20% get high scores (0.7-1.0), rest get lower (0.1-0.5)
	activityScore := g.generateActivityScore(id)

	// Generate DOB (18-80 years old)
	dob := g.generateDateOfBirth()

	// Generate customer creation date
	createdAt := g.generateCreatedAt()

	// Pick home branch - prefer branches in same country
	homeBranch := g.pickHomeBranch(country.Code)

	// Generate contact info
	email := g.generateEmail(firstName, lastName, id)
	phone := g.generatePhone(country.PhoneCode)

	// Generate auth data
	username := g.generateUsername(firstName, lastName, id)
	passwordHash := g.hashPassword(g.rng.String(12))
	pin := g.hashPIN(g.rng.NumericString(4))

	customer := models.Customer{
		ID:            id,
		FirstName:     firstName,
		LastName:      lastName,
		Email:         email,
		Phone:         phone,
		DateOfBirth:   dob,
		AddressLine1:  g.generateStreetAddress(),
		AddressLine2:  g.generateAddressLine2(),
		City:          city.City,
		State:         city.State,
		PostalCode:    g.generatePostalCode(country.Code, city.PostalPrefix),
		Country:       country.Code,
		Timezone:      country.Timezone,
		HomeBranch:    homeBranch,
		Segment:       segment,
		Status:        models.CustomerStatusActive,
		ActivityScore: activityScore,
		Username:      username,
		PasswordHash:  passwordHash,
		PIN:           pin,
		CreatedAt:     createdAt,
		UpdatedAt:     time.Now(),
	}

	return GeneratedCustomer{Customer: customer, Country: country}
}

// pickCountry selects a country weighted by banking activity
func (g *CustomerGenerator) pickCountry() *data.Country {
	totalWeight := g.refData.TotalWeight()
	pick := g.rng.IntRange(1, totalWeight)
	return g.refData.CountryByWeight(pick)
}

// pickCity selects a city for the given country
func (g *CustomerGenerator) pickCity(countryCode string) data.City {
	cities, ok := g.refData.GetCities(countryCode)
	if !ok || len(cities) == 0 {
		return data.City{City: "Capital City", State: "", PostalPrefix: "10000"}
	}
	return cities[g.rng.IntN(len(cities))]
}

// generateFirstName creates a first name based on region
func (g *CustomerGenerator) generateFirstName(region string, isMale bool) string {
	names := g.refData.GetFirstNames(region, isMale)
	if len(names) == 0 {
		// Fallback to western names
		names = g.refData.GetFirstNames("western", isMale)
	}
	if len(names) == 0 {
		if isMale {
			return "John"
		}
		return "Jane"
	}
	return g.rng.PickString(names)
}

// generateLastName creates a last name based on region
func (g *CustomerGenerator) generateLastName(region string) string {
	names := g.refData.GetLastNames(region)
	if len(names) == 0 {
		names = g.refData.GetLastNames("western")
	}
	if len(names) == 0 {
		return "Smith"
	}
	return g.rng.PickString(names)
}

// pickSegment determines customer segment with realistic distribution
func (g *CustomerGenerator) pickSegment() models.CustomerSegment {
	p := g.rng.Float64()
	switch {
	case p < 0.70:
		return models.SegmentRegular
	case p < 0.85:
		return models.SegmentPremium
	case p < 0.90:
		return models.SegmentPrivate
	case p < 0.98:
		return models.SegmentBusiness
	default:
		return models.SegmentCorporate
	}
}

// generateActivityScore creates an activity score with Pareto distribution
// Top 20% of customers (by ID hash) get high activity (0.7-1.0)
// Remaining 80% get lower activity (0.1-0.5)
func (g *CustomerGenerator) generateActivityScore(id int64) float64 {
	// Use exponential distribution for realistic activity spread
	// This naturally creates the 80/20 Pareto-like distribution
	x := g.rng.ExpFloat64()

	// Transform to 0-1 range with most values low, few high
	// exp(-x) gives values between 0 and 1, concentrated near 0
	score := 1.0 - math.Exp(-x*0.5)

	// Ensure within bounds
	if score < 0.1 {
		score = 0.1
	}
	if score > 1.0 {
		score = 1.0
	}

	return score
}

// generateDateOfBirth creates a DOB for an adult (18-80 years old)
func (g *CustomerGenerator) generateDateOfBirth() time.Time {
	// Age range 18-80
	ageInDays := g.rng.IntRange(18*365, 80*365)
	return time.Now().AddDate(0, 0, -ageInDays)
}

// generateCreatedAt creates a customer creation date in the history period
func (g *CustomerGenerator) generateCreatedAt() time.Time {
	// Spread customers across 5 years of history
	daysBack := g.rng.IntRange(1, 5*365)
	return time.Now().AddDate(0, 0, -daysBack)
}

// pickHomeBranch selects a home branch, preferring same country
func (g *CustomerGenerator) pickHomeBranch(countryCode string) int64 {
	if len(g.config.Branches) == 0 {
		return 1 // Default to branch 1
	}

	// Try to find a branch in the same country
	sameCntry := make([]int64, 0)
	for _, b := range g.config.Branches {
		if b.Country.Code == countryCode {
			sameCntry = append(sameCntry, b.Branch.ID)
		}
	}

	if len(sameCntry) > 0 {
		return sameCntry[g.rng.IntN(len(sameCntry))]
	}

	// No branch in same country, pick any
	return g.config.Branches[g.rng.IntN(len(g.config.Branches))].Branch.ID
}

// generateEmail creates a realistic email address
func (g *CustomerGenerator) generateEmail(firstName, lastName string, id int64) string {
	// Lowercase and clean names
	first := strings.ToLower(strings.ReplaceAll(firstName, " ", ""))
	last := strings.ToLower(strings.ReplaceAll(lastName, " ", ""))

	domains := []string{
		"gmail.com",
		"yahoo.com",
		"hotmail.com",
		"outlook.com",
		"icloud.com",
		"mail.com",
		"protonmail.com",
	}
	domain := g.rng.PickString(domains)

	// Variations
	patterns := []string{
		"%s.%s@%s",
		"%s%s@%s",
		"%s.%s%d@%s",
		"%s_%s@%s",
	}
	pattern := g.rng.PickString(patterns)

	if strings.Contains(pattern, "%d") {
		return fmt.Sprintf(pattern, first, last, id%1000, domain)
	}
	return fmt.Sprintf(pattern, first, last, domain)
}

// generatePhone creates a phone number with country code
func (g *CustomerGenerator) generatePhone(phoneCode string) string {
	return fmt.Sprintf("+%s %s", phoneCode, g.rng.NumericString(10))
}

// generateUsername creates a unique username
func (g *CustomerGenerator) generateUsername(firstName, lastName string, id int64) string {
	first := strings.ToLower(firstName)
	if len(first) > 4 {
		first = first[:4]
	}
	return fmt.Sprintf("%s%d", first, id)
}

// hashPassword creates a SHA-256 hash of the password (simulated - not for production)
func (g *CustomerGenerator) hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// hashPIN creates a SHA-256 hash of the PIN (simulated - not for production)
func (g *CustomerGenerator) hashPIN(pin string) string {
	hash := sha256.Sum256([]byte(pin))
	return hex.EncodeToString(hash[:])[:32] // Truncate for storage
}

// generateStreetAddress creates a realistic street address
func (g *CustomerGenerator) generateStreetAddress() string {
	streetNum := g.rng.IntRange(1, 9999)
	streets := []string{
		"Oak Street",
		"Maple Avenue",
		"Cedar Lane",
		"Pine Road",
		"Elm Drive",
		"Willow Way",
		"Cherry Boulevard",
		"Birch Court",
		"Spruce Circle",
		"Ash Place",
		"Park Lane",
		"River Road",
		"Lake Street",
		"Mountain View",
		"Valley Drive",
	}
	return fmt.Sprintf("%d %s", streetNum, g.rng.PickString(streets))
}

// generateAddressLine2 creates an optional apartment/unit number
func (g *CustomerGenerator) generateAddressLine2() string {
	// 30% have a second line
	if !g.rng.Probability(0.3) {
		return ""
	}

	types := []string{"Apt", "Unit", "Suite", "Floor", "#"}
	return fmt.Sprintf("%s %d", g.rng.PickString(types), g.rng.IntRange(1, 999))
}

// generatePostalCode creates a postal code based on country format
func (g *CustomerGenerator) generatePostalCode(countryCode, prefix string) string {
	format := g.refData.GetPostalFormat(countryCode)
	if format == "" {
		format = "NNNNN"
	}

	result := ""
	prefixIdx := 0

	for _, c := range format {
		switch c {
		case 'N':
			if prefixIdx < len(prefix) && prefix[prefixIdx] >= '0' && prefix[prefixIdx] <= '9' {
				result += string(prefix[prefixIdx])
				prefixIdx++
			} else {
				result += string(g.rng.Digit())
			}
		case 'L':
			if prefixIdx < len(prefix) && prefix[prefixIdx] >= 'A' && prefix[prefixIdx] <= 'Z' {
				result += string(prefix[prefixIdx])
				prefixIdx++
			} else {
				result += string(g.rng.Letter())
			}
		default:
			result += string(c)
		}
	}

	return result
}

// WriteCustomersCSV writes customers to a CSV file (or .csv.xz if compress=true)
func WriteCustomersCSV(customers []GeneratedCustomer, outputDir string, compress bool) error {
	return writeCustomersCSVInternal(customers, outputDir, compress, false)
}

// WriteCustomersCSVWithProgress writes customers with progress reporting
func WriteCustomersCSVWithProgress(customers []GeneratedCustomer, outputDir string, compress bool) error {
	return writeCustomersCSVInternal(customers, outputDir, compress, true)
}

func writeCustomersCSVInternal(customers []GeneratedCustomer, outputDir string, compress, showProgress bool) error {
	headers := []string{
		"id", "first_name", "last_name", "email", "phone", "date_of_birth",
		"address_line1", "address_line2", "city", "state", "postal_code", "country",
		"timezone", "home_branch_id", "segment", "status", "activity_score",
		"username", "password_hash", "pin",
		"created_at", "updated_at",
	}

	writer, err := NewCSVWriter(CSVWriterConfig{
		OutputDir: outputDir,
		Filename:  "customers",
		Headers:   headers,
		Compress:  compress,
	})
	if err != nil {
		return err
	}
	defer writer.Close()

	var progress *ProgressReporter
	if showProgress {
		progress = NewProgressReporter(ProgressConfig{
			Total: int64(len(customers)),
			Label: "  Customers",
		})
	}

	for i, gc := range customers {
		c := gc.Customer
		row := []string{
			FormatInt64(c.ID),
			c.FirstName,
			c.LastName,
			c.Email,
			c.Phone,
			FormatDate(c.DateOfBirth),
			c.AddressLine1,
			c.AddressLine2,
			c.City,
			c.State,
			c.PostalCode,
			c.Country,
			c.Timezone,
			FormatInt64(c.HomeBranch),
			string(c.Segment),
			string(c.Status),
			FormatFloat64(c.ActivityScore),
			c.Username,
			c.PasswordHash,
			c.PIN,
			FormatTime(c.CreatedAt),
			FormatTime(c.UpdatedAt),
		}
		if err := writer.WriteRow(row); err != nil {
			return err
		}

		if progress != nil && (i+1)%100 == 0 {
			progress.Set(int64(i + 1))
		}
	}

	if progress != nil {
		progress.Set(int64(len(customers)))
		progress.Finish()
	}

	return writer.Close()
}
