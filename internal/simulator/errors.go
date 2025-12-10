package simulator

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/willfong/load-generator/internal/config"
	"github.com/willfong/load-generator/internal/utils"
)

// Error types for simulation
var (
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrAuthenticationFailed = errors.New("authentication failed")
	ErrTimeout = errors.New("operation timeout")
	ErrRateLimited = errors.New("rate limited")
	ErrAccountLocked = errors.New("account locked")
	ErrInvalidBeneficiary = errors.New("invalid beneficiary")
	ErrDailyLimitExceeded = errors.New("daily limit exceeded")
	ErrServiceUnavailable = errors.New("service temporarily unavailable")
)

// ErrorType categorizes errors for metrics and reporting
type ErrorType string

const (
	ErrorTypeAuth          ErrorType = "auth"
	ErrorTypeFunds         ErrorType = "funds"
	ErrorTypeTimeout       ErrorType = "timeout"
	ErrorTypeRateLimit     ErrorType = "rate_limit"
	ErrorTypeAccountLock   ErrorType = "account_lock"
	ErrorTypeBeneficiary   ErrorType = "beneficiary"
	ErrorTypeDailyLimit    ErrorType = "daily_limit"
	ErrorTypeService       ErrorType = "service"
	ErrorTypeDatabase      ErrorType = "database"
	ErrorTypeUnknown       ErrorType = "unknown"
)

// IsSimulatedErrorType returns true for error types that represent
// deliberate simulation behavior (failed logins, insufficient funds, etc.)
// rather than actual system problems.
func IsSimulatedErrorType(errType ErrorType) bool {
	switch errType {
	case ErrorTypeAuth, ErrorTypeFunds, ErrorTypeTimeout,
		ErrorTypeRateLimit, ErrorTypeAccountLock, ErrorTypeBeneficiary,
		ErrorTypeDailyLimit, ErrorTypeService:
		return true
	default:
		return false
	}
}

// IsInfrastructureError returns true for errors that indicate system failure
// (database issues, connection problems) rather than simulated business errors.
// Infrastructure errors should halt the simulation immediately.
func IsInfrastructureError(err error) bool {
	if err == nil {
		return false
	}
	// These are simulated/expected business errors - NOT infrastructure
	if errors.Is(err, ErrInsufficientFunds) ||
		errors.Is(err, ErrAuthenticationFailed) ||
		errors.Is(err, ErrTimeout) ||
		errors.Is(err, ErrRateLimited) ||
		errors.Is(err, ErrAccountLocked) ||
		errors.Is(err, ErrInvalidBeneficiary) ||
		errors.Is(err, ErrDailyLimitExceeded) ||
		errors.Is(err, ErrServiceUnavailable) {
		return false
	}
	// Everything else is infrastructure (database errors, connection issues, etc.)
	return true
}

// ErrorSimulator handles controlled injection of errors for realistic simulation
type ErrorSimulator struct {
	config config.SimulateConfig

	// Retry configuration
	maxRetries     int
	baseRetryDelay time.Duration
	maxRetryDelay  time.Duration

	// Statistics
	stats ErrorStats
}

// ErrorStats tracks error occurrences by type
type ErrorStats struct {
	mu sync.RWMutex

	// Counts by error type
	counts map[ErrorType]*atomic.Int64

	// Retry statistics
	totalRetries    atomic.Int64
	successfulRetries atomic.Int64
	exhaustedRetries  atomic.Int64

	// Timing
	startTime time.Time
}

// NewErrorSimulator creates an error simulator with configuration
func NewErrorSimulator(cfg config.SimulateConfig) *ErrorSimulator {
	return &ErrorSimulator{
		config:         cfg,
		maxRetries:     3,
		baseRetryDelay: 100 * time.Millisecond,
		maxRetryDelay:  2 * time.Second,
		stats: ErrorStats{
			counts:    make(map[ErrorType]*atomic.Int64),
			startTime: time.Now(),
		},
	}
}

// ShouldSimulateLoginFailure returns true if a login should fail
func (e *ErrorSimulator) ShouldSimulateLoginFailure(rng *utils.Random) bool {
	return rng.Float64() < e.config.FailedLoginRate
}

