package streaming

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/swedishborgie/daytripper/har"
	"github.com/swedishborgie/daytripper/receiver"
)

// Receiver is a receiver that writes entries to the provided writer as they arrive, rather than buffering them all
// in memory. Pages are still buffered in memory (there are typically very few per session).
//
// The output is not valid JSON until Close() is called, which writes the pages array and closes the JSON structure.
// Flush() pushes the internal write buffer to the underlying writer for durability, but does not produce parseable
// output on its own.
//
// Unlike receiver.HARFileReceiver, this implementation writes the "entries" array before the "pages" array in the
// JSON output. This ordering is valid per the JSON specification (object field order is not mandated) and is handled
// correctly by HAR viewers.
//
// The caller is responsible for closing the underlying writer after Close() returns.
type Receiver struct {
	mutex      sync.Mutex
	pages      []*har.Page
	version    *receiver.Version
	bw         *bufio.Writer // 64 KiB buffer over the provided writer
	entryCount int           // tracks comma-prefix logic
	closed     bool          // idempotent close guard
}

// New creates a new Receiver that writes HAR output to w. Start must be called before any entries or pages are
// accepted. The caller retains ownership of w and is responsible for closing it after Close() returns.
func New(w io.Writer) *Receiver {
	return &Receiver{
		bw:    bufio.NewWriterSize(w, 64*1024),
		pages: make([]*har.Page, 0),
	}
}

// Start writes the HAR JSON prologue to the underlying writer.
func (r *Receiver) Start(version *receiver.Version) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.version = version

	creatorBytes, err := json.Marshal(&har.Agent{
		Name:    version.Creator,
		Version: version.Version,
	})
	if err != nil {
		return err
	}

	versionBytes, err := json.Marshal(version.HARVersion)
	if err != nil {
		return err
	}

	if _, err = fmt.Fprintf(r.bw, `{"log":{"version":%s,"creator":%s,"entries":[`, versionBytes, creatorBytes); err != nil {
		return err
	}

	return r.bw.Flush()
}

// Entry writes a single HTTP request/response pair to the underlying writer.
func (r *Receiver) Entry(entry *har.Entry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.entryCount > 0 {
		if _, err := r.bw.WriteString(",\n"); err != nil {
			return err
		}
	}

	if _, err := r.bw.Write(data); err != nil {
		return err
	}

	r.entryCount++
	return nil
}

// Page buffers a page in memory. Pages are written to the underlying writer during Close().
func (r *Receiver) Page(page *har.Page) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.pages = append(r.pages, page)
}

// Flush pushes the internal write buffer to the underlying writer. The output is not valid JSON after this call;
// use Close() to produce a parseable HAR document.
func (r *Receiver) Flush() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	return r.bw.Flush()
}

// Close finalizes the HAR JSON by writing the pages array and closing the JSON structure, then flushes the internal
// buffer. The caller is responsible for closing the underlying writer afterward.
// Calling Close more than once is safe; subsequent calls are no-ops.
func (r *Receiver) Close() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.closed {
		return nil
	}
	r.closed = true

	var retErr error

	capture := func(err error) {
		if err != nil && retErr == nil {
			retErr = err
		}
	}

	// Close the entries array and open the pages array.
	_, err := r.bw.WriteString(`],"pages":[`)
	capture(err)

	for i, page := range r.pages {
		if i > 0 {
			_, err = r.bw.WriteString(",\n")
			capture(err)
		}

		data, err := json.Marshal(page)
		capture(err)

		_, err = r.bw.Write(data)
		capture(err)
	}

	// Close pages array, log object, and root object. Trailing newline matches json.Encoder behaviour.
	_, err = r.bw.WriteString("]}}\n")
	capture(err)

	capture(r.bw.Flush())

	return retErr
}
