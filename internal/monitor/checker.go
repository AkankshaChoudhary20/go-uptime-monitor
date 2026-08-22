package monitor

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// HTTPChecker is the default Checker: does a GET and considers anything
// below 500 "up" (a 404 means the server answered; a 5xx or a network
// error means it's actually down).
func HTTPChecker(ctx context.Context, url string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("server error: status %d", resp.StatusCode)
	}
	return nil
}
