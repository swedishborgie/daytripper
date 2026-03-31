package streaming_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/swedishborgie/daytripper/har"
	"github.com/swedishborgie/daytripper/receiver"
	"github.com/swedishborgie/daytripper/receiver/streaming"
)

var testVersion = &receiver.Version{
	HARVersion: "1.2",
	Creator:    "test_creator",
	Version:    "1.2.3",
}

func newRecv(t *testing.T) (*streaming.Receiver, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	recv := streaming.New(&buf)
	if err := recv.Start(testVersion); err != nil {
		t.Fatalf("Start: %v", err)
	}
	return recv, &buf
}

func decodeHAR(t *testing.T, buf *bytes.Buffer) *har.HTTPArchive {
	t.Helper()
	archive := &har.HTTPArchive{}
	if err := json.NewDecoder(buf).Decode(archive); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return archive
}

// TestReceiver_Basic checks a single entry + single page round-trip, including version and creator metadata.
func TestReceiver_Basic(t *testing.T) {
	recv, buf := newRecv(t)

	if err := recv.Entry(&har.Entry{Comment: "test entry"}); err != nil {
		t.Fatal(err)
	}
	recv.Page(&har.Page{Comment: "test page"})

	if err := recv.Close(); err != nil {
		t.Fatal(err)
	}

	archive := decodeHAR(t, buf)
	if got := archive.Log.Entries[0].Comment; got != "test entry" {
		t.Errorf("entry comment: got %q, want %q", got, "test entry")
	}
	if got := archive.Log.Pages[0].Comment; got != "test page" {
		t.Errorf("page comment: got %q, want %q", got, "test page")
	}
	if got := archive.Log.Version; got != "1.2" {
		t.Errorf("version: got %q, want %q", got, "1.2")
	}
	if got := archive.Log.Creator.Name; got != "test_creator" {
		t.Errorf("creator name: got %q, want %q", got, "test_creator")
	}
	if got := archive.Log.Creator.Version; got != "1.2.3" {
		t.Errorf("creator version: got %q, want %q", got, "1.2.3")
	}
}

// TestReceiver_MultipleEntries checks that a large number of entries survive the round-trip.
func TestReceiver_MultipleEntries(t *testing.T) {
	recv, buf := newRecv(t)

	for i := range 100 {
		if err := recv.Entry(&har.Entry{Comment: "entry"}); err != nil {
			t.Fatalf("Entry %d: %v", i, err)
		}
	}
	if err := recv.Close(); err != nil {
		t.Fatal(err)
	}

	archive := decodeHAR(t, buf)
	if got := len(archive.Log.Entries); got != 100 {
		t.Errorf("entry count: got %d, want 100", got)
	}
}

// TestReceiver_MultiplePages checks interleaved pages and entries.
func TestReceiver_MultiplePages(t *testing.T) {
	recv, buf := newRecv(t)

	recv.Page(&har.Page{Comment: "page 0"})
	for range 5 {
		if err := recv.Entry(&har.Entry{}); err != nil {
			t.Fatal(err)
		}
	}
	recv.Page(&har.Page{Comment: "page 1"})
	for range 3 {
		if err := recv.Entry(&har.Entry{}); err != nil {
			t.Fatal(err)
		}
	}
	recv.Page(&har.Page{Comment: "page 2"})
	for range 2 {
		if err := recv.Entry(&har.Entry{}); err != nil {
			t.Fatal(err)
		}
	}

	if err := recv.Close(); err != nil {
		t.Fatal(err)
	}

	archive := decodeHAR(t, buf)
	if got := len(archive.Log.Pages); got != 3 {
		t.Errorf("page count: got %d, want 3", got)
	}
	if got := len(archive.Log.Entries); got != 10 {
		t.Errorf("entry count: got %d, want 10", got)
	}
}

// TestReceiver_ZeroEntries checks that a session with only pages produces valid HAR.
func TestReceiver_ZeroEntries(t *testing.T) {
	recv, buf := newRecv(t)

	recv.Page(&har.Page{Comment: "lonely page"})

	if err := recv.Close(); err != nil {
		t.Fatal(err)
	}

	archive := decodeHAR(t, buf)
	if got := len(archive.Log.Entries); got != 0 {
		t.Errorf("entry count: got %d, want 0", got)
	}
	if got := len(archive.Log.Pages); got != 1 {
		t.Errorf("page count: got %d, want 1", got)
	}
}

