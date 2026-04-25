package webhook

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

// WebhookError is an error associated with HTTP errors from webhook requests.
type WebhookError struct {
	Status     string
	StatusCode int
}

func (err WebhookError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", err.StatusCode, err.Status)
}

// IsRetryable returns true for HTTP errors 429 (too many requests) and 5xx (server errors).
// It also returns true if the request times out. All other errors return false.
func IsRetryable(err error) bool {
	webhookErr, ok := errors.AsType[WebhookError](err)
	if ok {
		return webhookErr.StatusCode == http.StatusTooManyRequests || webhookErr.StatusCode >= http.StatusInternalServerError
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return false
}
