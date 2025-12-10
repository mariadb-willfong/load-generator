package generator

import (
	"fmt"
	"strings"
	"time"

	"github.com/willfong/load-generator/internal/data"
	"github.com/willfong/load-generator/internal/models"
	"github.com/willfong/load-generator/internal/utils"
)

// BusinessType represents the type of business entity
type BusinessType string

const (
	BusinessTypeEmployer   BusinessType = "employer"
	BusinessTypeMerchant   BusinessType = "merchant"
	BusinessTypeUtility    BusinessType = "utility"
	BusinessTypeGovernment BusinessType = "government"
	BusinessTypeGeneral    BusinessType = "general"
)

// BusinessGenerator creates business entities for transaction counterparties.
// Businesses are stored as customers with business/corporate segments.
type BusinessGenerator struct {
	rng     *utils.Random
	refData *data.ReferenceData
	config  BusinessGeneratorConfig
}

// BusinessGeneratorConfig holds settings for business generation
type BusinessGeneratorConfig struct {
	NumBusinesses int
	// StartID is the starting customer ID for businesses (to avoid overlap with retail customers)
	StartID int64
	// Branches to assign businesses to
	Branches []GeneratedBranch
}

// NewBusinessGenerator creates a new business generator
func NewBusinessGenerator(rng *utils.Random, refData *data.ReferenceData, config BusinessGeneratorConfig) *BusinessGenerator {
	return &BusinessGenerator{
		rng:     rng,
		refData: refData,
		config:  config,
	}
}

// GeneratedBusiness holds a generated business with metadata
type GeneratedBusiness struct {
	Customer     models.Customer
	Country      *data.Country
	BusinessType BusinessType
	BusinessName string // Full business name (stored in FirstName field)
}

// GenerateBusinesses creates all business entities
func (g *BusinessGenerator) GenerateBusinesses() []GeneratedBusiness {
	businesses := make([]GeneratedBusiness, 0, g.config.NumBusinesses)

	// Distribution of business types:
	// 40% employers, 35% merchants, 15% utilities, 10% government
	distribution := g.calculateDistribution()

	for i := 0; i < g.config.NumBusinesses; i++ {
		bizType := g.pickBusinessType(i, distribution)
		business := g.generateBusiness(g.config.StartID+int64(i), bizType)
		businesses = append(businesses, business)
	}

	return businesses
}

// calculateDistribution determines how many of each business type to create
func (g *BusinessGenerator) calculateDistribution() map[BusinessType]int {
	n := g.config.NumBusinesses
	return map[BusinessType]int{
		BusinessTypeEmployer:   int(float64(n) * 0.40),
		BusinessTypeMerchant:   int(float64(n) * 0.35),
		BusinessTypeUtility:    int(float64(n) * 0.15),
		BusinessTypeGovernment: int(float64(n) * 0.10),
	}
}

// pickBusinessType determines business type based on distribution
func (g *BusinessGenerator) pickBusinessType(idx int, dist map[BusinessType]int) BusinessType {
	cumulative := 0
	for _, bt := range []BusinessType{
		BusinessTypeEmployer,
		BusinessTypeMerchant,
		BusinessTypeUtility,
		BusinessTypeGovernment,
	} {
		cumulative += dist[bt]
		if idx < cumulative {
			return bt
		}
	}
	return BusinessTypeGeneral
}

