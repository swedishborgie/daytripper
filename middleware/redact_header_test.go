package middleware_test

import (
	"github.com/swedishborgie/daytripper/har"
	"github.com/swedishborgie/daytripper/middleware"
	"net/http"
	"testing"
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			redactMW := middleware.RedactHeader(tc.key, tc.value)

			err := redactMW(func(entry *har.Entry) error {
				for _, h := range entry.Request.Headers {
					if http.CanonicalHeaderKey(h.Name) == http.CanonicalHeaderKey(tc.key) &&
						h.Value != tc.value {
						t.Fatalf("redacted header value is %s, want %s", h.Value, tc.value)
					}
				}

				for _, h := range entry.Response.Headers {
					if http.CanonicalHeaderKey(h.Name) == http.CanonicalHeaderKey(tc.key) &&
						h.Value != tc.value {
						t.Fatalf("redacted header value is %s, want %s", h.Value, tc.value)
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
