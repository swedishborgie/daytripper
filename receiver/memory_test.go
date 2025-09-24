package receiver_test

import (
	"testing"

	"github.com/swedishborgie/daytripper/har"
	"github.com/swedishborgie/daytripper/receiver"
)

func TestMemoryReceiver(t *testing.T) {
	mem := receiver.NewMemoryReceiver()
	if err := mem.Start(&receiver.Version{}); err != nil {
		t.Fatal(err)
	}
	mem.Page(&har.Page{})
	if err := mem.Entry(&har.Entry{}); err != nil {
		t.Fatal(err)
	}
	if err := mem.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := mem.Close(); err != nil {
		t.Fatal(err)
	}
}
