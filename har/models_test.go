package har_test

import (
	"encoding/json"
	"github.com/swedishborgie/daytripper/har"
	"testing"
	"time"
)

func TestTimeMSJSON(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		Name     string
		Input    time.Time
		Expected string
	}{
		{
			Name:     "time",
			Input:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			Expected: "1735689600000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			tm := har.TimeMS(tc.Input)

			// Test Marshal
			b, err := json.Marshal(tm)
			if err != nil {
				t.Fatal(err)
				return
			}

			if string(b) != tc.Expected {
				t.Errorf("got %s, want %s", string(b), tc.Expected)
				t.Fail()
			}

			var newTM har.TimeMS

			if err := json.Unmarshal(b, &newTM); err != nil {
				t.Fatal(err)
				return
			}

			// Test Unmarshal
			if time.Time(newTM).UnixNano() != tc.Input.UnixNano() {
				t.Errorf("got %s, want %s", time.Time(newTM), tc.Input)
			}
		})
	}
}

func TestTimeMSJSONError(t *testing.T) {
	tm := &har.TimeMS{}

	if err := json.Unmarshal([]byte(`"string"`), tm); err == nil {
		t.Errorf("got nil, want error")
	}
}

func TestDurationMSJSON(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		Name     string
		Input    time.Duration
		Expected string
	}{
		{
			Name:     "time",
			Input:    time.Second,
			Expected: "1000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			tm := har.DurationMS(tc.Input)

			// Test Marshal
			b, err := json.Marshal(tm)
			if err != nil {
				t.Fatal(err)
				return
			}

			if string(b) != tc.Expected {
				t.Errorf("got %s, want %s", string(b), tc.Expected)
				t.Fail()
			}

			var newTM har.DurationMS

			if err := json.Unmarshal(b, &newTM); err != nil {
				t.Fatal(err)
				return
			}

			// Test Unmarshal
			if time.Duration(newTM) != tc.Input {
				t.Errorf("got %s, want %s", time.Duration(newTM), tc.Input)
			}
		})
	}
}

func TestDurationMSJSONError(t *testing.T) {
	var tm har.DurationMS

	if err := json.Unmarshal([]byte(`"string"`), &tm); err == nil {
		t.Errorf("got nil, want error")
	}
}
