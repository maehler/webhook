package webhook

import (
	"net/http"
	"time"
)

type (
	ClientOptions    struct{}
	ClientOptionFunc func(*Client)
)

var ClientOpts ClientOptions = struct{}{}

// WithTimout sets the timeout for individual webhook requests.
func (o ClientOptions) WithTimeout(timeout time.Duration) ClientOptionFunc {
	return func(c *Client) {
		c.Timeout = timeout
	}
}

// WithHeaders sets HTTP headers that should be used for all webhook requests.
func (o ClientOptions) WithHeaders(headers http.Header) ClientOptionFunc {
	return func(c *Client) {
		c.Headers = headers
	}
}

// WithRetries sets the maximum number of retries that should be made.
func (o ClientOptions) WithRetries(n int) ClientOptionFunc {
	return func(c *Client) {
		c.retryConfig.maxRetries = n
	}
}

// WithBackoffFunc sets the backoff function to use when retrying requests.
func (o ClientOptions) WithBackoffFunc(fn func(int) time.Duration) ClientOptionFunc {
	return func(c *Client) {
		c.retryConfig.backoff = fn
	}
}

// WithRetryableFunc sets the retryable function for determining which HTTP errors should trigger a retry if encountered.
func (o ClientOptions) WithRetryableFunc(fn func(error) bool) ClientOptionFunc {
	return func(c *Client) {
		c.retryConfig.retryable = fn
	}
}
