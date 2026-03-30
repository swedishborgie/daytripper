package daytripper

import (
	"net/http"

	"github.com/swedishborgie/daytripper/receiver"
)

type Option func(*DayTripper)

// WithCreator sets the creator name that will be passed to the receiver.
func WithCreator(creator string) Option {
	return func(d *DayTripper) {
		d.version.Creator = creator
	}
}

// WithHARVersion sets the HAR version that will be passed to the receiver.
func WithHARVersion(harVersion string) Option {
	return func(d *DayTripper) {
		d.version.HARVersion = harVersion
	}
}

// WithVersion sets the version of the creator that will be passed to the receiver.
func WithVersion(version string) Option {
	return func(d *DayTripper) {
		d.version.Version = version
	}
}

// WithReceiver sets the receiver to use.
func WithReceiver(receiver receiver.Receiver) Option {
	return func(d *DayTripper) {
		d.receiver = receiver
	}
}

// WithPageMiddleware adds receiver.PageMiddleware to the set of middlewares. These can be used to filter or change
// pages before they're passed to the receiver.
func WithPageMiddleware(pageMWs ...receiver.PageMiddleware) Option {
	return func(d *DayTripper) {
		d.pageMWs = append(d.pageMWs, pageMWs...)
	}
}

// WithEntryMiddleware adds receiver.EntryMiddleware to the set of middleware. These can be used to filter or change
// entries before they're passed to the receiver.
func WithEntryMiddleware(entryMW ...receiver.EntryMiddleware) Option {
	return func(d *DayTripper) {
		d.entryMWs = append(d.entryMWs, entryMW...)
	}
}

// WithIncludeAll allows you to toggle whether all entries are included or not. By default, everything is recorded, if
// you toggle this off, you can use IncludeContext in a request context to indicate the request should be recorded.
func WithIncludeAll(includeAll bool) Option {
	return func(d *DayTripper) {
		d.includeAll = includeAll
	}
}

// WithTripper allows you to set the transport to forward requests to when executing requests. By default, it uses
// http.DefaultTransport.
func WithTripper(transport http.RoundTripper) Option {
	return func(d *DayTripper) {
		d.wrapped = transport
	}
}

// WithMaxBodySize sets the maximum number of bytes to buffer from request and response bodies.
// Bodies exceeding this limit are truncated in the HAR output, and a comment is added to the
// relevant HAR field indicating truncation. The limit also applies to decompressed content,
// preventing unlimited expansion of compressed bodies. A value of 0 (the default) means
// unlimited.
func WithMaxBodySize(maxBodySize int64) Option {
	return func(d *DayTripper) {
		d.maxBodySize = maxBodySize
	}
}

// WithBodyDecoder sets a custom BodyDecoder function used to decode response bodies based on their
// Content-Encoding header. Use this to add support for encodings not handled by the default decoder
// (e.g. brotli, zstd). The provided function receives the Content-Encoding value, the raw body
// bytes, and the maximum decoded size (0 means unlimited), and should return the decoded bytes.
func WithBodyDecoder(decoder BodyDecoder) Option {
	return func(d *DayTripper) {
		d.bodyDecoder = decoder
	}
}

// WithClient will take the passed in client and will wrap the configured transport and assign the round tripper.
// This won't work in all circumstances, you should look at WithTripper if you need more control over client
// composition.
func WithClient(client *http.Client) Option {
	return func(d *DayTripper) {
		tripper := http.DefaultTransport
		if client.Transport != nil {
			tripper = client.Transport
		}

		d.wrapped = tripper
		client.Transport = d
	}
}