// generateBusiness creates a single business entity
func (g *BusinessGenerator) generateBusiness(id int64, bizType BusinessType) GeneratedBusiness {
	// Pick country weighted by economic activity
	country := g.pickCountry()

	// Generate business name based on type and country
	businessName := g.generateBusinessName(bizType, country)

	// Pick city for address
	city := g.pickCity(country.Code)

	// Determine segment based on business type
	segment := g.pickSegment(bizType)

	// Businesses have high activity scores (they're counterparties for many transactions)
	activityScore := g.rng.Float64Range(0.7, 1.0)

	// Generate creation date
	createdAt := g.generateCreatedAt()

	// Pick home branch
	homeBranch := g.pickHomeBranch(country.Code)

	// Generate contact info
	email := g.generateBusinessEmail(businessName, country.Code)
	phone := g.generatePhone(country.PhoneCode)

	// Generate auth data (businesses also have online banking access)
	username := g.generateUsername(businessName, id)
	passwordHash := hashString(g.rng.String(16))

	customer := models.Customer{
		ID:            id,
		FirstName:     businessName, // Store full business name in FirstName
		LastName:      string(bizType), // Store business type in LastName for identification
		Email:         email,
		Phone:         phone,
		DateOfBirth:   g.generateIncorporationDate(),
		AddressLine1:  g.generateBusinessAddress(),
		AddressLine2:  g.generateSuiteNumber(),
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
		PIN:           "", // Businesses don't use ATM PINs
		CreatedAt:     createdAt,
		UpdatedAt:     time.Now(),
	}

	return GeneratedBusiness{
		Customer:     customer,
		Country:      country,
		BusinessType: bizType,
		BusinessName: businessName,
	}
}

// generateBusinessName creates a realistic business name based on type
func (g *BusinessGenerator) generateBusinessName(bizType BusinessType, country *data.Country) string {
	switch bizType {
	case BusinessTypeEmployer:
		return g.generateEmployerName()
	case BusinessTypeMerchant:
		return g.generateMerchantName()
	case BusinessTypeUtility:
		return g.generateUtilityName(country)
	case BusinessTypeGovernment:
		return g.generateGovernmentName(country)
	default:
		return g.generateGenericBusinessName()
	}
}

// generateEmployerName creates a company name for payroll
func (g *BusinessGenerator) generateEmployerName() string {
	prefixes := []string{
		"Global", "United", "National", "International", "Premier",
		"Advanced", "Strategic", "Innovative", "Dynamic", "Precision",
		"Alpha", "Omega", "Summit", "Apex", "Pinnacle",
	}

	industries := []string{
		"Technologies", "Industries", "Solutions", "Systems", "Dynamics",
		"Enterprises", "Holdings", "Group", "Partners", "Associates",
		"Consulting", "Services", "Manufacturing", "Development", "Logistics",
	}

	suffixes := []string{"Inc.", "Corp.", "LLC", "Ltd.", "Co.", ""}

	name := fmt.Sprintf("%s %s", g.rng.PickString(prefixes), g.rng.PickString(industries))
	suffix := g.rng.PickString(suffixes)
	if suffix != "" {
		name = name + " " + suffix
	}

	return name
}

// generateMerchantName creates a retail/e-commerce business name
func (g *BusinessGenerator) generateMerchantName() string {
	types := []string{
		// Retail
		"Supermarket", "Grocery", "Department Store", "Electronics", "Fashion",
		"Hardware", "Pharmacy", "Books", "Sports", "Home Goods",
		// Food & Beverage
		"Restaurant", "Cafe", "Bakery", "Pizza", "Coffee Shop",
		// Services
		"Auto Parts", "Gas Station", "Dry Cleaning", "Salon", "Gym",
	}

	names := []string{
		"Quick", "Fresh", "Metro", "Urban", "City", "Express", "Plus",
		"Best", "Value", "Smart", "Super", "Mega", "Prime", "Gold",
	}

	merchantType := g.rng.PickString(types)
	namePart := g.rng.PickString(names)

	// Different patterns
	patterns := []string{
		"%s %s",
		"%s's %s",
		"The %s %s",
		"%s %s Center",
	}
	pattern := g.rng.PickString(patterns)

	return fmt.Sprintf(pattern, namePart, merchantType)
}

// generateUtilityName creates a utility company name
func (g *BusinessGenerator) generateUtilityName(country *data.Country) string {
	prefixes := []string{
		country.Name,
		"National",
		"Regional",
		"Metro",
		"City",
		"Central",
	}

	types := []string{
		"Electric Company",
		"Power & Light",
		"Gas Company",
		"Water Authority",
		"Telecom",
		"Cable Services",
		"Internet Services",
		"Energy",
		"Utilities",
	}

	return fmt.Sprintf("%s %s", g.rng.PickString(prefixes), g.rng.PickString(types))
}