// TestReceiver_ZeroPages checks that a session with only entries produces valid HAR.
func TestReceiver_ZeroPages(t *testing.T) {
	recv, buf := newRecv(t)

	for range 3 {
		if err := recv.Entry(&har.Entry{}); err != nil {
			t.Fatal(err)
		}
	}

	if err := recv.Close(); err != nil {
		t.Fatal(err)
	}

	archive := decodeHAR(t, buf)
	if got := len(archive.Log.Entries); got != 3 {
		t.Errorf("entry count: got %d, want 3", got)
	}
	if got := len(archive.Log.Pages); got != 0 {
		t.Errorf("page count: got %d, want 0", got)
	}
}

// TestReceiver_ZeroEntriesZeroPages checks that Start + Close alone produces valid HAR.
func TestReceiver_ZeroEntriesZeroPages(t *testing.T) {
	recv, buf := newRecv(t)

	if err := recv.Close(); err != nil {
		t.Fatal(err)
	}

	archive := decodeHAR(t, buf)
	if got := len(archive.Log.Entries); got != 0 {
		t.Errorf("entry count: got %d, want 0", got)
	}
	if got := len(archive.Log.Pages); got != 0 {
		t.Errorf("page count: got %d, want 0", got)
	}
}

// TestReceiver_FlushThenMoreEntries checks that Flush() does not finalize the JSON and that entries written after
// Flush() are still included in the final output.
func TestReceiver_FlushThenMoreEntries(t *testing.T) {
	recv, buf := newRecv(t)

	for range 3 {
		if err := recv.Entry(&har.Entry{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := recv.Flush(); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := recv.Entry(&har.Entry{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := recv.Close(); err != nil {
		t.Fatal(err)
	}

	archive := decodeHAR(t, buf)
	if got := len(archive.Log.Entries); got != 6 {
		t.Errorf("entry count: got %d, want 6", got)
	}
}

// TestReceiver_PageAfterEntry checks that a page added after entries are written is still present in the output.
func TestReceiver_PageAfterEntry(t *testing.T) {
	recv, buf := newRecv(t)

	if err := recv.Entry(&har.Entry{Comment: "first"}); err != nil {
		t.Fatal(err)
	}
	recv.Page(&har.Page{Comment: "mid-session page"})
	if err := recv.Entry(&har.Entry{Comment: "second"}); err != nil {
		t.Fatal(err)
	}

	if err := recv.Close(); err != nil {
		t.Fatal(err)
	}

	archive := decodeHAR(t, buf)
	if got := len(archive.Log.Entries); got != 2 {
		t.Errorf("entry count: got %d, want 2", got)
	}
	if got := len(archive.Log.Pages); got != 1 {
		t.Errorf("page count: got %d, want 1", got)
	}
	if got := archive.Log.Pages[0].Comment; got != "mid-session page" {
		t.Errorf("page comment: got %q, want %q", got, "mid-session page")
	}
}

// TestReceiver_WriteError checks that a write error from the underlying writer is surfaced.
func TestReceiver_WriteError(t *testing.T) {
	recv := streaming.New(&errWriter{})
	err := recv.Start(testVersion)
	if err == nil {
		t.Fatal("expected error from failing writer, got nil")
	}
}

// TestReceiver_DoubleClose checks that calling Close twice is safe.
func TestReceiver_DoubleClose(t *testing.T) {
	recv, _ := newRecv(t)

	if err := recv.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := recv.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

// TestReceiver_ConcurrentEntries checks thread safety: 50 goroutines each write one entry.
func TestReceiver_ConcurrentEntries(t *testing.T) {
	recv, buf := newRecv(t)

	const n = 50
	var wg sync.WaitGroup
	wg.Add(n)
	for range n {
		go func() {
			defer wg.Done()
			if err := recv.Entry(&har.Entry{}); err != nil {
				t.Errorf("concurrent Entry: %v", err)
			}
		}()
	}
	wg.Wait()

	if err := recv.Close(); err != nil {
		t.Fatal(err)
	}

	archive := decodeHAR(t, buf)
	if got := len(archive.Log.Entries); got != n {
		t.Errorf("entry count: got %d, want %d", got, n)
	}
}

// errWriter is an io.Writer that always returns an error.
type errWriter struct{}

func (e *errWriter) Write(_ []byte) (int, error) {
	return 0, fmt.Errorf("write error")
}
