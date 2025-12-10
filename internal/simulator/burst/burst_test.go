package burst

import (
	"testing"
	"time"
)

func TestBurstManager_NewManager(t *testing.T) {
	mgr := NewManager()
	if mgr == nil {
		t.Fatal("NewManager returned nil")
	}

	// Should have no active bursts initially
	active := mgr.GetActiveBursts()
	if len(active) != 0 {
		t.Errorf("Expected 0 active bursts, got %d", len(active))
	}

	// Multiplier should be 1.0 with no bursts
	mult := mgr.GetActiveMultiplier()
	if mult != 1.0 {
		t.Errorf("Expected multiplier 1.0, got %f", mult)
	}
}

func TestBurstManager_ManualBurst(t *testing.T) {
	mgr := NewManager()

	// Trigger a manual burst
	event := mgr.TriggerManualBurst(2.5, 5*time.Minute, 10)

	if event == nil {
		t.Fatal("TriggerManualBurst returned nil")
	}
	if event.Type != BurstTypeManual {
		t.Errorf("Expected type %s, got %s", BurstTypeManual, event.Type)
	}
	if event.Multiplier != 2.5 {
		t.Errorf("Expected multiplier 2.5, got %f", event.Multiplier)
	}
	if event.SessionIncrease != 10 {
		t.Errorf("Expected 10 extra sessions, got %d", event.SessionIncrease)
	}

	// Should now have 1 active burst
	active := mgr.GetActiveBursts()
	if len(active) != 1 {
		t.Errorf("Expected 1 active burst, got %d", len(active))
	}

	// Multiplier should reflect the burst
	mult := mgr.GetActiveMultiplier()
	if mult != 2.5 {
		t.Errorf("Expected multiplier 2.5, got %f", mult)
	}

	// Check stats
	stats := mgr.GetStats()
	if stats.TotalBurstsTriggered != 1 {
		t.Errorf("Expected 1 burst triggered, got %d", stats.TotalBurstsTriggered)
	}
	if stats.BurstsByType[BurstTypeManual] != 1 {
		t.Errorf("Expected 1 manual burst, got %d", stats.BurstsByType[BurstTypeManual])
	}
}

func TestBurstEvent_IsActive(t *testing.T) {
	now := time.Now()

	// Active burst
	active := &BurstEvent{
		StartTime: now.Add(-1 * time.Minute),
		EndTime:   now.Add(5 * time.Minute),
	}
	if !active.IsActive() {
		t.Error("Expected burst to be active")
	}

	// Expired burst
	expired := &BurstEvent{
		StartTime: now.Add(-10 * time.Minute),
		EndTime:   now.Add(-5 * time.Minute),
	}
	if expired.IsActive() {
		t.Error("Expected burst to be expired")
	}

	// Future burst (not started yet)
	future := &BurstEvent{
		StartTime: now.Add(5 * time.Minute),
		EndTime:   now.Add(10 * time.Minute),
	}
	if future.IsActive() {
		t.Error("Expected burst to not be active yet")
	}
}

func TestLunchBurst_Type(t *testing.T) {
	lb := NewLunchBurst(BurstConfig{Enabled: true, Multiplier: 1.5})
	if lb.Type() != BurstTypeLunch {
		t.Errorf("Expected type %s, got %s", BurstTypeLunch, lb.Type())
	}
}

func TestPayrollBurst_Type(t *testing.T) {
	pb := NewPayrollBurst(BurstConfig{Enabled: true, Multiplier: 3.0})
	if pb.Type() != BurstTypePayroll {
		t.Errorf("Expected type %s, got %s", BurstTypePayroll, pb.Type())
	}
}

func TestPayrollBurst_GetPayrollDays(t *testing.T) {
	pb := NewPayrollBurst(BurstConfig{Enabled: true, Multiplier: 3.0})
	days := pb.GetPayrollDays()

	// Should have default payroll days
	if len(days) == 0 {
		t.Error("Expected payroll days to be set")
	}

	// Should include day 25
	found := false
	for _, d := range days {
		if d == 25 {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected day 25 to be in payroll days")
	}
}

func TestPayrollBurst_SetPayrollDays(t *testing.T) {
	pb := NewPayrollBurst(BurstConfig{Enabled: true, Multiplier: 3.0})
	pb.SetPayrollDays([]int{1, 15})

	days := pb.GetPayrollDays()
	if len(days) != 2 {
		t.Errorf("Expected 2 payroll days, got %d", len(days))
	}
}

func TestRandomBurst_Type(t *testing.T) {
	rb := NewRandomBurst(BurstConfig{Enabled: true, Probability: 0.1}, 42)
	if rb.Type() != BurstTypeRandom {
		t.Errorf("Expected type %s, got %s", BurstTypeRandom, rb.Type())
	}
}

func TestRandomBurst_ForceTrigger(t *testing.T) {
	rb := NewRandomBurst(BurstConfig{Enabled: true, Probability: 0.1}, 42)

	// Force trigger should always produce an event
	event := rb.ForceTrigger()
	if event == nil {
		t.Fatal("ForceTrigger returned nil")
	}
	if event.Type != BurstTypeRandom {
		t.Errorf("Expected type %s, got %s", BurstTypeRandom, event.Type)
	}
}

func TestRandomBurst_Disabled(t *testing.T) {
	rb := NewRandomBurst(BurstConfig{Enabled: false}, 42)

	// CheckBurst should return nil when disabled
	event := rb.CheckBurst("America/New_York")
	if event != nil {
		t.Error("Expected nil event when disabled")
	}

	// ForceTrigger should also return nil when disabled
	event = rb.ForceTrigger()
	if event != nil {
		t.Error("Expected nil event for ForceTrigger when disabled")
	}
}

func TestRandomBurst_Stats(t *testing.T) {
	rb := NewRandomBurst(BurstConfig{Enabled: true, Probability: 0.1}, 42)

	stats := rb.Stats()
	if stats.IsOnCooldown {
		t.Error("Expected no cooldown initially")
	}

	// After force trigger, should be on cooldown
	rb.ForceTrigger()
	stats = rb.Stats()
	if !stats.IsOnCooldown {
		t.Error("Expected cooldown after trigger")
	}
	if stats.CooldownRemaining <= 0 {
		t.Error("Expected positive cooldown remaining")
	}
}

func TestBurstManager_MultipleMultipliers(t *testing.T) {
	mgr := NewManager()

	// Trigger two bursts
	mgr.TriggerManualBurst(2.0, 5*time.Minute, 5)
	mgr.TriggerManualBurst(1.5, 5*time.Minute, 3)

	// Multipliers should compound
	mult := mgr.GetActiveMultiplier()
	expected := 2.0 * 1.5
	if mult != expected {
		t.Errorf("Expected multiplier %f, got %f", expected, mult)
	}

	// Should have 2 active bursts
	active := mgr.GetActiveBursts()
	if len(active) != 2 {
		t.Errorf("Expected 2 active bursts, got %d", len(active))
	}

	// Extra sessions should be summed
	extra := mgr.GetExtraSessionCount()
	if extra != 8 { // 5 + 3
		t.Errorf("Expected 8 extra sessions, got %d", extra)
	}
}
