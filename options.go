package daytripper

import (
	"github.com/swedishborgie/daytripper/receiver"
	"net/http"
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
