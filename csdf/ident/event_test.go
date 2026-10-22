package ident_test

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/ident"
	"github.com/google/go-cmp/cmp"
)

func TestSplitEvent(t *testing.T) {
	nonEmpty := func(s string) bool { return s != "" }
	testCases := map[string]struct {
		Text    string
		Field   func(string) bool
		Channel string
		Fields  []ident.EventField
		Err     bool
	}{
		"a channel alone":             {Text: "tick", Field: ident.Is, Channel: "tick"},
		"names, trimmed":              {Text: "pay ?coin !change", Field: ident.Is, Channel: "pay", Fields: []ident.EventField{{Name: "coin"}, {Send: true, Name: "change"}}},
		"values where values may be":  {Text: "pay?5!2", Field: nonEmpty, Channel: "pay", Fields: []ident.EventField{{Name: "5"}, {Send: true, Name: "2"}}},
		"a value where names must be": {Text: "pay?5.0", Field: ident.Is, Err: true},
		"no channel":                  {Text: "?3", Field: nonEmpty, Err: true},
		"a marker with nothing after": {Text: "pay?", Field: nonEmpty, Err: true},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Execute
			channel, fields, err := ident.SplitEvent(tc.Text, tc.Field)

			// Assert
			if tc.Err {
				if err == nil {
					t.Errorf("SplitEvent(%q) = %q %v, want an error", tc.Text, channel, fields)
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
