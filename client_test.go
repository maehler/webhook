package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type capturedRequest struct {
	body   []byte
	header http.Header
}

func NewTestHandler(t *testing.T, d time.Duration, status int, handler func(*capturedRequest)) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		handler(&capturedRequest{body: body, header: r.Header})
		w.WriteHeader(status)
		if d != 0 {
			time.Sleep(d)
		}
	})
}

func NewTestServer(t *testing.T, status int, handler func(*capturedRequest)) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(NewTestHandler(t, 0, status, handler))
	t.Logf("test server at %s", s.URL)
	return s
}

func NewSlowServer(t *testing.T, d time.Duration, status int, handler func(*capturedRequest)) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(NewTestHandler(t, d, status, handler))
	t.Logf("slow test server at %s", s.URL)
	return s
}

func NewBadServer(t *testing.T, failures int, handler func(*capturedRequest)) *httptest.Server {
	t.Helper()
	failureCount := 0
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if failureCount >= failures {
			w.WriteHeader(http.StatusOK)
			return
		}
		failureCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Logf("bad test server at %s", s.URL)
	return s
}

func TestClient(t *testing.T) {
	payload := "this is a test"
	expectedPayload, _ := json.Marshal(payload)

	server := NewTestServer(t, 200, func(r *capturedRequest) {
		if !bytes.Equal(r.body, expectedPayload) {
			t.Error("mismatching payload")
		}
	})
	defer server.Close()

	c := NewClient()
	if err := c.Send(server.URL, payload); err != nil {
		t.Error(err)
	}
}

func TestClientWithHeaders(t *testing.T) {
	payload := "this is a test"
	expectedPayload, _ := json.Marshal(payload)

	server := NewTestServer(t, http.StatusOK, func(r *capturedRequest) {
		if r.header.Get("x-version") != "1.2.3" {
			t.Error("wrong x-version in header")
		}
		if r.header.Get("user-agent") != "webhook-test/1.0" {
			t.Error("wrong user-agent in header")
		}
		if !bytes.Equal(r.body, expectedPayload) {
			t.Error("mismatching payload")
		}
	})
	defer server.Close()

	c := NewClient(
		ClientOpts.WithRequestTimeout(50*time.Millisecond),
		ClientOpts.WithBackoffFunc(NoBackoff()),
		ClientOpts.WithHeaders(http.Header{
			"x-version":  []string{"1.2.3"},
			"user-agent": []string{"webhook-test/1.0"},
		}),
	)
	if err := c.Send(server.URL, payload); err != nil {
		t.Error(err)
	}
}

func TestClientTimeout(t *testing.T) {
	count := 0
	server := NewSlowServer(t, 200*time.Millisecond, http.StatusOK, func(r *capturedRequest) {
		count += 1
	})
	defer server.Close()

	c := NewClient(
		ClientOpts.WithRetries(3),
		ClientOpts.WithRequestTimeout(50*time.Millisecond),
		ClientOpts.WithBackoffFunc(NoBackoff()),
	)
	err := c.Send(server.URL, "this is a test")
	if count != 3 {
		t.Errorf("expected %d attempts, got %d attempts", 3, count)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error(err)
	}
}

func TestRateLimit(t *testing.T) {
	server := NewTestServer(t, http.StatusOK, func(cr *capturedRequest) {
	})
	defer server.Close()
}

func TestClientRetryOn500(t *testing.T) {
	server := NewBadServer(t, 3, func(cr *capturedRequest) {})
	defer server.Close()

	c := NewClient(
		ClientOpts.WithBackoffFunc(NoBackoff()),
	)
	err := c.Send(server.URL, "this is a test")
	if err != nil {
		t.Error(err)
	}
}

func TestClientCancel(t *testing.T) {
	server := NewBadServer(t, 5, func(cr *capturedRequest) {})
	defer server.Close()

	c := NewClient(
		ClientOpts.WithBackoffFunc(ConstantBackoff(1 * time.Minute)),
	)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := c.SendCtx(ctx, server.URL, "this is a test")
	if !errors.Is(err, context.Canceled) {
		t.Error(err)
	}
}

func TestRetryTimelimit(t *testing.T) {
	count := 0
	server := NewSlowServer(t, 20*time.Millisecond, http.StatusOK, func(cr *capturedRequest) {
		count += 1
		t.Logf("request %d", count)
	})
	defer server.Close()

	c := NewClient(
		ClientOpts.WithRequestTimeout(10*time.Millisecond),
		ClientOpts.WithBackoffFunc(LinearBackoff(4*time.Millisecond, 100*time.Millisecond)),
	)

	err := c.SendCtx(context.Background(), server.URL, "this is a test")

	// All retries should have gone through
	if count != 15 {
		t.Errorf("expected %d requests, got %d requests", 15, count)
	}
	t.Log(err)
}
