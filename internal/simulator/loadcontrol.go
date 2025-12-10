package simulator

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/willfong/load-generator/internal/config"
)

// LoadPhase represents the current phase of load control
type LoadPhase int

const (
	PhaseIdle LoadPhase = iota
	PhaseRampUp
	PhaseSteadyState
	PhaseRampDown
	PhaseComplete
)

func (p LoadPhase) String() string {
	switch p {
	case PhaseIdle:
		return "IDLE"
	case PhaseRampUp:
		return "RAMP_UP"
	case PhaseSteadyState:
		return "STEADY"
	case PhaseRampDown:
		return "RAMP_DOWN"
	case PhaseComplete:
		return "COMPLETE"
	default:
		return "UNKNOWN"
	}
}

// LoadController manages gradual load ramp-up and ramp-down.
// This prevents sudden load spikes that could overwhelm the system
// and allows for more realistic startup/shutdown behavior.
type LoadController struct {
	config      config.SimulateConfig
	targetLoad  int     // Target number of sessions
	currentLoad atomic.Int32 // Current active sessions (thread-safe)

	phase      LoadPhase
	phaseMu    sync.RWMutex
	phaseStart time.Time

	// Ramp state
	rampStep     int
	rampStepSize int

	// Callbacks
	onPhaseChange func(LoadPhase)
	onLoadChange  func(int, int) // current, target

	// Statistics
	stats LoadControlStats
}

// LoadControlStats tracks load control statistics
type LoadControlStats struct {
	RampUpStartTime   time.Time
	RampUpEndTime     time.Time
	SteadyStateStart  time.Time
	RampDownStartTime time.Time
	RampDownEndTime   time.Time
	MaxLoadReached    int
	TimeInSteadyState time.Duration
}

// NewLoadController creates a new load controller
func NewLoadController(cfg config.SimulateConfig) *LoadController {
	// Guard against divide by zero - default to 1 step if RampSteps is 0
	rampSteps := cfg.RampSteps
	if rampSteps < 1 {
		rampSteps = 1
	}

	return &LoadController{
		config:       cfg,
		targetLoad:   cfg.NumSessions,
		phase:        PhaseIdle,
		rampStepSize: cfg.NumSessions / rampSteps,
	}
}

// GetCurrentLoad returns the current number of sessions that should be running
func (lc *LoadController) GetCurrentLoad() int {
	return int(lc.currentLoad.Load())
}

// GetTargetLoad returns the target number of sessions
func (lc *LoadController) GetTargetLoad() int {
	return lc.targetLoad
}

// GetPhase returns the current load phase
func (lc *LoadController) GetPhase() LoadPhase {
	lc.phaseMu.RLock()
	defer lc.phaseMu.RUnlock()
	return lc.phase
}

// GetLoadPercentage returns current load as percentage of target
func (lc *LoadController) GetLoadPercentage() float64 {
	if lc.targetLoad == 0 {
		return 0
	}
	return float64(lc.GetCurrentLoad()) / float64(lc.targetLoad) * 100
}

// SetOnPhaseChange sets a callback for phase changes
func (lc *LoadController) SetOnPhaseChange(fn func(LoadPhase)) {
	lc.onPhaseChange = fn
}

// SetOnLoadChange sets a callback for load changes
func (lc *LoadController) SetOnLoadChange(fn func(int, int)) {
	lc.onLoadChange = fn
}

// Start begins the load control sequence
func (lc *LoadController) Start() {
	if !lc.config.EnableRamp {
		// No ramping - immediately set to full load
		lc.setPhase(PhaseSteadyState)
		lc.currentLoad.Store(int32(lc.targetLoad))
		lc.stats.SteadyStateStart = time.Now()
		lc.notifyLoadChange()
		return
	}

	// Start with ramp-up
	lc.setPhase(PhaseRampUp)
	lc.stats.RampUpStartTime = time.Now()
	lc.rampStep = 0
	lc.currentLoad.Store(0)

	// Schedule first step immediately
	lc.executeRampStep()
}

// StartRampDown initiates graceful load reduction
func (lc *LoadController) StartRampDown() {
	lc.phaseMu.Lock()
	if lc.phase == PhaseRampDown || lc.phase == PhaseComplete {
		lc.phaseMu.Unlock()
		return
	}
	lc.phaseMu.Unlock()

	// Record steady state duration
	if lc.stats.SteadyStateStart.IsZero() {
		lc.stats.SteadyStateStart = lc.stats.RampUpEndTime
	}
	lc.stats.TimeInSteadyState = time.Since(lc.stats.SteadyStateStart)

	lc.setPhase(PhaseRampDown)
	lc.stats.RampDownStartTime = time.Now()
	lc.rampStep = lc.config.RampSteps
	lc.stats.MaxLoadReached = lc.GetCurrentLoad()
}

