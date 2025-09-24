package checkpoint

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/swedishborgie/daytripper/har"
	"github.com/swedishborgie/daytripper/receiver"
)

// Receiver wraps receiver.HARFileReceiver and adds a checkpointing mechanism. Instead of buffering all entries in
// memory, it will write them to disk every so often with a given file pattern and flush them to disk when the
// buffer reaches a certain size or duration.
type Receiver struct {
	nextFile    FileNameGenerator
	maxBytes    uint64
	maxDuration time.Duration
	version     *receiver.Version

	mutex        sync.Mutex
	currentBytes uint64
	lastRoll     time.Time
	harWriter    *receiver.HARFileReceiver
}

// FileNameGenerator is a function that returns a file name for the next HAR file. This should always return a new
// file name every time it is called.
type FileNameGenerator func() (string, error)

func New(opts ...Option) *Receiver {
	r := &Receiver{
		maxBytes:    10 * 1024 * 1024, // 10 MB
		nextFile:    defaultFileNameGenerator(),
		maxDuration: 0,
	}

	for _, o := range opts {
		o(r)
	}

	return r
}

func (r *Receiver) Start(version *receiver.Version) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.version = version

	if r.harWriter == nil {
		return nil
	}

	return r.Start(version)
}

func (r *Receiver) Entry(entry *har.Entry) error {
	if err := r.rotateIfNeeded(entry); err != nil {
		return err
	}

	return r.harWriter.Entry(entry)
}

func (r *Receiver) Page(page *har.Page) {
	if err := r.rotateIfNeeded(page); err != nil {
		return
	}

	if r.harWriter != nil {
		r.harWriter.Page(page)
	}
}

func (r *Receiver) Flush() error {
	if err := r.rotateIfNeeded(nil); err != nil {
		return err
	}

	return r.harWriter.Flush()
}

func (r *Receiver) Close() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.harWriter == nil {
		return nil
	}

	return r.harWriter.Close()
}

func (r *Receiver) rotateIfNeeded(toAdd any) error {
	rotate, err := r.shouldRotate(toAdd)
	if err != nil {
		return fmt.Errorf("failed to determine if we should rotate: %w", err)
	}

	if rotate {
		return r.doRotate()
	}

	return nil
}

func (r *Receiver) doRotate() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.harWriter != nil {
		if err := r.harWriter.Close(); err != nil {
			return fmt.Errorf("failed to close HAR file: %w", err)
		}
	}

	r.lastRoll = time.Now()
	r.currentBytes = 0

	fileName, err := r.nextFile()
	if err != nil {
		return fmt.Errorf("failed to get next file name: %w", err)
	}

	r.harWriter = receiver.NewHARFileReceiver(fileName)

	return r.harWriter.Start(r.version)
}

func (r *Receiver) shouldRotate(toAdd any) (bool, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.harWriter == nil {
		return true, nil
	}

	if r.maxBytes > 0 && toAdd != nil {
		sz, err := json.Marshal(toAdd)
		if err != nil {
			return false, fmt.Errorf("failed to get size of entry: %w", err)
		}
		r.currentBytes += uint64(len(sz))

		if r.currentBytes > r.maxBytes {
			return true, nil
		}
	}

	if r.maxDuration > 0 {
		if time.Since(r.lastRoll) > r.maxDuration {
			return true, nil
		}
	}

	return false, nil
}

func defaultFileNameGenerator() FileNameGenerator {
	return TimestampFileGenerator(".", "daytripper-", "2006-01-02_15-04-05.999")
}

func TimestampFileGenerator(basePath, prefix, format string) FileNameGenerator {
	var inc atomic.Int64
	return func() (string, error) {
		tm := time.Now().Format(format)
		count := inc.Add(1)
		return filepath.Join(basePath, fmt.Sprintf("%s%s-%d.har", prefix, tm, count)), nil
	}
}
