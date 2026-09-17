package tracechk_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/tracechk"
	"github.com/google/go-cmp/cmp"
)

func TestReadTSV(t *testing.T) {
	type testCase struct {
		Input string
		// Want is the events; nil when an error is wanted.
		Want []csdf.Event
	}

	testCases := map[string]testCase{
		"one event per row under an event header": {
			Input: "event\ninsert(coin)\nshowPurchasable(purchasableProducts)\n",
			Want:  []csdf.Event{"insert(coin)", "showPurchasable(purchasableProducts)"},
		},
		"quoted fields follow CSV with a tab delimiter, CRLF and a missing last newline are accepted, blank lines are skipped": {
			Input: "event\r\n\"a\tb\"\r\n\n\"say \"\"hi\"\"\"\nplain (x)",
			Want:  []csdf.Event{"a\tb", "say \"hi\"", "plain (x)"},
		},
		"header only (lower boundary value)": {
			Input: "event\n",
			Want:  []csdf.Event{},
		},
		"missing header":           {Input: "insert(coin)\n"},
		"empty input":              {Input: ""},
		"header in the wrong case": {Input: "Event\ninsert(coin)\n"},
		"tau is not visible":       {Input: "event\ntau\n"},
		"two columns":              {Input: "event\ninsert(coin)\tfoo\n"},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			r := strings.NewReader(testCase.Input)

			// Act
			got, err := tracechk.ReadTSV(r)

			// Assert
			if testCase.Want == nil {
				if err == nil {
					t.Errorf("want an error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("want nil, got %v", err)
			}
			if diff := cmp.Diff(testCase.Want, got); diff != "" {
				t.Error(diff)
			}
		})
	}
}

func TestReadTraceNamesTheTraceInItsError(t *testing.T) {
	// Arrange
	r := strings.NewReader("no header\n")

	// Act
	_, err := tracechk.ReadTrace("trace.tsv", r)

	// Assert
	var readErr *tracechk.ReadError
	if !errors.As(err, &readErr) {
		t.Fatalf("want a *ReadError, got %#v", err)
	}
	if want := "trace.tsv: want a header row \"event\""; err.Error() != want {
		t.Errorf("want %q, got %q", want, err.Error())
	}
}
