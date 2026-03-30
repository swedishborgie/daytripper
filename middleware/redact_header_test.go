package middleware_test

import (
	"net/http"
	"testing"

	"github.com/swedishborgie/daytripper/har"
	"github.com/swedishborgie/daytripper/middleware"
)

func TestRedactHeader(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name  string
		entry *har.Entry
		key   string
		value string
	}{
		{
			name:  "authorization",
			key:   "authorization",
			value: "******",
			entry: &har.Entry{
				Request: &har.Request{
					Headers: []*har.Header{
						{
							Name:  "Authorization",
							Value: "Basic QWxhZGRpbjpvcGVuCg==",
						},
					},
				},
				Response: &har.Response{
					Headers: []*har.Header{
						{
							Name:  "AuThOrIzAtIoN",
							Value: "Basic QWxhZGRpbjpvcGVuCg==",
						},
					},
				},
			},
		},
		{
			name:  "nil check",
			key:   "authorization",
			value: "******",
			entry: &har.Entry{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			redactMW := middleware.RedactHeader(tc.key, tc.value)

			err := redactMW(func(entry *har.Entry) error {
				if entry.Request != nil {
					for _, h := range entry.Request.Headers {
						if http.CanonicalHeaderKey(h.Name) == http.CanonicalHeaderKey(tc.key) &&
							h.Value != tc.value {
							t.Fatalf("redacted header value is %s, want %s", h.Value, tc.value)
						}
					}
				}

				if entry.Response != nil {
					for _, h := range entry.Response.Headers {
						if http.CanonicalHeaderKey(h.Name) == http.CanonicalHeaderKey(tc.key) &&
							h.Value != tc.value {
							t.Fatalf("redacted header value is %s, want %s", h.Value, tc.value)
						}
					}
				}

				return nil
			})(tc.entry)
			if err != nil {
				t.Errorf("redactMW returned unexpected error: %v", err)
			}
		})
	}
}

func TestRedactHeaderNilResponse(t *testing.T) {
	t.Parallel()

	entry := &har.Entry{
		Request: &har.Request{
			Headers: []*har.Header{
				{Name: "Authorization", Value: "secret"},
			},
		},
		Response: nil,
	}

	redactMW := middleware.RedactHeader("Authorization", "***")
	err := redactMW(func(e *har.Entry) error { return nil })(entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Request.Headers[0].Value != "***" {
		t.Errorf("request header value = %q, want %q", entry.Request.Headers[0].Value, "***")
	}
}

func TestRedactHeaderNotPresent(t *testing.T) {
	t.Parallel()

	const originalValue = "Bearer token"
	entry := &har.Entry{
		Request: &har.Request{
			Headers: []*har.Header{
				{Name: "X-Custom-Header", Value: originalValue},
			},
		},
		Response: &har.Response{
			Headers: []*har.Header{
				{Name: "X-Custom-Header", Value: originalValue},
			},
		},
	}

	redactMW := middleware.RedactHeader("Authorization", "***")
	err := redactMW(func(e *har.Entry) error { return nil })(entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if entry.Request.Headers[0].Value != originalValue {
		t.Errorf("request header changed to %q, want %q", entry.Request.Headers[0].Value, originalValue)
	}
	if entry.Response.Headers[0].Value != originalValue {
		t.Errorf("response header changed to %q, want %q", entry.Response.Headers[0].Value, originalValue)
	}
}

func TestRedactHeaderMultipleValues(t *testing.T) {
	t.Parallel()

	entry := &har.Entry{
		Request: &har.Request{
			Headers: []*har.Header{
				{Name: "Authorization", Value: "first-secret"},
				{Name: "Authorization", Value: "second-secret"},
				{Name: "X-Other", Value: "keep-me"},
			},
		},
	}

	redactMW := middleware.RedactHeader("Authorization", "***")
	err := redactMW(func(e *har.Entry) error { return nil })(entry)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, h := range entry.Request.Headers {
		if http.CanonicalHeaderKey(h.Name) == "Authorization" && h.Value != "***" {
			t.Errorf("Authorization header not redacted: %q", h.Value)
		}
	}
	if entry.Request.Headers[2].Value != "keep-me" {
		t.Errorf("X-Other header changed to %q, want %q", entry.Request.Headers[2].Value, "keep-me")
	}
}