// generateGovernmentName creates a government agency name
func (g *BusinessGenerator) generateGovernmentName(country *data.Country) string {
	agencies := []string{
		"Tax Authority",
		"Revenue Service",
		"Motor Vehicles Department",
		"Social Security Administration",
		"Immigration Services",
		"Customs Authority",
		"Municipal Treasury",
		"Property Tax Office",
		"Business Licensing",
		"Building Permits",
	}

	return fmt.Sprintf("%s %s", country.Name, g.rng.PickString(agencies))
}

// generateGenericBusinessName creates a generic business name
func (g *BusinessGenerator) generateGenericBusinessName() string {
	names := []string{
		"ACME Corporation",
		"Smith & Associates",
		"Johnson Enterprises",
		"Williams Holdings",
		"Brown Industries",
		"Davis Group",
		"Miller Partners",
		"Wilson Services",
		"Moore & Co.",
		"Taylor Business Solutions",
	}
	return g.rng.PickString(names)
}

// pickCountry selects a country weighted by economic activity
func (g *BusinessGenerator) pickCountry() *data.Country {
	totalWeight := g.refData.TotalWeight()
	pick := g.rng.IntRange(1, totalWeight)
	return g.refData.CountryByWeight(pick)
}

// pickCity selects a city for the given country
func (g *BusinessGenerator) pickCity(countryCode string) data.City {
	cities, ok := g.refData.GetCities(countryCode)
	if !ok || len(cities) == 0 {
		return data.City{City: "Business District", State: "", PostalPrefix: "10000"}
	}
	return cities[g.rng.IntN(len(cities))]
}

// pickSegment determines customer segment based on business type
func (g *BusinessGenerator) pickSegment(bizType BusinessType) models.CustomerSegment {
	switch bizType {
	case BusinessTypeEmployer:
		if g.rng.Probability(0.3) {
			return models.SegmentCorporate // Large employers
		}
		return models.SegmentBusiness
	case BusinessTypeMerchant:
		return models.SegmentBusiness
	case BusinessTypeUtility:
		return models.SegmentCorporate
	case BusinessTypeGovernment:
		return models.SegmentCorporate
	default:
		return models.SegmentBusiness
	}
}

// pickHomeBranch selects a home branch, preferring same country
func (g *BusinessGenerator) pickHomeBranch(countryCode string) int64 {
	if len(g.config.Branches) == 0 {
		return 1
	}

	sameCntry := make([]int64, 0)
	for _, b := range g.config.Branches {
		if b.Country.Code == countryCode {
			sameCntry = append(sameCntry, b.Branch.ID)
		}
	}

	if len(sameCntry) > 0 {
		return sameCntry[g.rng.IntN(len(sameCntry))]
	}

	return g.config.Branches[g.rng.IntN(len(g.config.Branches))].Branch.ID
}

// generateBusinessEmail creates a business email
func (g *BusinessGenerator) generateBusinessEmail(businessName, countryCode string) string {
	// Simplify business name for email
	name := strings.ToLower(businessName)
	name = strings.ReplaceAll(name, " ", "")
	name = strings.ReplaceAll(name, ".", "")
	name = strings.ReplaceAll(name, "'", "")
	name = strings.ReplaceAll(name, "&", "and")

	// Truncate if too long
	if len(name) > 20 {
		name = name[:20]
	}

	domains := []string{".com", ".net", ".biz", ".co"}
	domain := g.rng.PickString(domains)

	return fmt.Sprintf("accounts@%s%s", name, domain)
}

// generatePhone creates a phone number with country code
func (g *BusinessGenerator) generatePhone(phoneCode string) string {
	return fmt.Sprintf("+%s %s", phoneCode, g.rng.NumericString(10))
}

// generateUsername creates a unique username for business account
func (g *BusinessGenerator) generateUsername(businessName string, id int64) string {
	name := strings.ToLower(businessName)
	name = strings.ReplaceAll(name, " ", "_")
	if len(name) > 10 {
		name = name[:10]
	}
	return fmt.Sprintf("biz_%s_%d", name, id)
}

