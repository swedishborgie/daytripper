package daytripper

import (
	"net/http"
	"net/url"
	"testing"

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