// Run executes the load control loop
func (lc *LoadController) Run(ctx context.Context) {
	lc.Start()

	if !lc.config.EnableRamp {
		// Just wait for context cancellation
		<-ctx.Done()
		lc.setPhase(PhaseComplete)
		return
	}

	// Calculate step interval
	rampUpStepInterval := lc.config.RampUpDuration / time.Duration(lc.config.RampSteps)
	rampDownStepInterval := lc.config.RampDownDuration / time.Duration(lc.config.RampSteps)

	ticker := time.NewTicker(rampUpStepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Context cancelled - start ramp down if not already
			if lc.GetPhase() != PhaseRampDown && lc.GetPhase() != PhaseComplete {
				lc.StartRampDown()
				// Continue with ramp-down loop
				ticker.Reset(rampDownStepInterval)
				continue
			}
			return

		case <-ticker.C:
			phase := lc.GetPhase()
			switch phase {
			case PhaseRampUp:
				lc.executeRampStep()
				if lc.rampStep >= lc.config.RampSteps {
					lc.setPhase(PhaseSteadyState)
					lc.stats.RampUpEndTime = time.Now()
					lc.stats.SteadyStateStart = time.Now()
				}

			case PhaseRampDown:
				lc.executeRampDownStep()
				if lc.rampStep <= 0 {
					lc.setPhase(PhaseComplete)
					lc.stats.RampDownEndTime = time.Now()
					return
				}

			case PhaseSteadyState:
				// Stay steady - just update the ticker for potential ramp-down
				continue

			case PhaseComplete:
				return
			}
		}
	}
}

// executeRampStep increases load by one step
func (lc *LoadController) executeRampStep() {
	lc.rampStep++

	// Calculate new load level
	newLoad := lc.rampStep * lc.rampStepSize
	if newLoad > lc.targetLoad {
		newLoad = lc.targetLoad
	}

	lc.currentLoad.Store(int32(newLoad))
	lc.notifyLoadChange()
}

// executeRampDownStep decreases load by one step
func (lc *LoadController) executeRampDownStep() {
	lc.rampStep--

	// Calculate new load level
	newLoad := lc.rampStep * lc.rampStepSize
	if newLoad < 0 {
		newLoad = 0
	}

	lc.currentLoad.Store(int32(newLoad))
	lc.notifyLoadChange()
}

// setPhase updates the current phase and notifies listeners
func (lc *LoadController) setPhase(phase LoadPhase) {
	lc.phaseMu.Lock()
	oldPhase := lc.phase
	lc.phase = phase
	lc.phaseStart = time.Now()
	lc.phaseMu.Unlock()

	if oldPhase != phase && lc.onPhaseChange != nil {
		lc.onPhaseChange(phase)
	}
}

// notifyLoadChange calls the load change callback if set
func (lc *LoadController) notifyLoadChange() {
	if lc.onLoadChange != nil {
		lc.onLoadChange(lc.GetCurrentLoad(), lc.targetLoad)
	}
}

// GetStats returns load control statistics
func (lc *LoadController) GetStats() LoadControlStats {
	return lc.stats
}

// GetPhaseProgress returns progress through the current phase (0.0-1.0)
func (lc *LoadController) GetPhaseProgress() float64 {
	lc.phaseMu.RLock()
	phase := lc.phase
	start := lc.phaseStart
	lc.phaseMu.RUnlock()

	var duration time.Duration
	switch phase {
	case PhaseRampUp:
		duration = lc.config.RampUpDuration
	case PhaseRampDown:
		duration = lc.config.RampDownDuration
	default:
		return 1.0
	}

	if duration == 0 {
		return 1.0
	}

	elapsed := time.Since(start)
	progress := float64(elapsed) / float64(duration)
	if progress > 1.0 {
		progress = 1.0
	}
	return progress
}

// StatusString returns a human-readable status
func (lc *LoadController) StatusString() string {
	phase := lc.GetPhase()
	current := lc.GetCurrentLoad()
	target := lc.targetLoad
	percentage := lc.GetLoadPercentage()

	switch phase {
	case PhaseIdle:
		return "Idle"
	case PhaseRampUp:
		return fmt.Sprintf("Ramping up: %d/%d (%.0f%%)", current, target, percentage)
	case PhaseSteadyState:
		return fmt.Sprintf("Steady state: %d sessions", current)
	case PhaseRampDown:
		return fmt.Sprintf("Ramping down: %d/%d (%.0f%%)", current, target, percentage)
	case PhaseComplete:
		return "Complete"
	default:
		return "Unknown"
	}
}

// ShouldSpawnSession returns true if a new session should be spawned
// based on current load level
func (lc *LoadController) ShouldSpawnSession(currentActiveSessions int) bool {
	targetLoad := lc.GetCurrentLoad()
	return currentActiveSessions < targetLoad
}

// ShouldTerminateSession returns true if sessions should be terminated
// during ramp-down
func (lc *LoadController) ShouldTerminateSession(currentActiveSessions int) bool {
	if lc.GetPhase() != PhaseRampDown {
		return false
	}
	targetLoad := lc.GetCurrentLoad()
	return currentActiveSessions > targetLoad
}
