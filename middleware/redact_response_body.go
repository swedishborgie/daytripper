package middleware

import (
	"github.com/swedishborgie/daytripper/har"
	"github.com/swedishborgie/daytripper/receiver"
)

// RedactResponseBodyIf replaces the response body with value when match returns true for the request URL.
// The entry itself (status, headers, timing) is preserved. Use this when you need a custom matcher across
// multiple URLs (prefix, host set, regexp, etc.).
func RedactResponseBodyIf(match func(url string) bool, value string) receiver.EntryMiddleware {
	return func(recv receiver.EntryReceiver) receiver.EntryReceiver {
		return func(entry *har.Entry) error {
			if entry.Request != nil && match(entry.Request.URL) {
				if entry.Response != nil && entry.Response.Content != nil {
					entry.Response.Content.Text = value
					entry.Response.Content.Encoding = ""
				}
			}

			return recv(entry)
		}
	}
}

// RedactResponseBody checks if the request URL equals url and if so, replaces the response body with value.
// It is equivalent to RedactResponseBodyIf with a string equality matcher.
func RedactResponseBody(url, value string) receiver.EntryMiddleware {
	return RedactResponseBodyIf(func(u string) bool { return u == url }, value)
}
