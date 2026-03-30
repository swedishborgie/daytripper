package checkpoint_test

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/swedishborgie/daytripper/har"
	"github.com/swedishborgie/daytripper/receiver"
	"github.com/swedishborgie/daytripper/receiver/checkpoint"
)

func TestCheckpointFileNameGeneratorError(t *testing.T) {
	t.Parallel()

	genErr := errors.New("gen fail")
	recv := checkpoint.New(
		checkpoint.WithFileNameGenerator(func() (string, error) {
			return "", genErr
		}),
	)

	if err := recv.Start(&receiver.Version{}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	err := recv.Entry(&har.Entry{Comment: "test"})
	if err == nil {
		t.Fatal("Entry: expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to get next file name") {
		t.Errorf("error = %q, want it to contain %q", err.Error(), "failed to get next file name")
	}
}

func TestCheckpointCloseWithNoEntries(t *testing.T) {
	t.Parallel()

	recv := checkpoint.New(
		checkpoint.WithFileNameGenerator(
			checkpoint.TimestampFileGenerator(t.TempDir(), "test-", "2006-01-02_15-04-05.999"),
		),
	)

	if err := recv.Start(&receiver.Version{}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := recv.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestCheckpointNoRotationWhenMaxBytesZero(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	recv := checkpoint.New(
		checkpoint.WithMaxBytes(0),
		checkpoint.WithFileNameGenerator(
			checkpoint.TimestampFileGenerator(tmpDir, "test-", "2006-01-02_15-04-05.999"),
		),
	)

	if err := recv.Start(&receiver.Version{}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	for range 10 {
		if err := recv.Entry(&har.Entry{Comment: "entry"}); err != nil {
			t.Fatalf("Entry: %v", err)
		}
	}
	if err := recv.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 {
		t.Errorf("got %d files, want 1 (no rotation should occur)", len(files))
	}
}

func TestCheckpointDoubleStart(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	recv := checkpoint.New(
		checkpoint.WithFileNameGenerator(
			checkpoint.TimestampFileGenerator(tmpDir, "test-", "2006-01-02_15-04-05.999"),
		),
	)

	ver := &receiver.Version{HARVersion: "1.2", Creator: "test", Version: "0.1"}
	if err := recv.Start(ver); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	// Trigger file creation so harWriter is non-nil.
	if err := recv.Entry(&har.Entry{Comment: "entry"}); err != nil {
		t.Fatalf("Entry: %v", err)
	}
	// Second Start should call harWriter.Start (not recurse infinitely after fix).
	if err := recv.Start(ver); err != nil {
		t.Fatalf("second Start: %v", err)
	}
}

func TestReceiverWithMaxBytes(t *testing.T) {
	tmpDir := t.TempDir()

	recv := checkpoint.New(
		checkpoint.WithMaxBytes(100),
		checkpoint.WithFileNameGenerator(
			checkpoint.TimestampFileGenerator(tmpDir, "test-file-", "2006-01-02_15-04-05.999"),
		),
	)

	if err := recv.Start(&receiver.Version{}); err != nil {
		t.Fatal(err)
	}

	recv.Page(&har.Page{Comment: "test page"})

	for range 3 {
		if err := recv.Entry(&har.Entry{Comment: "test entry"}); err != nil {
			t.Fatal(err)
		}
	}

	if err := recv.Flush(); err != nil {
		t.Fatal(err)
	}

	if err := recv.Close(); err != nil {
		t.Fatal(err)
	}

	// There should be two files.
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("Expected 2 files, got %d", len(files))
	}

	for f := range files {
		t.Logf("File: %s", files[f].Name())
		fstat, _ := files[f].Info()
		if fstat.Size() == 0 {
			t.Fatalf("File %s is empty", files[f].Name())
		}
	}
}

func TestReceiverWithMaxDuration(t *testing.T) {
	tmpDir := t.TempDir()

	recv := checkpoint.New(
		checkpoint.WithMaxDuration(time.Nanosecond),
		checkpoint.WithFileNameGenerator(
			checkpoint.TimestampFileGenerator(tmpDir, "test-file-", "2006-01-02_15-04-05.999"),
		),
	)

	if err := recv.Start(&receiver.Version{}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(time.Nanosecond)

	for range 4 {
		if err := recv.Entry(&har.Entry{Comment: "test entry"}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(time.Nanosecond)
	}

	if err := recv.Close(); err != nil {
		t.Fatal(err)
	}

	// There should be two files.
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 4 {
		t.Fatalf("Expected 4 files, got %d", len(files))
	}
	for f := range files {
		t.Logf("File: %s", files[f].Name())
		fstat, _ := files[f].Info()
		if fstat.Size() == 0 {
			t.Fatalf("File %s is empty", files[f].Name())
		}
	}
}
