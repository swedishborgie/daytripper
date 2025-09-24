package checkpoint

import "time"

type Option func(r *Receiver)

// WithFileNameGenerator sets the file name generator to use. You can use this to change the naming scheme of the
// files.
func WithFileNameGenerator(nextFile FileNameGenerator) Option {
	return func(r *Receiver) {
		r.nextFile = nextFile
	}
}

// WithMaxBytes sets the maximum number of bytes that can be written to the file before it is rotated. If set to 0,
// the file will never be rotated based on file size.
func WithMaxBytes(maxBytes uint64) Option {
	return func(r *Receiver) {
		r.maxBytes = maxBytes
	}
}

// WithMaxDuration sets the maximum duration that can be written to the file before it is rotated. If set to 0, the
// file will never be rotated based on duration.
func WithMaxDuration(maxDuration time.Duration) Option {
	return func(r *Receiver) {
		r.maxDuration = maxDuration
	}
}
