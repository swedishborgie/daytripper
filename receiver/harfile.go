package receiver

import (
	"encoding/json"
	"github.com/swedishborgie/daytripper/har"
	"os"
	"sync"
)

// HARFileReceiver is a basic implementation of a receiver that stores state in memory and flushes to a single file.
// This is a good option if you have a small number of requests, but you'll run into issues if you have too many
// requests or if those requests are too large.
//
// With this implementation every time you flush, the entire archive needs to be re-written from the beginning.
// You should either flush infrequently or flush on close.
type HARFileReceiver struct {
	mutex    sync.Mutex
	entries  []*har.Entry
	pages    []*har.Page
	encoder  *json.Encoder
	fileName string
	fp       *os.File
	version  *Version
}

// NewHARFileReceiver creates a new receiver with a given file name. The file won't be opened until
// HARFileReceiver.Start is called.
func NewHARFileReceiver(fileName string) *HARFileReceiver {
	return &HARFileReceiver{
		fileName: fileName,
		pages:    make([]*har.Page, 0),
		entries:  make([]*har.Entry, 0),
	}
}

// Start opens the file and sets up the encoder.
func (s *HARFileReceiver) Start(version *Version) error {
	s.version = version

	fp, err := os.Create(s.fileName)
	if err != nil {
		return err
	}
	s.fp = fp

	s.encoder = json.NewEncoder(fp)

	return nil
}

// Entry receives a single HTTP request / response pair.
func (s *HARFileReceiver) Entry(entry *har.Entry) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.entries = append(s.entries, entry)
	return nil
}

// Page receives a new page.
func (s *HARFileReceiver) Page(page *har.Page) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.pages = append(s.pages, page)
}

// Flush flushes the entire archive to disk. It will truncate the file and re-write the entire state.
func (s *HARFileReceiver) Flush() error {
	harLog := &har.HTTPArchive{
		Log: &har.Log{
			Version: s.version.HARVersion,
			Creator: &har.Agent{
				Name:    s.version.Creator,
				Version: s.version.Version,
			},
			Pages:   s.pages,
			Entries: s.entries,
		},
	}

	if _, err := s.fp.Seek(0, 0); err != nil {
		return err
	}

	if err := s.fp.Truncate(0); err != nil {
		return err
	}

	if err := s.encoder.Encode(harLog); err != nil {
		return err
	}

	return nil
}

// Close will flush and close the file, it will also dispose of all the recorded entries and pages.
func (s *HARFileReceiver) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if err := s.Flush(); err != nil {
		return err
	}

	s.pages = make([]*har.Page, 0)
	s.entries = make([]*har.Entry, 0)

	return s.fp.Close()
}
