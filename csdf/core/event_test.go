package core_test

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/google/go-cmp/cmp"
)

func TestParseEvent(t *testing.T) {
	testCases := map[string]struct {
		Text string
		Want core.Event
		Err  bool
	}{
		"a channel":                   {Text: "tick", Want: core.Event{Channel: "tick"}},
		"a received and a sent value": {Text: "pay?5!2", Want: core.Event{Channel: "pay", Fields: []core.Field{{Name: "5"}, {Send: true, Name: "2"}}}},
		"a field named":               {Text: "insert?coin", Want: core.Event{Channel: "insert", Fields: []core.Field{{Name: "coin"}}}},
		"no channel":                  {Text: "?3", Err: true},
		"a marker with no value":      {Text: "insert?", Err: true},
		"two markers together":        {Text: "pay?!2", Err: true},
		"nothing":                     {Text: "", Err: true},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := core.ParseEvent(tc.Text)

			// Assert
			if tc.Err {
				if err == nil {
					t.Errorf("ParseEvent(%q) = %v, want an error", tc.Text, got)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.Want, got); diff != "" {
				t.Error(diff)
			}
		})
	}
}
