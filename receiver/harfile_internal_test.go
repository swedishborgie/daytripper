package receiver

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHARFileReceiverFlushAfterClose(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	recv := NewHARFileReceiver(filepath.Join(dir, "test.har"))

	if err := recv.Start(&Version{HARVersion: "1.2", Creator: "test", Version: "0.1"}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Close the underlying file directly to force Seek to fail.
	if err := recv.fp.Close(); err != nil {
		t.Fatalf("closing fp directly: %v", err)
	}

	if err := recv.Flush(); err == nil {
		t.Fatal("Flush after closed fp: expected error, got nil")
	}
}

func TestHARFileReceiverFlushTruncateError(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	harPath := filepath.Join(dir, "test.har")
	recv := NewHARFileReceiver(harPath)

	if err := recv.Start(&Version{HARVersion: "1.2", Creator: "test", Version: "0.1"}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Close the writable file and replace fp with a read-only handle.
	// Seek(0,0) succeeds on a read-only file; Truncate fails with an error.
	_ = recv.fp.Close()
	roFile, err := os.OpenFile(harPath, os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("opening read-only file: %v", err)
	}
	recv.fp = roFile

	if err := recv.Flush(); err == nil {
		t.Fatal("Flush with read-only fp: expected error, got nil")
	}
}

func TestHARFileReceiverStartPermissionError(t *testing.T) {
	t.Parallel()

	recv := NewHARFileReceiver("/nonexistent/directory/test.har")
	if err := recv.Start(&Version{HARVersion: "1.2", Creator: "test", Version: "0.1"}); err == nil {
		t.Fatal("Start with bad path: expected error, got nil")
	}
}
