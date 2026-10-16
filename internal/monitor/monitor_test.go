package monitor

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockNotifier struct {
	events []AlertEvent
}

func (m *mockNotifier) Notify(e AlertEvent) {
	m.events = append(m.events, e)
}

func sequenceChecker(results ...error) Checker {
	i := 0
	return func(ctx context.Context, url string, timeout time.Duration) error {
		if i >= len(results) {
			return results[len(results)-1]
		}
		r := results[i]
		i++
		return r
	}
}

func TestRunCheck_FirstCheckNeverAlerts(t *testing.T) {
	n := &mockNotifier{}
	m := New(n, sequenceChecker(nil))
	target := Target{Name: "svc", URL: "http://x", IntervalSeconds: 1, TimeoutSeconds: 1}

	m.RunCheck(context.Background(), target)

	if len(n.events) != 0 {
		t.Fatalf("expected no alerts on first check, got %d", len(n.events))
	}
}

func TestRunCheck_AlertsOnDownTransition(t *testing.T) {
	n := &mockNotifier{}
	m := New(n, sequenceChecker(nil, errors.New("boom")))
	target := Target{Name: "svc", URL: "http://x", IntervalSeconds: 1, TimeoutSeconds: 1}

	m.RunCheck(context.Background(), target) // establishes baseline: up
	m.RunCheck(context.Background(), target) // transitions to down

	if len(n.events) != 1 {
		t.Fatalf("expected exactly 1 alert, got %d", len(n.events))
	}
	ev := n.events[0]
	if ev.OldState != StateUp || ev.NewState != StateDown {
		t.Fatalf("expected up->down, got %s->%s", ev.OldState, ev.NewState)
	}
}

func TestRunCheck_AlertsOnRecovery(t *testing.T) {
	n := &mockNotifier{}
	m := New(n, sequenceChecker(errors.New("boom"), nil))
	target := Target{Name: "svc", URL: "http://x", IntervalSeconds: 1, TimeoutSeconds: 1}

	m.RunCheck(context.Background(), target) // baseline: down
	m.RunCheck(context.Background(), target) // recovers to up

	if len(n.events) != 1 {
		t.Fatalf("expected exactly 1 alert, got %d", len(n.events))
	}
	ev := n.events[0]
	if ev.OldState != StateDown || ev.NewState != StateUp {
		t.Fatalf("expected down->up, got %s->%s", ev.OldState, ev.NewState)
	}
}

func TestRunCheck_NoAlertWhenStateUnchanged(t *testing.T) {
	n := &mockNotifier{}
	m := New(n, sequenceChecker(nil))
	target := Target{Name: "svc", URL: "http://x", IntervalSeconds: 1, TimeoutSeconds: 1}

	for i := 0; i < 5; i++ {
		m.RunCheck(context.Background(), target)
	}

	if len(n.events) != 0 {
		t.Fatalf("expected no alerts for a target that stays up, got %d", len(n.events))
	}
}

func TestSnapshot_ReturnsIndependentCopy(t *testing.T) {
	n := &mockNotifier{}
	m := New(n, sequenceChecker(nil))
	target := Target{Name: "svc", URL: "http://x", IntervalSeconds: 1, TimeoutSeconds: 1}
	m.RunCheck(context.Background(), target)

	snap := m.Snapshot()
	if len(snap) != 1 {
		t.Fatalf("expected 1 entry in snapshot, got %d", len(snap))
	}
	delete(snap, "svc") // mutating the returned map must not affect internal state

	snap2 := m.Snapshot()
	if len(snap2) != 1 {
		t.Fatalf("mutating a snapshot copy corrupted internal state")
	}
}

func TestRunCheck_CancellationPreservesHealthAndDoesNotAlert(t *testing.T) {
	n := &mockNotifier{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	checker := func(ctx context.Context, _ string, _ time.Duration) error {
		calls++
		if calls == 1 {
			return nil
		}
		cancel()
		return ctx.Err()
	}
	m := New(n, checker)
	target := Target{Name: "svc", URL: "http://x"}
	baseline := m.RunCheck(ctx, target)
	result := m.RunCheck(ctx, target)
	if result != baseline || m.Snapshot()["svc"] != baseline {
		t.Fatal("shutdown cancellation changed the last health observation")
	}
	if len(n.events) != 0 {
		t.Fatal("shutdown emitted a false outage alert")
	}
}

func TestRunCheck_CancelledFirstObservationIsNotPublished(t *testing.T) {
	n := &mockNotifier{}
	ctx, cancel := context.WithCancel(context.Background())
	m := New(n, func(ctx context.Context, _ string, _ time.Duration) error { cancel(); return ctx.Err() })
	m.RunCheck(ctx, Target{Name: "svc"})
	if len(m.Snapshot()) != 0 || len(n.events) != 0 {
		t.Fatal("cancelled check published health")
	}
}

func TestRunCheck_TargetTimeoutStillMarksDown(t *testing.T) {
	n := &mockNotifier{}
	m := New(n, sequenceChecker(nil, context.DeadlineExceeded))
	target := Target{Name: "svc"}
	m.RunCheck(context.Background(), target)
	m.RunCheck(context.Background(), target)
	if m.Snapshot()["svc"].State != StateDown || len(n.events) != 1 {
		t.Fatal("target timeout should still report an outage when monitor context is live")
	}
}
