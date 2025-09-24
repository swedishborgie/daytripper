package receiver

import (
	"sync"

	"github.com/swedishborgie/daytripper/har"
)

// MemoryReceiver simply buffers pages and entries in memory. It doesn't make any attempt to save anything. Mostly
// useful for tests or use cases with a low number of requests.
type MemoryReceiver struct {
	Version *Version
	Pages   []*har.Page
	Entries []*har.Entry
	mutex   sync.Mutex
}

func NewMemoryReceiver() *MemoryReceiver {
	return &MemoryReceiver{
		Version: &Version{},
	}
}

func (m *MemoryReceiver) Start(version *Version) error {
	m.Version = version

	return nil
}

func (m *MemoryReceiver) Entry(entry *har.Entry) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.Entries = append(m.Entries, entry)
	return nil
}

func (m *MemoryReceiver) Page(page *har.Page) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.Pages = append(m.Pages, page)
}

func (m *MemoryReceiver) Flush() error {
	return nil
}

func (m *MemoryReceiver) Close() error {
	return m.Flush()
}
