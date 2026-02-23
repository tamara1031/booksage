package httpclient

import (
	"bytes"
	"errors"
	"net/http"
	"testing"
)

type mockRoundTripper struct {
	attempts int
	err      error
	resp     *http.Response
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.attempts++
	return m.resp, m.err
}

func TestRetryTransport_RoundTrip_Retry(t *testing.T) {
	mock := &mockRoundTripper{
		err: errors.New("transient error"),
	}

	rt := &RetryTransport{
		Base:       mock,
		MaxRetries: 1, // Minimize sleep
	}

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	_, err := rt.RoundTrip(req)

	if err == nil {
		t.Error("Expected error")
	}
	// 1 initial + 1 retry = 2 attempts
	if mock.attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", mock.attempts)
	}
}

func TestRetryTransport_RoundTrip_NoRetryBodyWithoutGetBody(t *testing.T) {
	mock := &mockRoundTripper{
		err: errors.New("transient error"),
	}

	rt := &RetryTransport{
		Base:       mock,
		MaxRetries: 1,
	}

	req, _ := http.NewRequest("POST", "http://example.com", bytes.NewReader([]byte("body")))
	// Simulate non-rewindable body (nil GetBody)
	req.GetBody = nil
	// req.Body is not nil (bytes.NewReader)

	_, err := rt.RoundTrip(req)

	if err == nil {
		t.Error("Expected error")
	}
	if mock.attempts != 1 { // Should not retry
		t.Errorf("Expected 1 attempt, got %d", mock.attempts)
	}
}

func TestRetryTransport_RoundTrip_RetryBodyWithGetBody(t *testing.T) {
	mock := &mockRoundTripper{
		err: errors.New("transient error"),
	}

	rt := &RetryTransport{
		Base:       mock,
		MaxRetries: 1,
	}

	req, _ := http.NewRequest("POST", "http://example.com", bytes.NewBufferString("body"))
	// NewRequest sets GetBody automatically for bytes.Buffer

	_, err := rt.RoundTrip(req)

	if err == nil {
		t.Error("Expected error")
	}
	if mock.attempts != 2 { // 1 initial + 1 retry
		t.Errorf("Expected 2 attempts, got %d", mock.attempts)
	}
}
