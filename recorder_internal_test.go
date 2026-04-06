package daytripper

import (
	"crypto/tls"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"testing"
	"time"

	"github.com/swedishborgie/daytripper/har"
	"github.com/swedishborgie/daytripper/receiver"
)

func newTestDayTripper(t *testing.T) *DayTripper {
	t.Helper()
	recv := receiver.NewMemoryReceiver()
	dt := &DayTripper{
		version: &receiver.Version{HARVersion: "1.2", Creator: "test", Version: "0.1"},
	}
	dt.receiver = recv
	dt.sendEntry = recv.Entry
	dt.sendPage = recv.Page
	return dt
}

func TestRecordRequestNoHeaders(t *testing.T) {
	t.Parallel()

	dt := newTestDayTripper(t)
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/", nil)
	req.Header = http.Header{}

	report := &tripReport{
		req:   req,
		entry: &har.Entry{},
	}
	dt.recordRequest(report)

	// With no headers, only the trailing \r\n contributes (2 bytes).
	if report.entry.Request.HeadersSize != 2 {
		t.Errorf("HeadersSize = %d, want 2", report.entry.Request.HeadersSize)
	}
}

func TestRecordRequestSpecialCharQueryParams(t *testing.T) {
	t.Parallel()

	dt := newTestDayTripper(t)
	u, _ := url.Parse("http://example.com/path?key=hello%20world&empty=")
	req, _ := http.NewRequest(http.MethodGet, u.String(), nil)

	report := &tripReport{
		req:   req,
		entry: &har.Entry{},
	}
	dt.recordRequest(report)

	qs := report.entry.Request.QueryString
	if len(qs) != 2 {
		t.Fatalf("got %d query params, want 2", len(qs))
	}

	params := make(map[string]string)
	for _, q := range qs {
		params[q.Name] = q.Value
	}

	if params["key"] != "hello world" {
		t.Errorf("key = %q, want %q", params["key"], "hello world")
	}
	if params["empty"] != "" {
		t.Errorf("empty = %q, want %q", params["empty"], "")
	}
}

// TestTimingsTracker_SkippedHooks verifies that no timing field ends up with a bogus
// value (negative or astronomically large) when httptrace hooks are skipped.
//
// Several hooks are conditional: DNS and connect hooks are skipped on reused connections,
// TLS hooks are skipped for plain HTTP, and WroteRequest is not called for reused HTTP/2
// connections. A skipped "start" hook leaves its time.Time at the zero value; if the
// corresponding "done" computation still runs against it, time.Since(time.Time{}) produces
// a value close to math.MaxInt64 nanoseconds (~9.2e12 ms), breaking HAR consumers.
func TestTimingsTracker_SkippedHooks(t *testing.T) {
	t.Parallel()

	// maxReasonable is a generous upper bound for any timing on a loopback request.
	const maxReasonable = har.DurationMS(10 * time.Second)

	assertValidTiming := func(t *testing.T, name string, d har.DurationMS) {
		t.Helper()
		if d == har.DurationMSNotApplicable {
			return // -1 ms is the explicit "not applicable" sentinel — always valid
		}
		if d < 0 {
			t.Errorf("%s = %v, want >= 0 or DurationMSNotApplicable", name, d)
		}
		if d > maxReasonable {
			t.Errorf("%s = %v, want <= 10s — likely caused by time.Since(zero time.Time)", name, d)
		}
	}

	tests := []struct {
		name  string
		fire  func(hooks *httptrace.ClientTrace)
	}{
		{
			// Reused HTTP/2 connection: GetConn → GotConn → GotFirstResponseByte.
			// WroteRequest is never called for multiplexed HTTP/2 streams, leaving
			// startTimes.wait at zero — the original bug.
			name: "reused HTTP/2 connection (no WroteRequest)",
			fire: func(h *httptrace.ClientTrace) {
				h.GetConn("host:443")
				h.GotConn(httptrace.GotConnInfo{Reused: true})
				h.GotFirstResponseByte()
			},
		},
		{
			// Reused HTTP/1 connection from idle pool: same set of hooks, but
			// WroteRequest does fire for HTTP/1. Included for completeness.
			name: "reused HTTP/1 connection (WroteRequest fires)",
			fire: func(h *httptrace.ClientTrace) {
				h.GetConn("host:443")
				h.GotConn(httptrace.GotConnInfo{Reused: true})
				h.WroteRequest(httptrace.WroteRequestInfo{})
				h.GotFirstResponseByte()
			},
		},
		{
			// New plain-HTTP connection: no TLS hooks fire.
			// Connect hooks fire; TLSHandshake hooks are skipped.
			name: "new plain HTTP connection (no TLS hooks)",
			fire: func(h *httptrace.ClientTrace) {
				h.GetConn("host:80")
				h.DNSStart(httptrace.DNSStartInfo{Host: "host"})
				h.DNSDone(httptrace.DNSDoneInfo{})
				h.ConnectStart("tcp", "1.2.3.4:80")
				h.ConnectDone("tcp", "1.2.3.4:80", nil)
				h.GotConn(httptrace.GotConnInfo{})
				h.WroteRequest(httptrace.WroteRequestInfo{})
				h.GotFirstResponseByte()
			},
		},
		{
			// New HTTPS connection: all hooks fire.
			name: "new HTTPS connection (all hooks fire)",
			fire: func(h *httptrace.ClientTrace) {
				h.GetConn("host:443")
				h.DNSStart(httptrace.DNSStartInfo{Host: "host"})
				h.DNSDone(httptrace.DNSDoneInfo{})
				h.ConnectStart("tcp", "1.2.3.4:443")
				h.ConnectDone("tcp", "1.2.3.4:443", nil)
				h.TLSHandshakeStart()
				h.TLSHandshakeDone(tls.ConnectionState{}, nil)
				h.GotConn(httptrace.GotConnInfo{})
				h.WroteRequest(httptrace.WroteRequestInfo{})
				h.GotFirstResponseByte()
			},
		},
		{
			// GotFirstResponseByte never fires (server error / connection drop before
			// any response byte). Receive is handled via responseRead(), which already
			// guards against a zero startTimes.response. Included to confirm no panic.
			name: "no GotFirstResponseByte (connection error)",
			fire: func(h *httptrace.ClientTrace) {
				h.GetConn("host:443")
				h.GotConn(httptrace.GotConnInfo{Reused: true})
				// GotFirstResponseByte intentionally omitted
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			report := &tripReport{entry: &har.Entry{}}
			tracker := newTimingsTracker(report)
			hooks := tracker.GetTracker()

			tt.fire(hooks)
			tracker.responseRead()

			timings := report.entry.Timings
			assertValidTiming(t, "Blocked", timings.Blocked)
			assertValidTiming(t, "DNS", timings.DNS)
			assertValidTiming(t, "Connect", timings.Connect)
			assertValidTiming(t, "SSL", timings.SSL)
			assertValidTiming(t, "Send", timings.Send)
			assertValidTiming(t, "Wait", timings.Wait)
			assertValidTiming(t, "Receive", timings.Receive)
		})
	}
}

func TestHeaderSize(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		headers  http.Header
		expected uint64
	}{
		{
			name: "single header",
			headers: http.Header{
				"Foo": []string{"Bar"},
			},
			expected: 12,
		},
		{
			name: "multiple headers",
			headers: http.Header{
				"Foo": []string{"Bar"},
				"Baz": []string{"Qux"},
			},
			expected: 22,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actual := headerSize(tc.headers)

			if actual != tc.expected {
				t.Errorf("got %d, want %d", actual, tc.expected)
			}
		})
	}
}
