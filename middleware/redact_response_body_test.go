package middleware_test

import (
	"strings"
	"testing"

	"github.com/swedishborgie/daytripper/har"
	"github.com/swedishborgie/daytripper/middleware"
)

func TestRedactResponseBody(t *testing.T) {
	t.Parallel()

	const (
		matchURL    = "https://api.example.com/v1/token"
		otherURL    = "https://api.example.com/v1/other"
		redactValue = "<redacted>"
	)

	apply := func(entry *har.Entry) error {
		mw := middleware.RedactResponseBody(matchURL, redactValue)
		return mw(func(e *har.Entry) error { return nil })(entry)
	}

	t.Run("body and encoding", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name       string
			entry      *har.Entry
			wantText   string
			wantEnc    string
		}{
			{
				name: "redacts when request URL matches",
				entry: &har.Entry{
					Request: &har.Request{URL: matchURL},
					Response: &har.Response{
						Content: &har.Content{
							Text:     `{"api_key":"super-secret"}`,
							Encoding: "base64",
						},
					},
				},
				wantText: redactValue,
				wantEnc:  "",
			},
			{
				name: "leaves body when URL does not match",
				entry: &har.Entry{
					Request: &har.Request{URL: otherURL},
					Response: &har.Response{
						Content: &har.Content{Text: `{"result":"ok"}`},
					},
				},
				wantText: `{"result":"ok"}`,
				wantEnc:  "",
			},
			{
				name: "leaves body when request is nil",
				entry: &har.Entry{
					Request: nil,
					Response: &har.Response{
						Content: &har.Content{Text: "secret"},
					},
				},
				wantText: "secret",
				wantEnc:  "",
			},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				if err := apply(tt.entry); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if tt.entry.Response.Content.Text != tt.wantText {
					t.Errorf("Content.Text = %q, want %q", tt.entry.Response.Content.Text, tt.wantText)
				}
				if tt.entry.Response.Content.Encoding != tt.wantEnc {
					t.Errorf("Content.Encoding = %q, want %q", tt.entry.Response.Content.Encoding, tt.wantEnc)
				}
			})
		}
	})

	t.Run("no response or content to redact", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name  string
			entry *har.Entry
		}{
			{
				name: "nil response",
				entry: &har.Entry{
					Request:  &har.Request{URL: matchURL},
					Response: nil,
				},
			},
			{
				name: "nil content",
				entry: &har.Entry{
					Request:  &har.Request{URL: matchURL},
					Response: &har.Response{Content: nil},
				},
			},
		}

		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				if err := apply(tt.entry); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
		}
	})
}

func TestRedactResponseBodyIf(t *testing.T) {
	t.Parallel()

	const redactValue = "<redacted>"

	matchSecrets := func(u string) bool {
		return strings.HasPrefix(u, "https://api.example.com/v1/secrets/")
	}

	apply := func(entry *har.Entry) error {
		mw := middleware.RedactResponseBodyIf(matchSecrets, redactValue)
		return mw(func(e *har.Entry) error { return nil })(entry)
	}

	t.Run("redacts when matcher returns true", func(t *testing.T) {
		t.Parallel()
		entry := &har.Entry{
			Request: &har.Request{URL: "https://api.example.com/v1/secrets/token"},
			Response: &har.Response{
				Content: &har.Content{Text: `{"token":"x"}`, Encoding: "base64"},
			},
		}
		if err := apply(entry); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.Response.Content.Text != redactValue {
			t.Errorf("Content.Text = %q, want %q", entry.Response.Content.Text, redactValue)
		}
		if entry.Response.Content.Encoding != "" {
			t.Errorf("Content.Encoding = %q, want empty", entry.Response.Content.Encoding)
		}
	})

	t.Run("skips when matcher returns false", func(t *testing.T) {
		t.Parallel()
		entry := &har.Entry{
			Request:  &har.Request{URL: "https://api.example.com/v1/other"},
			Response: &har.Response{Content: &har.Content{Text: `{"ok":true}`}},
		}
		if err := apply(entry); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.Response.Content.Text != `{"ok":true}` {
			t.Errorf("Content.Text = %q, want unchanged", entry.Response.Content.Text)
		}
	})
}