// ShouldSimulateInsufficientFunds returns true if an insufficient funds error should occur
func (e *ErrorSimulator) ShouldSimulateInsufficientFunds(rng *utils.Random) bool {
	return rng.Float64() < e.config.InsufficientFundsRate
}

// ShouldSimulateTimeout returns true if a timeout should be simulated
func (e *ErrorSimulator) ShouldSimulateTimeout(rng *utils.Random) bool {
	return rng.Float64() < e.config.TimeoutRate
}

// SimulateTimeout artificially delays an operation to trigger a timeout
func (e *ErrorSimulator) SimulateTimeout(ctx context.Context, rng *utils.Random) error {
	if !e.ShouldSimulateTimeout(rng) {
		return nil
	}

	// Simulate a delay that exceeds typical timeout thresholds
	delay := time.Duration(5+rng.IntN(10)) * time.Second

	select {
	case <-time.After(delay):
		e.RecordError(ErrorTypeTimeout)
		return ErrTimeout
	case <-ctx.Done():
		e.RecordError(ErrorTypeTimeout)
		return ctx.Err()
	}
}

// RecordError increments the counter for an error type
func (e *ErrorSimulator) RecordError(errType ErrorType) {
	e.stats.mu.Lock()
	defer e.stats.mu.Unlock()

	if _, exists := e.stats.counts[errType]; !exists {
		e.stats.counts[errType] = &atomic.Int64{}
	}
	e.stats.counts[errType].Add(1)
}

// GetErrorCount returns the count for a specific error type
func (e *ErrorSimulator) GetErrorCount(errType ErrorType) int64 {
	e.stats.mu.RLock()
	defer e.stats.mu.RUnlock()

	if counter, exists := e.stats.counts[errType]; exists {
		return counter.Load()
	}
	return 0
}

// GetAllErrorCounts returns counts for all error types
func (e *ErrorSimulator) GetAllErrorCounts() map[ErrorType]int64 {
	e.stats.mu.RLock()
	defer e.stats.mu.RUnlock()

	result := make(map[ErrorType]int64)
	for errType, counter := range e.stats.counts {
		result[errType] = counter.Load()
	}
	return result
}

// RetryConfig holds configuration for a retry operation
type RetryConfig struct {
	MaxRetries     int
	BaseDelay      time.Duration
	MaxDelay       time.Duration
	Jitter         bool
	RetryableCheck func(error) bool
}

// DefaultRetryConfig returns sensible default retry configuration
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 3,
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   2 * time.Second,
		Jitter:     true,
		RetryableCheck: func(err error) bool {
			// By default, only retry timeout and service errors
			return errors.Is(err, ErrTimeout) ||
				   errors.Is(err, ErrServiceUnavailable) ||
				   errors.Is(err, context.DeadlineExceeded)
		},
	}
}

// RetryableOperation wraps an operation with retry logic
type RetryableOperation struct {
	config  RetryConfig
	errSim  *ErrorSimulator
	rng     *utils.Random
}

// NewRetryableOperation creates a new retryable operation wrapper
func NewRetryableOperation(errSim *ErrorSimulator, rng *utils.Random, cfg RetryConfig) *RetryableOperation {
	return &RetryableOperation{
		config: cfg,
		errSim: errSim,
		rng:    rng,
	}
}

