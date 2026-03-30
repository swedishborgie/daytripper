package daytripper

import (
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"
)

type callbackCounter struct {
	count int
	err   error
}

func (c *callbackCounter) cb() error {
	c.count++
	return c.err
}

func TestStreamCopierDoubleClose(t *testing.T) {
	t.Parallel()

	cc := &callbackCounter{}
	sc := newStreamCopier(io.NopCloser(strings.NewReader("hello")), cc.cb, 0)

	if err := sc.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := sc.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	if cc.count != 1 {
		t.Errorf("callback fired %d times, want 1", cc.count)
	}
}

func TestStreamCopierCloseWithoutRead(t *testing.T) {
	t.Parallel()

	cc := &callbackCounter{}
	sc := newStreamCopier(io.NopCloser(strings.NewReader("hello")), cc.cb, 0)

	if err := sc.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if cc.count != 1 {
		t.Errorf("callback fired %d times, want 1", cc.count)
	}
	if sc.buffer.Len() != 0 {
		t.Errorf("buffer has %d bytes, want 0", sc.buffer.Len())
	}
}

func TestStreamCopierEmptyBody(t *testing.T) {
	t.Parallel()

	cc := &callbackCounter{}
	sc := newStreamCopier(io.NopCloser(strings.NewReader("")), cc.cb, 0)

	buf := make([]byte, 16)
	n, err := sc.Read(buf)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("Read error = %v, want io.EOF", err)
	}
	if n != 0 {
		t.Errorf("Read returned %d bytes, want 0", n)
	}
	if cc.count != 1 {
		t.Errorf("callback fired %d times, want 1", cc.count)
	}
	if sc.count != 0 {
		t.Errorf("byte count = %d, want 0", sc.count)
	}
}

func TestStreamCopierReadError(t *testing.T) {
	t.Parallel()

	readErr := errors.New("read fail")
	cc := &callbackCounter{}
	sc := newStreamCopier(io.NopCloser(iotest.ErrReader(readErr)), cc.cb, 0)

	buf := make([]byte, 16)
	_, err := sc.Read(buf)
	if !errors.Is(err, readErr) {
		t.Fatalf("Read error = %v, want %v", err, readErr)
	}
	// Non-EOF error should NOT trigger the callback.
	if cc.count != 0 {
		t.Errorf("callback fired %d times, want 0 (non-EOF error should not notify)", cc.count)
	}
}

func TestStreamCopierCallbackErrorOnEOF(t *testing.T) {
	t.Parallel()

	cbErr := errors.New("callback fail")
	cc := &callbackCounter{err: cbErr}
	sc := newStreamCopier(io.NopCloser(strings.NewReader("")), cc.cb, 0)

	buf := make([]byte, 16)
	_, err := sc.Read(buf)
	if !errors.Is(err, cbErr) {
		t.Fatalf("Read error = %v, want %v", err, cbErr)
	}
}

func TestStreamCopierCallbackErrorOnClose(t *testing.T) {
	t.Parallel()

	cbErr := errors.New("callback fail")
	cc := &callbackCounter{err: cbErr}
	sc := newStreamCopier(io.NopCloser(strings.NewReader("data")), cc.cb, 0)

	err := sc.Close()
	if !errors.Is(err, cbErr) {
		t.Fatalf("Close error = %v, want %v", err, cbErr)
	}
}
