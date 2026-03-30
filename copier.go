package daytripper

import (
	"bytes"
	"errors"
	"io"
	"sync"
)

// streamCloseCallback is a callback function to call when the stream is closed.
type streamCloseCallback func() error

// streamCopier will copy a stream as it gets read, this will ensure we don't change the behavior between the client
// and server. We passively observe the results. Someday we may want to cause this to offload to disk if the stream
// gets too big.
type streamCopier struct {
	buffer    bytes.Buffer
	wrapped   io.ReadCloser
	count     uint64
	maxSize   int64
	truncated bool

	cb         streamCloseCallback
	closed     bool
	done       chan bool
	closeMutex sync.Mutex
}

func newStreamCopier(wrapped io.ReadCloser, cb streamCloseCallback, maxSize int64) *streamCopier {
	return &streamCopier{wrapped: wrapped, done: make(chan bool), cb: cb, maxSize: maxSize}
}

func (s *streamCopier) Read(p []byte) (n int, err error) {
	cnt, err := s.wrapped.Read(p)
	s.count += uint64(cnt)

	if s.maxSize <= 0 || int64(s.buffer.Len()) < s.maxSize {
		canBuffer := cnt
		if s.maxSize > 0 {
			remaining := s.maxSize - int64(s.buffer.Len())
			if int64(canBuffer) > remaining {
				canBuffer = int(remaining)
				s.truncated = true
			}
		}
		s.buffer.Write(p[:canBuffer])
	}

	if errors.Is(err, io.EOF) {
		if err := s.closeNotify(); err != nil {
			return 0, err
		}
	}

	return cnt, err
}

func (s *streamCopier) Close() error {
	if err := s.closeNotify(); err != nil {
		return err
	}

	return s.wrapped.Close()
}

// closeNotify will be called either when the stream is closed naturally (e.g. EOF) or when the stream is explicitly
// closed. A signal is
func (s *streamCopier) closeNotify() error {
	s.closeMutex.Lock()
	if s.closed {
		return nil
	}
	close(s.done)
	s.closed = true
	s.closeMutex.Unlock()

	if s.cb != nil {
		return s.cb()
	}

	return nil
}
