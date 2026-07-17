// Package httpx provides an HTTP transport that retries transient failures
// (429 and 5xx responses, and network errors) with exponential backoff.
// One mechanism covers every client built on net/http — Linear and OpenAI alike.
package httpx

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RetryTransport wraps a base RoundTripper with bounded, context-aware retries.
type RetryTransport struct {
	Base        http.RoundTripper
	MaxAttempts int           // total attempts including the first (default 4)
	BaseDelay   time.Duration // first backoff; doubles each retry (default 500ms)
}

func (t *RetryTransport) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}

func (t *RetryTransport) maxAttempts() int {
	if t.MaxAttempts > 0 {
		return t.MaxAttempts
	}
	return 4
}

func (t *RetryTransport) baseDelay() time.Duration {
	if t.BaseDelay > 0 {
		return t.BaseDelay
	}
	return 500 * time.Millisecond
}

// RoundTrip buffers the request body so it can be replayed across attempts,
// then retries on transient failures until success or attempts are exhausted.
func (t *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("httpx: buffer request body: %w", err)
		}
		body = b
	}

	ctx := req.Context()
	max := t.maxAttempts()
	var lastResp *http.Response
	var lastErr error

	for attempt := 1; attempt <= max; attempt++ {
		if body != nil {
			req.Body = io.NopCloser(bytes.NewReader(body))
		}
		resp, err := t.base().RoundTrip(req)
		lastResp, lastErr = resp, err

		transient := err != nil || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		if !transient {
			return resp, nil
		}
		if attempt == max {
			return resp, err // hand the caller the final result/status
		}
		if resp != nil {
			resp.Body.Close()
		}

		delay := t.baseDelay() * (1 << (attempt - 1))
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		}
	}
	return lastResp, lastErr
}

// Client returns an *http.Client whose transport retries transient failures.
// If base is nil, a fresh client is used.
func Client(base *http.Client) *http.Client {
	c := &http.Client{}
	if base != nil {
		*c = *base
	}
	c.Transport = &RetryTransport{Base: c.Transport}
	return c
}