// generateBusinessAddress creates a business address
func (g *BusinessGenerator) generateBusinessAddress() string {
	streetNum := g.rng.IntRange(100, 9999)
	streets := []string{
		"Commerce Drive",
		"Business Park",
		"Corporate Boulevard",
		"Industrial Way",
		"Trade Center Road",
		"Executive Plaza",
		"Professional Drive",
		"Enterprise Lane",
		"Technology Park",
		"Financial Center",
	}
	return fmt.Sprintf("%d %s", streetNum, g.rng.PickString(streets))
}

// generateSuiteNumber creates a suite/floor number
func (g *BusinessGenerator) generateSuiteNumber() string {
	// Most businesses have suite numbers
	if g.rng.Probability(0.8) {
		suites := []string{"Suite", "Floor", "Office", "Building"}
		return fmt.Sprintf("%s %d", g.rng.PickString(suites), g.rng.IntRange(1, 50))
	}
	return ""
}

// generateIncorporationDate creates a business incorporation date
func (g *BusinessGenerator) generateIncorporationDate() time.Time {
	// Businesses are 1-30 years old
	yearsBack := g.rng.IntRange(1, 30)
	return time.Now().AddDate(-yearsBack, 0, 0)
}

// generateCreatedAt creates a customer record creation date
func (g *BusinessGenerator) generateCreatedAt() time.Time {
	daysBack := g.rng.IntRange(30, 5*365)
	return time.Now().AddDate(0, 0, -daysBack)
}

// generatePostalCode creates a postal code based on country format
func (g *BusinessGenerator) generatePostalCode(countryCode, prefix string) string {
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

// hashString creates a simple hash for a string
func hashString(s string) string {
	// Reuse the hashPassword logic from customer.go
	// For simplicity, inline a basic implementation
	sum := 0
	for _, c := range s {
		sum = sum*31 + int(c)
	}
	return fmt.Sprintf("%064x", sum)
}

// WriteBusinessesCSV writes businesses to the customers CSV file (or .csv.xz if compress=true)
// (businesses are stored in the same table as customers)
func WriteBusinessesCSV(businesses []GeneratedBusiness, outputDir string, compress bool) error {
	return writeBusinessesCSVInternal(businesses, outputDir, compress, false)
}

// WriteBusinessesCSVWithProgress writes businesses with progress reporting
func WriteBusinessesCSVWithProgress(businesses []GeneratedBusiness, outputDir string, compress bool) error {
	return writeBusinessesCSVInternal(businesses, outputDir, compress, true)
}

func writeBusinessesCSVInternal(businesses []GeneratedBusiness, outputDir string, compress, showProgress bool) error {
	headers := []string{
		"id", "first_name", "last_name", "email", "phone", "date_of_birth",
		"address_line1", "address_line2", "city", "state", "postal_code", "country",
		"timezone", "home_branch_id", "segment", "status", "activity_score",
		"username", "password_hash", "pin",
		"created_at", "updated_at",
	}

	writer, err := NewCSVWriter(CSVWriterConfig{
		OutputDir: outputDir,
		Filename:  "businesses",
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
			Total: int64(len(businesses)),
			Label: "  Businesses",
		})
	}

	for i, gb := range businesses {
		c := gb.Customer
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
		progress.Set(int64(len(businesses)))
		progress.Finish()
	}

	return writer.Close()
}

// GetEmployers returns only employer businesses (for payroll transactions)
func GetEmployers(businesses []GeneratedBusiness) []GeneratedBusiness {
	result := make([]GeneratedBusiness, 0)
	for _, b := range businesses {
		if b.BusinessType == BusinessTypeEmployer {
			result = append(result, b)
		}
	}
	return result
}

// GetMerchants returns only merchant businesses (for purchase transactions)
func GetMerchants(businesses []GeneratedBusiness) []GeneratedBusiness {
	result := make([]GeneratedBusiness, 0)
	for _, b := range businesses {
		if b.BusinessType == BusinessTypeMerchant {
			result = append(result, b)
		}
	}
	return result
}

// GetUtilities returns only utility businesses (for bill payments)
func GetUtilities(businesses []GeneratedBusiness) []GeneratedBusiness {
	result := make([]GeneratedBusiness, 0)
	for _, b := range businesses {
		if b.BusinessType == BusinessTypeUtility {
			result = append(result, b)
		}
	}
	return result
}
