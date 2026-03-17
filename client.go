package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// Client is a webhook client.
type Client struct {
	client *http.Client
	Method string
	// Timeout is the timeout for each individual request that is sent.
	Timeout     time.Duration
	Headers     http.Header
	retryConfig retryConfig
}

// NewClient creates a new webhook client. The default config uses a timeout for each individual request of 1 minute,
// and a maximum of 15 retries with exponential backoff starting at 500ms.
func NewClient(opts ...ClientOptionFunc) *Client {
	client := Client{
		client:  &http.Client{},
		Timeout: 1 * time.Second,
		retryConfig: retryConfig{
			maxRetries: 15,
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

// Send a webhook payload to the specified url.
func (c *Client) Send(url string, payload any) error {
	return c.SendCtx(context.Background(), url, payload)
}

// Send a webhook payload to the specified url with a given context.
func (c *Client) SendCtx(ctx context.Context, url string, payload any) error {
	return retry(ctx, c.retryConfig, func() error {
		return c.send(ctx, url, payload)
	})
}

func (c *Client) send(ctx context.Context, url string, payload any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	br := bytes.NewReader(b)
	r, err := http.NewRequest(c.Method, url, br)
	if err != nil {
		return err
	}
	for key, value := range c.Headers {
		r.Header.Set(key, strings.Join(value, " "))
	}
	r.Header.Set("Content-Type", "application/json")
	r = r.WithContext(ctx)

	res, err := c.client.Do(r)
	if err != nil {
		return err
	}

	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}

	return WebhookError{
		StatusCode: res.StatusCode,
		Status:     res.Status,
	}
}
