package monitor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// WebhookNotifier POSTs a JSON AlertEvent to a configured URL (Slack
// incoming webhooks, a custom endpoint, etc.). A failed delivery is logged,
// never fatal -- a broken alert channel shouldn't take the monitor down.
type WebhookNotifier struct {
	URL    string
	Client *http.Client
}

func NewWebhookNotifier(url string) *WebhookNotifier {
	return &WebhookNotifier{
		URL:    url,
		Client: &http.Client{Timeout: 5 * time.Second},
	}
}

func (w *WebhookNotifier) Notify(event AlertEvent) {
	body, err := json.Marshal(event)
	if err != nil {
		log.Printf("webhook: failed to marshal event: %v", err)
		return
	}

	resp, err := w.Client.Post(w.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("webhook: delivery failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		log.Printf("webhook: unexpected status %d", resp.StatusCode)
	}
}

// LogNotifier just prints alerts to stdout -- the default when no webhook
// is configured, and handy for local testing.
type LogNotifier struct{}

func (LogNotifier) Notify(event AlertEvent) {
	fmt.Printf("[ALERT] %s: %s -> %s at %s\n", event.Target, event.OldState, event.NewState, event.At.Format(time.RFC3339))
}
