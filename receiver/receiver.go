package receiver

import "github.com/swedishborgie/daytripper/har"

// Version is a message sent to Receiver.Start so that a Receiver can be aware of the version context of the recorder.
type Version struct {
	// The version of the HAR spec to adhere to.
	HARVersion string
	// Version is the version of the creating utility.
	Version string
	// Creator is the name of the creating utility.
	Creator string
}

// Receiver is an interface capable of accepting page and entry information from the recorder.
type Receiver interface {
	// Start indicates the Receiver should initialize and ready itself to receive Entry's and Page's. This should only
	// be called once.
	Start(*Version) error
	// Entry receives an Entry from the recorder.
	Entry(entry *har.Entry) error
	// Page receives a Page from the recorder.
	Page(page *har.Page)
	// Flush should ensure that all information received is stored durably and consistently.
	Flush() error
	// Close should flush and close all open resources. The Receiver should not be used after this is called. This
	// should only be called once.
	Close() error
}

type (
	EntryReceiver   func(*har.Entry) error
	PageReceiver    func(*har.Page)
	EntryMiddleware func(EntryReceiver) EntryReceiver
	PageMiddleware  func(PageReceiver) PageReceiver
)
