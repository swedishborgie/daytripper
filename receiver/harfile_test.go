package receiver_test

import (
	"encoding/json"
	"github.com/swedishborgie/daytripper/har"
	"github.com/swedishborgie/daytripper/receiver"
	"os"
	"testing"
)

func TestHarFileReceiver(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-*.har")
	if err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := os.Remove(tmpFile.Name()); err != nil {
			t.Fatal(err)
		}
	}()

	recv := receiver.NewHARFileReceiver(tmpFile.Name())
	if err := recv.Start(&receiver.Version{
		HARVersion: "1.2",
		Creator:    "test_creator",
		Version:    "1.2.3",
	}); err != nil {
		t.Fatal(err)
	}

	if err := recv.Entry(&har.Entry{Comment: "test entry"}); err != nil {
		t.Fatal(err)
	}

	recv.Page(&har.Page{Comment: "test page"})

	if err := recv.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := recv.Close(); err != nil {
		t.Fatal(err)
	}

	fp, err := os.Open(tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}

	harFile := &har.HTTPArchive{}
	if err := json.NewDecoder(fp).Decode(harFile); err != nil {
		t.Fatal(err)
	}

	if harFile.Log.Entries[0].Comment != "test entry" {
		t.Errorf("got %s, want %s", harFile.Log.Entries[0].Comment, "test entry")
	}
	if harFile.Log.Pages[0].Comment != "test page" {
		t.Errorf("got %s, want %s", harFile.Log.Pages[0].Comment, "test page")
	}
	if harFile.Log.Version != "1.2" {
		t.Errorf("got %s, want %s", harFile.Log.Version, "1.2")
	}
	if harFile.Log.Creator.Name != "test_creator" {
		t.Errorf("got %s, want %s", harFile.Log.Creator.Name, "test_creator")
	}
	if harFile.Log.Creator.Version != "1.2.3" {
		t.Errorf("got %s, want %s", harFile.Log.Creator.Version, "1.2.3")
	}
}
