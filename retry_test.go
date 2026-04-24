package webhook

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestRetry(t *testing.T) {
	cfg := retryConfig{
		maxTime:    10 * time.Second,
		maxRetries: 5,
		backoff:    func(n int) time.Duration { return 0 * time.Second },
		retryable:  func(error) bool { return true },
	}

	attempts := 3
	attempt := 0
	start := time.Now()

	err := retry(context.Background(), cfg, func() error {
		attempt += 1
		t.Logf("attempt %d after %s", attempt, time.Since(start))
		if attempt == attempts {
			return nil
		}
		return fmt.Errorf("failed")
	})
	if attempt != attempts {
		t.Errorf("expected %d attempts, only saw %d", attempts, attempt)
	}
	if err != nil {
		t.Error(err)
	}
}

func TestExponentialBackoff(t *testing.T) {
	backoffFunc := ExponentialBackoff(500*time.Millisecond, 1*time.Minute)
	testcases := []struct {
		name     string
		n        int
		expected time.Duration
	}{
		{"0 tries", 0, 0},
		{"1 tries", 1, 500 * time.Millisecond},
		{"2 tries", 2, 1 * time.Second},
		{"100 tries", 100, 1 * time.Minute},
	}

	for _, c := range testcases {
		if backoff := backoffFunc(c.n); backoff != c.expected {
			t.Errorf("expected %s backoff for attempt %d, got %s", c.expected, c.n, backoff)
		}
	}
}

func TestLinearBackoff(t *testing.T) {
	backoffFunc := LinearBackoff(500*time.Millisecond, 1*time.Minute)
	testcases := []struct {
		name     string
		n        int
		expected time.Duration
	}{
		{"0 tries", 0, 0},
		{"1 tries", 1, 500 * time.Millisecond},
		{"2 tries", 2, 1000 * time.Millisecond},
		{"3 tries", 3, 1500 * time.Millisecond},
	}

	for _, c := range testcases {
		if backoff := backoffFunc(c.n); backoff != c.expected {
			t.Errorf("expected %s backoff for attempt %d, got %s", c.expected, c.n, backoff)
		}
	}
}
