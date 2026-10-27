package core_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
	"github.com/google/go-cmp/cmp"
)

func TestSplitEvent(t *testing.T) {
	testCases := map[string]struct {
		Text    string
		Field   core.FieldRule
		Channel string
		Fields  []core.EventField
		Err     []string // what the error says, the input among it
	}{
		"a channel alone":             {Text: "tick", Field: core.FieldIdentifier, Channel: "tick"},
		"names, trimmed":              {Text: "swap ?item !back", Field: core.FieldIdentifier, Channel: "swap", Fields: []core.EventField{{Name: "item"}, {Send: true, Name: "back"}}},
		"values where values may be":  {Text: "swap?5!2", Field: core.FieldNameOrValue, Channel: "swap", Fields: []core.EventField{{Name: "5"}, {Send: true, Name: "2"}}},
		"a value where names must be": {Text: "swap?5.0", Field: core.FieldIdentifier, Err: []string{`"swap?5.0"`, `expected an identifier after "?"`}},
		"no channel":                  {Text: "?3", Field: core.FieldNameOrValue, Err: []string{`"?3"`, "expected a channel, an identifier"}},
		"a marker with nothing after": {Text: "swap?", Field: core.FieldNameOrValue, Err: []string{`"swap?"`, `expected a name or a value after "?"`}},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Execute
			channel, fields, err := core.SplitEvent(tc.Text, tc.Field)

			// Assert
			if tc.Err != nil {
				if err == nil {
					t.Fatalf("SplitEvent(%q) = %q %v, want an error", tc.Text, channel, fields)
				}
				for _, want := range tc.Err {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("err = %q, want it to say %q", err, want)
					}
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if channel != tc.Channel {
				t.Errorf("channel = %q, want %q", channel, tc.Channel)
			}
			if diff := cmp.Diff(tc.Fields, fields); diff != "" {
				t.Error(diff)
			}
		})
	}
}
