package checkpoint_test

import (
	"os"
	"testing"
	"time"

	"github.com/swedishborgie/daytripper/har"
	"github.com/swedishborgie/daytripper/receiver"
	"github.com/swedishborgie/daytripper/receiver/checkpoint"
)

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
