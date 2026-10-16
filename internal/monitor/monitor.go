// Package monitor tracks the up/down state of a set of HTTP targets and
// fires alerts on state transitions (not on every check, so a target that
// stays down doesn't spam the alert channel).
package monitor

import (
	"context"
	"sync"
	"time"
)

type State string

const (
	StateUp   State = "up"
	StateDown State = "down"
)

// Target is one endpoint to watch.
type Target struct {
	Name            string
	URL             string
	IntervalSeconds int
	TimeoutSeconds  int
}

// CheckResult is the latest known state of a target.
type CheckResult struct {
	Target    string    `json:"target"`
	State     State     `json:"state"`
	CheckedAt time.Time `json:"checked_at"`
	Error     string    `json:"error,omitempty"`
}

// AlertEvent is emitted whenever a target's state changes.
type AlertEvent struct {
	Target   string    `json:"target"`
	OldState State     `json:"old_state"`
	NewState State     `json:"new_state"`
	At       time.Time `json:"at"`
}

// Notifier is notified whenever a target transitions between up/down.
type Notifier interface {
	Notify(event AlertEvent)
}

// Checker performs one liveness check against url, returning a non-nil
// error if the target is considered down.
type Checker func(ctx context.Context, url string, timeout time.Duration) error

// Monitor runs concurrent checks against a set of targets and keeps their
// latest state in memory.
type Monitor struct {
	mu       sync.RWMutex
	states   map[string]CheckResult
	notifier Notifier
	checker  Checker
}

func New(notifier Notifier, checker Checker) *Monitor {
	return &Monitor{
		states:   make(map[string]CheckResult),
		notifier: notifier,
		checker:  checker,
	}
}

// RunCheck performs a single check for t and updates state. If the state
// changed since the last check (and there was a previous check), the
// notifier is invoked exactly once. The very first check for a target only
// establishes the baseline state; it never fires an alert.
func (m *Monitor) RunCheck(ctx context.Context, t Target) CheckResult {
	timeout := time.Duration(t.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	err := m.checker(ctx, t.URL, timeout)
	newState := StateUp
	errMsg := ""
	if err != nil {
		newState = StateDown
		errMsg = err.Error()
	}

	result := CheckResult{
		Target:    t.Name,
		State:     newState,
		CheckedAt: time.Now(),
		Error:     errMsg,
	}

	m.mu.Lock()
	old, existed := m.states[t.Name]
	// Cancellation belongs to the monitor lifecycle, not target health.
	// Preserve the last observation and avoid a false outage on shutdown.
	if ctx.Err() != nil {
		m.mu.Unlock()
		if existed {
			return old
		}
		return result
	}
	m.states[t.Name] = result
	m.mu.Unlock()

	if existed && old.State != newState {
		m.notifier.Notify(AlertEvent{
			Target:   t.Name,
			OldState: old.State,
			NewState: newState,
			At:       result.CheckedAt,
		})
	}

	return result
}

// Start launches one goroutine per target that checks it on its own
// interval until ctx is cancelled.
func (m *Monitor) Start(ctx context.Context, targets []Target) {
	for _, t := range targets {
		go m.runLoop(ctx, t)
	}
}

func (m *Monitor) runLoop(ctx context.Context, t Target) {
	interval := time.Duration(t.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if ctx.Err() != nil {
		return
	}
	m.RunCheck(ctx, t)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.RunCheck(ctx, t)
		}
	}
}

// Snapshot returns a copy of the current state of every target checked so far.
func (m *Monitor) Snapshot() map[string]CheckResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]CheckResult, len(m.states))
	for k, v := range m.states {
		out[k] = v
	}
	return out
}
