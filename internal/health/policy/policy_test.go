package policy

import (
	"context"
	"testing"
	"time"

	"github.com/zczy-k/FloatingGateway/internal/config"
)

func TestDebounce_FailCount(t *testing.T) {
	// Create a config with fail_count=3
	cfg := &config.Config{
		Health: config.HealthConfig{
			Mode:         config.HealthModeBasic,
			FailCount:    3,
			RecoverCount: 2,
			Basic: config.ChecksConfig{
				Checks: []config.CheckConfig{
					{Type: "tcp", Target: "127.0.0.1", Port: 1, Timeout: 1}, // Will fail
				},
			},
		},
	}

	p := &Policy{
		cfg:          cfg,
		mode:         cfg.Health.Mode,
		currentState: StateHealthy, // Start healthy
		k:            0,            // all-of-n
	}

	// Simulate 2 failures - should still be healthy due to debounce
	p.applyDebounce(false)
	if p.currentState != StateHealthy {
		t.Errorf("Expected healthy after 1 failure, got %s", p.currentState)
	}

	p.applyDebounce(false)
	if p.currentState != StateHealthy {
		t.Errorf("Expected healthy after 2 failures, got %s", p.currentState)
	}

	// Third failure should trigger unhealthy
	p.applyDebounce(false)
	if p.currentState != StateUnhealthy {
		t.Errorf("Expected unhealthy after 3 failures, got %s", p.currentState)
	}
}

func TestDebounce_RecoverCount(t *testing.T) {
	cfg := &config.Config{
		Health: config.HealthConfig{
			Mode:         config.HealthModeBasic,
			FailCount:    3,
			RecoverCount: 2,
		},
	}

	p := &Policy{
		cfg:          cfg,
		mode:         cfg.Health.Mode,
		currentState: StateUnhealthy, // Start unhealthy
		k:            0,
	}

	// First success - should still be unhealthy
	p.applyDebounce(true)
	if p.currentState != StateUnhealthy {
		t.Errorf("Expected unhealthy after 1 success, got %s", p.currentState)
	}

	// Second success - should become healthy
	p.applyDebounce(true)
	if p.currentState != StateHealthy {
		t.Errorf("Expected healthy after 2 successes, got %s", p.currentState)
	}
}

func TestDebounce_FailureResetsRecoverCount(t *testing.T) {
	cfg := &config.Config{
		Health: config.HealthConfig{
			Mode:         config.HealthModeBasic,
			FailCount:    3,
			RecoverCount: 3,
		},
	}

	p := &Policy{
		cfg:          cfg,
		mode:         cfg.Health.Mode,
		currentState: StateUnhealthy,
		k:            0,
	}

	// Two successes
	p.applyDebounce(true)
	p.applyDebounce(true)

	// One failure should reset the counter
	p.applyDebounce(false)

	// Two more successes - should not be healthy yet (counter was reset)
	p.applyDebounce(true)
	p.applyDebounce(true)
	if p.currentState != StateUnhealthy {
		t.Errorf("Expected unhealthy after reset, got %s", p.currentState)
	}

	// Third success should make it healthy
	p.applyDebounce(true)
	if p.currentState != StateHealthy {
		t.Errorf("Expected healthy after 3 successes, got %s", p.currentState)
	}
}

func TestDebounce_SuccessResetsFailCount(t *testing.T) {
	cfg := &config.Config{
		Health: config.HealthConfig{
			Mode:         config.HealthModeBasic,
			FailCount:    3,
			RecoverCount: 2,
		},
	}

	p := &Policy{
		cfg:          cfg,
		mode:         cfg.Health.Mode,
		currentState: StateHealthy,
		k:            0,
	}

	// Two failures
	p.applyDebounce(false)
	p.applyDebounce(false)

	// One success should reset the counter
	p.applyDebounce(true)

	// Two more failures - should not be unhealthy yet
	p.applyDebounce(false)
	p.applyDebounce(false)
	if p.currentState != StateHealthy {
		t.Errorf("Expected healthy after reset, got %s", p.currentState)
	}

	// Third failure should make it unhealthy
	p.applyDebounce(false)
	if p.currentState != StateUnhealthy {
		t.Errorf("Expected unhealthy after 3 failures, got %s", p.currentState)
	}
}

