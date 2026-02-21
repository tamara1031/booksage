package util

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"strings"
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

	// Request logging
	var reqBody []byte
	if req.Body != nil {
		reqBody, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(reqBody))
	}

	log.Printf("DEBUG OUTBOUND REQUEST: [%s] %s", req.Method, req.URL.String())
	if len(reqBody) > 0 {
		// Avoid logging large binary files (e.g., ebook uploads)
		if strings.Contains(req.Header.Get("Content-Type"), "multipart/form-data") {
			log.Printf("DEBUG OUTBOUND REQUEST BODY: <multipart/form-data, length=%d>", len(reqBody))
		} else {
			log.Printf("DEBUG OUTBOUND REQUEST BODY: %s", string(reqBody))
		}
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

	respBody, _ := io.ReadAll(resp.Body)
	resp.Body = io.NopCloser(bytes.NewBuffer(respBody))

	if len(respBody) > 0 {
		log.Printf("DEBUG OUTBOUND RESPONSE BODY: %s", string(respBody))
	}

	return resp, nil
}
