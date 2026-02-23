package httpclient

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// LoggingTransport is an http.RoundTripper that logs request and response bodies.
type LoggingTransport struct {
	Base     http.RoundTripper
	LogLevel string
}

func (t *LoggingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.ToLower(t.LogLevel) != "debug" {
		base := t.Base
		if base == nil {
			base = http.DefaultTransport
		}
		return base.RoundTrip(req)
	}

	log.Printf("DEBUG OUTBOUND REQUEST: [%s] %s", req.Method, req.URL.String())
	if strings.Contains(req.Header.Get("Content-Type"), "multipart/form-data") {
		log.Printf("DEBUG OUTBOUND REQUEST BODY: <multipart/form-data, logic skipped to prevent OOM>")
	} else if req.Body != nil {
		reqBody, _ := io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(reqBody))
		log.Printf("DEBUG OUTBOUND REQUEST BODY: %s", string(reqBody))
	}

	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}

	resp, err := base.RoundTrip(req)
	if err != nil {
		return resp, err
	}

	// Response logging
	log.Printf("DEBUG OUTBOUND RESPONSE: %d %s", resp.StatusCode, req.URL.String())

	// Skip body logging for binary or large responses to prevent OOM
	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "application/epub") ||
		strings.HasPrefix(contentType, "application/pdf") {
		log.Printf("DEBUG OUTBOUND RESPONSE BODY: <binary content skipped>")
		return resp, nil
	}

	respBody, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(respBody))

	if len(respBody) > 0 {
		log.Printf("DEBUG OUTBOUND RESPONSE BODY: %s", string(respBody))
	}

	return resp, nil
}

// RetryTransport is an http.RoundTripper that retries on transient errors.
type RetryTransport struct {
	Base       http.RoundTripper
	MaxRetries int
}

func (t *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}

	var lastErr error
	var resp *http.Response

	for i := 0; i <= t.MaxRetries; i++ {
		// Before retrying, rewind body if needed
		if i > 0 && req.Body != nil {
			if req.GetBody != nil {
				newBody, err := req.GetBody()
				if err != nil {
					// Cannot rewind body, cannot retry.
					return resp, lastErr
				}
				req.Body = newBody
			} else {
				// Cannot rewind body, cannot retry.
				// We already failed in previous step.
				return resp, lastErr
			}
		}

		resp, lastErr = base.RoundTrip(req)

		// If success and not 429/5xx, return immediately
		if lastErr == nil && resp.StatusCode < 500 && resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		// Prepare for retry
		if lastErr == nil {
			// Close response body before retrying to prevent leak
			resp.Body.Close()
		}

		// Don't sleep after the last attempt
		if i < t.MaxRetries {
			time.Sleep(t.backoff(i + 1))
		}
	}

	return resp, lastErr
}

func (t *RetryTransport) backoff(attempt int) time.Duration {
	if attempt == 0 {
		return 0
	}
	// Exponential backoff: 1s, 2s, 4s...
	return time.Duration(1<<(attempt-1)) * time.Second
}
