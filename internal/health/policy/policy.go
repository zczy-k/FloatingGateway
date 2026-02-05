// Package policy provides health check aggregation and debounce logic.
package policy

import (
	"context"
	"sync"
	"time"

	"github.com/zczy-k/FloatingGateway/internal/config"
	"github.com/zczy-k/FloatingGateway/internal/health/checks"
)

// State represents the current health state.
type State string

const (
	StateHealthy   State = "healthy"
	StateUnhealthy State = "unhealthy"
	StateUnknown   State = "unknown"
)

// Status represents the aggregated health status.
type Status struct {
	Healthy       bool             `json:"healthy"`
	State         State            `json:"state"`
	Reason        string           `json:"reason"`
	Mode          string           `json:"mode"`
	CheckResults  []*checks.Result `json:"check_results"`
	PassedCount   int              `json:"passed_count"`
	TotalCount    int              `json:"total_count"`
	RequiredCount int              `json:"required_count"` // k in k-of-n
	LastCheck     time.Time        `json:"last_check"`
	StateChangedAt time.Time       `json:"state_changed_at"`
}

// Policy handles health check aggregation and debouncing.
type Policy struct {
	mu sync.RWMutex

	cfg      *config.Config
	checkers []checks.Checker
	mode     config.HealthMode

	// k-of-n parameters (0 means all-of-n)
	k int
	n int

	// Debounce state
	failCount    int
	recoverCount int
	currentState State
	lastStatus   *Status
	stateChangedAt time.Time

	// Hold-down timer
	holdDownUntil time.Time
}

// NewPolicy creates a new health policy.
func NewPolicy(cfg *config.Config) (*Policy, error) {
	p := &Policy{
		cfg:          cfg,
		mode:         cfg.Health.Mode,
		currentState: StateUnknown,
	}

	// Parse k-of-n
	k, n, err := config.ParseKOfN(cfg.Health.KOfN)
	if err != nil {
		return nil, err
	}
	p.k = k
	p.n = n

	// Create checkers
	checkConfigs := cfg.GetChecks()
	checkers, err := checks.CreateCheckers(checkConfigs)
	if err != nil {
		return nil, err
	}
	p.checkers = checkers

	return p, nil
}

// Check performs a health check and returns the current status.
func (p *Policy) Check(ctx context.Context) *Status {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Run all checks
	results := checks.RunAll(ctx, p.checkers)

	// Count passed checks
	passed := 0
	for _, r := range results {
		if r.OK {
			passed++
		}
	}

	total := len(results)
	required := p.k
	if required == 0 {
		// all-of-n mode
		required = total
	}

	// Determine if this check round passed
	roundPassed := passed >= required

	// Apply debounce logic
	newState := p.applyDebounce(roundPassed)

	status := &Status{
		Healthy:       newState == StateHealthy,
		State:         newState,
		Mode:          string(p.mode),
		CheckResults:  results,
		PassedCount:   passed,
		TotalCount:    total,
		RequiredCount: required,
		LastCheck:     time.Now(),
		StateChangedAt: p.stateChangedAt,
	}

	// Set reason
	if newState == StateHealthy {
		status.Reason = "all checks passing"
		if p.k > 0 {
			status.Reason = "enough checks passing (k-of-n)"
		}
	} else if newState == StateUnhealthy {
		status.Reason = "checks failing"
		if p.k > 0 {
			status.Reason = "not enough checks passing (k-of-n)"
		}
	} else {
		status.Reason = "initializing"
	}

	p.lastStatus = status
	return status
}

func (p *Policy) applyDebounce(roundPassed bool) State {
	now := time.Now()

	// Check hold-down period
	if now.Before(p.holdDownUntil) {
		return p.currentState
	}

	if roundPassed {
		// Reset fail counter on success
		p.failCount = 0

		if p.currentState == StateHealthy {
			// Already healthy, stay healthy
			return StateHealthy
		}

		// Increment recover counter
		p.recoverCount++

		if p.recoverCount >= p.cfg.Health.RecoverCount {
			// Transition to healthy
			p.currentState = StateHealthy
			p.stateChangedAt = now
			p.recoverCount = 0

			// Apply hold-down if configured
			if p.cfg.Health.HoldDownSec > 0 {
				p.holdDownUntil = now.Add(time.Duration(p.cfg.Health.HoldDownSec) * time.Second)
			}
		}
	} else {
		// Reset recover counter on failure
		p.recoverCount = 0

		if p.currentState == StateUnhealthy {
			// Already unhealthy, stay unhealthy
			return StateUnhealthy
		}

		// Increment fail counter
		p.failCount++

		if p.failCount >= p.cfg.Health.FailCount {
			// Transition to unhealthy
			p.currentState = StateUnhealthy
			p.stateChangedAt = now
			p.failCount = 0
		}
	}

	// Handle initial state
	if p.currentState == StateUnknown {
		if roundPassed {
			p.currentState = StateHealthy
			p.stateChangedAt = now
		} else {
			p.currentState = StateUnhealthy
			p.stateChangedAt = now
		}
	}

	return p.currentState
}

// GetStatus returns the last known status.
func (p *Policy) GetStatus() *Status {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastStatus
}

// GetState returns the current state.
func (p *Policy) GetState() State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentState
}

// IsHealthy returns true if currently healthy.
func (p *Policy) IsHealthy() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.currentState == StateHealthy
}

// Reset resets the policy state.
func (p *Policy) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failCount = 0
	p.recoverCount = 0
	p.currentState = StateUnknown
	p.lastStatus = nil
	p.holdDownUntil = time.Time{}
}
