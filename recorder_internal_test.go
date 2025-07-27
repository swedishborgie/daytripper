package daytripper

import (
	"net/http"
	"testing"
)

func TestHeaderSize(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		headers  http.Header
		expected uint64
	}{
		{
			name: "single header",
			headers: http.Header{
				"Foo": []string{"Bar"},
			},
			expected: 12,
		},
		{
			name: "multiple headers",
			headers: http.Header{
				"Foo": []string{"Bar"},
				"Baz": []string{"Qux"},
			},
			expected: 22,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			actual := headerSize(tc.headers)

			if actual != tc.expected {
				t.Errorf("got %d, want %d", actual, tc.expected)
			}
		})
	}
}
