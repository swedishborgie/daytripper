package middleware

import (
	"github.com/swedishborgie/daytripper/har"
	"github.com/swedishborgie/daytripper/receiver"
	"net/http"
)

// RedactHeader will scan request and response headers for a particular header key, if found it will be replaced by the
// passed in value.
func RedactHeader(header, value string) receiver.EntryMiddleware {
	return func(recv receiver.EntryReceiver) receiver.EntryReceiver {
		redactHeader := http.CanonicalHeaderKey(header)

		return func(entry *har.Entry) error {
			for _, h := range entry.Request.Headers {
				if redactHeader == http.CanonicalHeaderKey(h.Name) {
					h.Value = value
				}
			}

			for _, h := range entry.Response.Headers {
				if redactHeader == http.CanonicalHeaderKey(h.Name) {
					h.Value = value
				}
			}

			// Pass to the next middleware.
			return recv(entry)
		}
	}
}
