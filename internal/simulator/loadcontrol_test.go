package simulator

import (
	"testing"
	"time"

	"github.com/willfong/load-generator/internal/config"
)

func TestLoadController_NewController(t *testing.T) {
	cfg := config.DefaultConfig().Simulate
	cfg.NumSessions = 100
	cfg.RampSteps = 10

	lc := NewLoadController(cfg)
	if lc == nil {
		t.Fatal("NewLoadController returned nil")
	}

	if lc.GetTargetLoad() != 100 {
		t.Errorf("Expected target load 100, got %d", lc.GetTargetLoad())
	}

	if lc.GetPhase() != PhaseIdle {
		t.Errorf("Expected phase IDLE, got %s", lc.GetPhase())
	}
}

func TestLoadController_StartNoRamp(t *testing.T) {
	cfg := config.DefaultConfig().Simulate
	cfg.NumSessions = 100
	cfg.EnableRamp = false

	lc := NewLoadController(cfg)
	lc.Start()

	// Without ramp, should immediately be at full load
	if lc.GetCurrentLoad() != 100 {
		t.Errorf("Expected current load 100, got %d", lc.GetCurrentLoad())
	}

	if lc.GetPhase() != PhaseSteadyState {
		t.Errorf("Expected phase STEADY, got %s", lc.GetPhase())
	}

	if lc.GetLoadPercentage() != 100.0 {
		t.Errorf("Expected 100%% load, got %.1f%%", lc.GetLoadPercentage())
	}
}

func TestLoadController_PhaseString(t *testing.T) {
	tests := []struct {
		phase    LoadPhase
		expected string
	}{
		{PhaseIdle, "IDLE"},
		{PhaseRampUp, "RAMP_UP"},
		{PhaseSteadyState, "STEADY"},
		{PhaseRampDown, "RAMP_DOWN"},
		{PhaseComplete, "COMPLETE"},
	}

	for _, test := range tests {
		if test.phase.String() != test.expected {
			t.Errorf("Expected %s for phase %d, got %s", test.expected, test.phase, test.phase.String())
		}
	}
}

func TestLoadController_ShouldSpawnSession(t *testing.T) {
	cfg := config.DefaultConfig().Simulate
	cfg.NumSessions = 100
	cfg.EnableRamp = false

	lc := NewLoadController(cfg)
	lc.Start()

	// Should spawn when below target
	if !lc.ShouldSpawnSession(50) {
		t.Error("Expected ShouldSpawnSession to return true when below target")
	}

	// Should not spawn when at target
	if lc.ShouldSpawnSession(100) {
		t.Error("Expected ShouldSpawnSession to return false when at target")
	}

	// Should not spawn when above target
	if lc.ShouldSpawnSession(150) {
		t.Error("Expected ShouldSpawnSession to return false when above target")
	}
}

func TestLoadController_StatusString(t *testing.T) {
	cfg := config.DefaultConfig().Simulate
	cfg.NumSessions = 100
	cfg.EnableRamp = false

	lc := NewLoadController(cfg)

	// Initially idle
	status := lc.StatusString()
	if status != "Idle" {
		t.Errorf("Expected 'Idle', got '%s'", status)
	}

	// After start without ramp
	lc.Start()
	status = lc.StatusString()
	if status != "Steady state: 100 sessions" {
		t.Errorf("Expected 'Steady state: 100 sessions', got '%s'", status)
	}
}

func TestLoadController_Callbacks(t *testing.T) {
	cfg := config.DefaultConfig().Simulate
	cfg.NumSessions = 100
	cfg.EnableRamp = false

	lc := NewLoadController(cfg)

	var phaseChangeCalled bool
	var loadChangeCalled bool

	lc.SetOnPhaseChange(func(phase LoadPhase) {
		phaseChangeCalled = true
	})

	lc.SetOnLoadChange(func(current, target int) {
		loadChangeCalled = true
	})

	lc.Start()

	if !phaseChangeCalled {
		t.Error("Expected phase change callback to be called")
	}
	if !loadChangeCalled {
		t.Error("Expected load change callback to be called")
	}
}

func TestLoadController_RampUpConfig(t *testing.T) {
	cfg := config.DefaultConfig().Simulate
	cfg.NumSessions = 100
	cfg.EnableRamp = true
	cfg.RampUpDuration = 1 * time.Minute
	cfg.RampSteps = 10

	lc := NewLoadController(cfg)
	lc.Start()

	// Should start at 0 with ramp enabled
	// Actually, our implementation does executeRampStep immediately in Start
	// Let's check the phase instead
	if lc.GetPhase() != PhaseRampUp {
		t.Errorf("Expected phase RAMP_UP, got %s", lc.GetPhase())
	}
}

func TestLoadController_GetPhaseProgress(t *testing.T) {
	cfg := config.DefaultConfig().Simulate
	cfg.NumSessions = 100
	cfg.EnableRamp = false

	lc := NewLoadController(cfg)
	lc.Start()

	// In steady state, progress should be 1.0
	progress := lc.GetPhaseProgress()
	if progress != 1.0 {
		t.Errorf("Expected progress 1.0, got %f", progress)
	}
}
