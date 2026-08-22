package monitor

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWebhookNotifier_PostsEventAsJSON(t *testing.T) {
	received := make(chan AlertEvent, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected application/json content type, got %s", r.Header.Get("Content-Type"))
		}
		var ev AlertEvent
		if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
			t.Errorf("failed to decode body: %v", err)
		}
		received <- ev
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	n := NewWebhookNotifier(srv.URL)
	want := AlertEvent{Target: "svc", OldState: StateUp, NewState: StateDown, At: time.Now()}
	n.Notify(want)

	select {
	case got := <-received:
		if got.Target != want.Target || got.OldState != want.OldState || got.NewState != want.NewState {
			t.Fatalf("received event %+v does not match sent event %+v", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("webhook was never delivered")
	}
}

func TestWebhookNotifier_DoesNotPanicOnFailure(t *testing.T) {
	// Point at a port nothing is listening on -- Notify must log, not panic.
	n := NewWebhookNotifier("http://127.0.0.1:1")
	n.Notify(AlertEvent{Target: "svc", OldState: StateUp, NewState: StateDown, At: time.Now()})
}