// Execute runs the operation with retries
func (r *RetryableOperation) Execute(ctx context.Context, operation func(ctx context.Context) error) error {
	var lastErr error

	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		// Check context before each attempt
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Execute the operation
		err := operation(ctx)
		if err == nil {
			if attempt > 0 {
				r.errSim.stats.successfulRetries.Add(1)
			}
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !r.config.RetryableCheck(err) {
			return err
		}

		// Don't retry if we've exhausted attempts
		if attempt >= r.config.MaxRetries {
			r.errSim.stats.exhaustedRetries.Add(1)
			break
		}

		r.errSim.stats.totalRetries.Add(1)

		// Calculate backoff delay with exponential backoff
		delay := r.calculateDelay(attempt)

		// Wait before retry
		select {
		case <-time.After(delay):
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	return lastErr
}

// calculateDelay computes the delay before the next retry attempt
func (r *RetryableOperation) calculateDelay(attempt int) time.Duration {
	// Exponential backoff: baseDelay * 2^attempt
	delay := r.config.BaseDelay * time.Duration(1<<uint(attempt))

	// Cap at max delay
	if delay > r.config.MaxDelay {
		delay = r.config.MaxDelay
	}

	// Add jitter (0-25% of delay) to prevent thundering herd
	if r.config.Jitter && r.rng != nil {
		jitter := time.Duration(r.rng.Int64N(int64(delay) / 4))
		delay += jitter
	}

	return delay
}

// GetRetryStats returns statistics about retry operations
func (e *ErrorSimulator) GetRetryStats() RetryStats {
	return RetryStats{
		TotalRetries:      e.stats.totalRetries.Load(),
		SuccessfulRetries: e.stats.successfulRetries.Load(),
		ExhaustedRetries:  e.stats.exhaustedRetries.Load(),
	}
}

// RetryStats holds retry operation statistics
type RetryStats struct {
	TotalRetries      int64
	SuccessfulRetries int64
	ExhaustedRetries  int64
}

// ClassifyError determines the error type from an error
func ClassifyError(err error) ErrorType {
	if err == nil {
		return ErrorTypeUnknown
	}

	switch {
	case errors.Is(err, ErrAuthenticationFailed):
		return ErrorTypeAuth
	case errors.Is(err, ErrInsufficientFunds):
		return ErrorTypeFunds
	case errors.Is(err, ErrTimeout), errors.Is(err, context.DeadlineExceeded):
		return ErrorTypeTimeout
	case errors.Is(err, ErrRateLimited):
		return ErrorTypeRateLimit
	case errors.Is(err, ErrAccountLocked):
		return ErrorTypeAccountLock
	case errors.Is(err, ErrInvalidBeneficiary):
		return ErrorTypeBeneficiary
	case errors.Is(err, ErrDailyLimitExceeded):
		return ErrorTypeDailyLimit
	case errors.Is(err, ErrServiceUnavailable):
		return ErrorTypeService
	default:
		// Check for common database error patterns
		errStr := err.Error()
		if contains(errStr, "connection") || contains(errStr, "database") || contains(errStr, "sql") {
			return ErrorTypeDatabase
		}
		return ErrorTypeUnknown
	}
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		containsIgnoreCase(s, substr))
}

func containsIgnoreCase(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFoldAt(s, substr, i) {
			return true
		}
	}
	return false
}

func equalFoldAt(s, substr string, start int) bool {
	for i := 0; i < len(substr); i++ {
		c1 := s[start+i]
		c2 := substr[i]
		if c1 != c2 && toLower(c1) != toLower(c2) {
			return false
		}
	}
	return true
}

func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}

// ErrorSummary provides a summary of all errors for reporting
type ErrorSummary struct {
	TotalErrors    int64
	ErrorsByType   map[ErrorType]int64
	ErrorRate      float64 // Errors per second
	RetryStats     RetryStats
	TopErrors      []ErrorTypeStat // Top 5 error types by count
}

// ErrorTypeStat holds count and rate for an error type
type ErrorTypeStat struct {
	Type  ErrorType
	Count int64
	Rate  float64 // Per second
}

// GetErrorSummary returns a comprehensive error summary
func (e *ErrorSimulator) GetErrorSummary() ErrorSummary {
	elapsed := time.Since(e.stats.startTime).Seconds()
	if elapsed < 1 {
		elapsed = 1
	}

	counts := e.GetAllErrorCounts()
	var total int64
	for _, count := range counts {
		total += count
	}

	// Build top errors list
	topErrors := make([]ErrorTypeStat, 0, len(counts))
	for errType, count := range counts {
		topErrors = append(topErrors, ErrorTypeStat{
			Type:  errType,
			Count: count,
			Rate:  float64(count) / elapsed,
		})
	}

	// Sort by count (simple bubble sort for small slice)
	for i := 0; i < len(topErrors); i++ {
		for j := i + 1; j < len(topErrors); j++ {
			if topErrors[j].Count > topErrors[i].Count {
				topErrors[i], topErrors[j] = topErrors[j], topErrors[i]
			}
		}
	}

	// Keep only top 5
	if len(topErrors) > 5 {
		topErrors = topErrors[:5]
	}

	return ErrorSummary{
		TotalErrors:  total,
		ErrorsByType: counts,
		ErrorRate:    float64(total) / elapsed,
		RetryStats:   e.GetRetryStats(),
		TopErrors:    topErrors,
	}
}
