package webhook

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type (
	RetryableFunc func(error) bool
	BackoffFunc   func(int) time.Duration
)

type retryConfig struct {
	// maxRetries is the maximum number of times to retry execution
	maxRetries int
	// backoff determines the amount of time to wait until the next attempt as a function of the number of attempts.
	backoff BackoffFunc
	// retryable determines what types of errors should be retried as a function of the error.
	retryable RetryableFunc
}

// retry retries the execution of a function if the function returns an error.
// The behaviour of the retrying is controlled by retryConfig.
func retry(ctx context.Context, cfg retryConfig, fn func() error) error {
	var err error
	for attempt := range cfg.maxRetries {
		slog.Info("retrying function call", "attempt", attempt)
		err = fn()
		if err == nil {
			return nil
		}
		if !cfg.retryable(err) {
			return err
		}
		wait := cfg.backoff(attempt + 1)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	return fmt.Errorf("failed after %d attempts, last error: %w", cfg.maxRetries, err)
}

// NoBackoff returns a BackoffFunc that always returns 0.
func NoBackoff() func(int) time.Duration {
	return ConstantBackoff(0)
}

// ConstantBackoff returns a BackoffFunc that returns a constant time.
func ConstantBackoff(wait time.Duration) BackoffFunc {
	return func(attempt int) time.Duration {
		return wait
	}
}

// LinearBackoff returns a BackoffFunc that returns a time that scales
// linearly with the number of attempts. The time is capped at max.
func LinearBackoff(base time.Duration, max time.Duration) BackoffFunc {
	return func(attempt int) time.Duration {
		t := base * time.Duration(attempt)
		if t > max {
			return max
		}
		return t
	}
}

// ExponentialBackoff returns a BackoffFunc that returns a time that scales
// exponentially with the number of attempts. The time is capped at max.
func ExponentialBackoff(base time.Duration, max time.Duration) BackoffFunc {
	return func(attempt int) time.Duration {
		if attempt == 0 {
			return 0
		}
		t := base * (1 << (attempt - 1))
		if t > max || t == 0 {
			return max
		}
		return t
	}
}
