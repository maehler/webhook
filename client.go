package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// Result is a result from Send on a webhook client.
type Result struct {
	Response *http.Response
	Attempts int
}

// Client is a webhook client.
type Client struct {
	client *http.Client
	URL    string
	Method string
	// Timeout is the timeout for each individual request that is sent.
	Timeout     time.Duration
	Headers     http.Header
	retryConfig retryConfig
}

// NewClient creates a new webhook client. The default config uses a timeout for each
// individual request of 10 seconds, and a maximum of 15 retries with exponential backoff
// starting at 500ms, and a total timeout for all retries of 5 minutes.
func NewClient(url string, opts ...ClientOptionFunc) *Client {
	client := Client{
		URL:     url,
		client:  &http.Client{},
		Method:  http.MethodPost,
		Timeout: 10 * time.Second,
		retryConfig: retryConfig{
			maxRetries: 15,
			maxTime:    5 * time.Minute,
			backoff:    ExponentialBackoff(500*time.Millisecond, 1*time.Minute),
			retryable:  IsRetryable,
		},
	}
	for _, f := range opts {
		f(&client)
	}
	client.client.Timeout = client.Timeout
	return &client
}

// Send a webhook payload to the specified url. `Result.Response` can be nil
// if a non-nil error is returned, but will always be defined if a nil error
// is returned.
func (c *Client) Send(payload any) (Result, error) {
	return c.SendContext(context.Background(), payload)
}

// SendContext sends a webhook payload to the specified url with a given context.
// ctx should not have a timeout set since Send manages this internally both
// for individual requests and the total time for all retries. Cancellation
// via ctx still happens. `Result.Response` can be nil if a non-nil error is
// returned, but will always be defined if a nil error is returned.
func (c *Client) SendContext(ctx context.Context, payload any) (Result, error) {
	var result Result
	attempts, retryErr := retry(ctx, c.retryConfig, func() error {
		resp, err := c.send(ctx, c.URL, payload)
		if resp != nil {
			result.Response = resp
		}
		return err
	})
	result.Attempts = attempts
	return result, retryErr
}

func (c *Client) send(ctx context.Context, url string, payload any) (*http.Response, error) {
	var resp *http.Response
	b, err := json.Marshal(payload)
	if err != nil {
		return resp, err
	}
	br := bytes.NewReader(b)
	r, err := http.NewRequestWithContext(ctx, c.Method, url, br)
	if err != nil {
		return resp, err
	}
	for key, value := range c.Headers {
		r.Header.Set(key, strings.Join(value, " "))
	}
	r.Header.Set("Content-Type", "application/json")

	resp, err = c.client.Do(r)
	if err != nil {
		return resp, err
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return resp, nil
	}

	return resp, WebhookError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
	}
}