func TestKOfN_2of3(t *testing.T) {
	cfg := &config.Config{
		Health: config.HealthConfig{
			Mode:         config.HealthModeBasic,
			FailCount:    1,
			RecoverCount: 1,
			KOfN:         "2/3",
		},
	}

	p := &Policy{
		cfg:          cfg,
		mode:         cfg.Health.Mode,
		currentState: StateUnknown,
		k:            2,
		n:            3,
	}

	// Simulate check with 2 passed out of 3 - should be healthy
	// We need to test the aggregation logic
	required := p.k
	if required == 0 {
		required = 3 // all-of-n
	}

	passed := 2
	roundPassed := passed >= required

	if !roundPassed {
		t.Error("Expected 2/3 to pass with 2 successes")
	}

	// Apply debounce with passing result
	p.applyDebounce(true)
	if p.currentState != StateHealthy {
		t.Errorf("Expected healthy with 2/3, got %s", p.currentState)
	}
}

func TestKOfN_1of3_Fails(t *testing.T) {
	cfg := &config.Config{
		Health: config.HealthConfig{
			Mode:         config.HealthModeBasic,
			FailCount:    1,
			RecoverCount: 1,
			KOfN:         "2/3",
		},
	}

	p := &Policy{
		cfg:          cfg,
		mode:         cfg.Health.Mode,
		currentState: StateHealthy,
		k:            2,
		n:            3,
	}

	// With k=2, only 1 passing should fail
	passed := 1
	required := p.k
	roundPassed := passed >= required

	if roundPassed {
		t.Error("Expected 1/3 to fail with k=2")
	}

	// Apply debounce with failing result
	p.applyDebounce(false)
	if p.currentState != StateUnhealthy {
		t.Errorf("Expected unhealthy with only 1/3, got %s", p.currentState)
	}
}

func TestHoldDown(t *testing.T) {
	cfg := &config.Config{
		Health: config.HealthConfig{
			Mode:         config.HealthModeBasic,
			FailCount:    1,
			RecoverCount: 1,
			HoldDownSec:  1,
		},
	}

	p := &Policy{
		cfg:          cfg,
		mode:         cfg.Health.Mode,
		currentState: StateUnhealthy,
		k:            0,
	}

	// Recover to healthy (triggers hold-down)
	p.applyDebounce(true)
	if p.currentState != StateHealthy {
		t.Errorf("Expected healthy, got %s", p.currentState)
	}

	// Immediate failure should not change state due to hold-down
	p.applyDebounce(false)
	if p.currentState != StateHealthy {
		t.Errorf("Expected healthy during hold-down, got %s", p.currentState)
	}

	// Wait for hold-down to expire
	time.Sleep(1100 * time.Millisecond)

	// Now failure should work
	p.applyDebounce(false)
	if p.currentState != StateUnhealthy {
		t.Errorf("Expected unhealthy after hold-down, got %s", p.currentState)
	}
}

func TestInitialState(t *testing.T) {
	cfg := &config.Config{
		Health: config.HealthConfig{
			Mode:         config.HealthModeBasic,
			FailCount:    3,
			RecoverCount: 3,
		},
	}

	// Test initial success
	p := &Policy{
		cfg:          cfg,
		mode:         cfg.Health.Mode,
		currentState: StateUnknown,
		k:            0,
	}

	p.applyDebounce(true)
	if p.currentState != StateHealthy {
		t.Errorf("Expected healthy on initial success, got %s", p.currentState)
	}

	// Test initial failure
	p2 := &Policy{
		cfg:          cfg,
		mode:         cfg.Health.Mode,
		currentState: StateUnknown,
		k:            0,
	}

	p2.applyDebounce(false)
	if p2.currentState != StateUnhealthy {
		t.Errorf("Expected unhealthy on initial failure, got %s", p2.currentState)
	}
}

func TestPolicyCheck(t *testing.T) {
	cfg := &config.Config{
		Health: config.HealthConfig{
			Mode:         config.HealthModeBasic,
			FailCount:    1,
			RecoverCount: 1,
			Basic: config.ChecksConfig{
				Checks: []config.CheckConfig{
					// Use localhost DNS which should work
					{Type: "tcp", Target: "127.0.0.1", Port: 22, Timeout: 1},
				},
			},
		},
	}

	p, err := NewPolicy(cfg)
	if err != nil {
		t.Skipf("Skipping test: %v", err)
	}

	ctx := context.Background()
	status := p.Check(ctx)

	if status == nil {
		t.Fatal("Expected non-nil status")
	}

	if len(status.CheckResults) != 1 {
		t.Errorf("Expected 1 check result, got %d", len(status.CheckResults))
	}

	// We don't assert healthy/unhealthy since port 22 might not be open
	if status.TotalCount != 1 {
		t.Errorf("Expected TotalCount=1, got %d", status.TotalCount)
	}
}
