package reload

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type HealthChecker struct {
	url     string
	timeout time.Duration
	client  *http.Client
}

func NewHealthChecker(url string) *HealthChecker {
	timeout := 500 * time.Millisecond
	return &HealthChecker{
		url:     url,
		timeout: timeout,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (h *HealthChecker) IsHealthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, h.url, nil)
	if err != nil {
		return false
	}

	resp, err := h.client.Do(req)
	if err != nil {
		return false
	}
	if err := resp.Body.Close(); err != nil {
		return false
	}

	return resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusBadRequest
}

func (h *HealthChecker) WaitForHealthy(ctx context.Context, pollInterval time.Duration) error {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if h.IsHealthy(ctx) {
				return nil
			}
		}
	}
}

func (h *HealthChecker) WaitForHealthyWithTimeout(timeout time.Duration, pollInterval time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return h.WaitForHealthy(ctx, pollInterval)
}

func BroadcastWhenHealthy(ctx context.Context, healthURL string, broadcaster *Broadcaster, log io.Writer, onHealthy func()) {
	if log == nil {
		log = io.Discard
	}
	checker := NewHealthChecker(healthURL)

	// Wait a brief moment for the server to actually stop
	time.Sleep(100 * time.Millisecond)

	// Wait for server to come back up (with timeout)
	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	err := checker.WaitForHealthy(waitCtx, 100*time.Millisecond)
	if err != nil {
		_, _ = fmt.Fprintf(log, "[run] Server health check timed out: %v\n", err) // Logging is best effort.
		return
	}

	// Small delay to ensure server is fully ready
	time.Sleep(50 * time.Millisecond)

	if onHealthy != nil {
		onHealthy()
	}

	broadcaster.Broadcast()
	_, _ = fmt.Fprintln(log, "[run] Server healthy, broadcasting reload") // Logging is best effort.
}
