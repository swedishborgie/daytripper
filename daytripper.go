// Package daytripper provides a set of utilities to record and store HTTP requests and responses from Go applications.
package daytripper

import (
	"context"
	"errors"
	"github.com/swedishborgie/daytripper/har"
	"github.com/swedishborgie/daytripper/receiver"
	"net/http"
	"net/http/httptrace"
	"sync"
	"time"
)

var ErrNoReceiver = errors.New("no receiver configured")

// DayTripper is a request recorder that implements the http.RoundTripper interface. This can be used to transparently
// record conversations to and from servers from any library or application that supports injecting an http.Transport
// or http.Client.
type DayTripper struct {
	wrapped    http.RoundTripper
	version    *receiver.Version
	receiver   receiver.Receiver
	pageMWs    []receiver.PageMiddleware
	entryMWs   []receiver.EntryMiddleware
	pageMap    map[string]*har.Page
	pageMutex  sync.RWMutex
	includeAll bool

	sendEntry receiver.EntryReceiver
	sendPage  receiver.PageReceiver
}

// New creates a new DayTripper instance.
func New(opts ...Option) (*DayTripper, error) {
	dt := &DayTripper{
		version: &receiver.Version{
			HARVersion: "1.2",
			Version:    "0.1.0",
			Creator:    "daytripper",
		},
		includeAll: true,
		pageMap:    make(map[string]*har.Page),
		wrapped:    http.DefaultTransport,
	}

	for _, opt := range opts {
		opt(dt)
	}

	if dt.receiver == nil {
		return nil, ErrNoReceiver
	}

	if err := dt.receiver.Start(dt.version); err != nil {
		return nil, err
	}

	// Apply middlewares
	dt.sendEntry = dt.receiver.Entry
	for _, mw := range dt.entryMWs {
		dt.sendEntry = mw(dt.sendEntry)
	}

	dt.sendPage = dt.receiver.Page
	for _, mw := range dt.pageMWs {
		dt.sendPage = mw(dt.sendPage)
	}

	return dt, nil
}

// RoundTrip will
func (d *DayTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if !d.shouldInclude(req.Context()) {
		// Skip and forward along.
		return d.wrapped.RoundTrip(req)
	}

	d.handleStartPage(req.Context())

	report := &tripReport{
		req: req,
		entry: &har.Entry{
			Cache:   &har.Cache{}, // Firefox requires this to be present and not null.
			PageRef: pageFromCtx(req.Context()),
		},
	}

	timer := newTimingsTracker(report)

	req = req.WithContext(httptrace.WithClientTrace(req.Context(), timer.GetTracker()))
	if req.Body != nil {
		reqBodyCopier := newStreamCopier(req.Body, nil)
		req.Body = reqBodyCopier
		report.reqBody = reqBodyCopier
	}

	rsp, err := d.wrapped.RoundTrip(req)
	report.rspErr = err

	doneFunc := func() error {
		timer.responseRead()
		if err := d.recordTrip(report); err != nil {
			return err
		}
		return nil
	}

	var rspBodyCopier *streamCopier
	if rsp != nil {
		rspBodyCopier = newStreamCopier(rsp.Body, doneFunc)
		rsp.Body = rspBodyCopier
		report.rsp = rsp
		report.rspBody = rspBodyCopier
	} else {
		if err := doneFunc(); err != nil {
			return nil, err
		}
	}

	d.handleEndPage(req.Context())

	return rsp, err
}

func (d *DayTripper) Flush() error {
	if d.receiver == nil {
		return nil
	}

	return d.receiver.Flush()
}

func (d *DayTripper) Close() error {
	if d.receiver == nil {
		return nil
	}

	return d.receiver.Close()
}

func (d *DayTripper) shouldInclude(ctx context.Context) bool {
	if d.includeAll {
		return true
	}

	return ctx.Value(contextKeyInclude) != nil
}

func (d *DayTripper) handleStartPage(ctx context.Context) {
	pgInf := ctx.Value(contextKeyStartPage)
	if pgInf == nil {
		return
	}
	pg, ok := pgInf.(*har.Page)
	if !ok {
		return
	}

	d.pageMutex.Lock()
	d.pageMap[pg.ID] = pg
	d.pageMutex.Unlock()
}

func (d *DayTripper) handleEndPage(ctx context.Context) {
	if ctx.Value(contextKeyEndPage) == nil {
		return
	}

	pageID, ok := ctx.Value(contextKeyEndPage).(string)
	if !ok {
		return
	}

	d.pageMutex.Lock()
	defer d.pageMutex.Unlock()

	page := d.pageMap[pageID]
	if page == nil {
		return
	}

	page.PageTimings.OnLoad = har.DurationMS(time.Since(page.StartedDateTime))

	d.sendPage(page)
	delete(d.pageMap, pageID)
}
