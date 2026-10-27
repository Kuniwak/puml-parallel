package core_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/csdf/core"
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

func TestSameShape(t *testing.T) {
	in := func(names ...string) []core.Field {
		fs := make([]core.Field, len(names))
		for i, n := range names {
			fs[i] = core.Field{Send: n[0] == '!', Name: n[1:]}
		}
		return fs
	}
	testCases := map[string]struct {
		A, B core.Event
		Want bool
	}{
		"one channel, no fields":          {core.Event{Channel: "a"}, core.Event{Channel: "a"}, true},
		"fields named apart":              {core.Event{Channel: "a", Fields: in("?k", "!n")}, core.Event{Channel: "a", Fields: in("?j", "!m")}, true},
		"another channel":                 {core.Event{Channel: "a"}, core.Event{Channel: "b"}, false},
		"another number of fields":        {core.Event{Channel: "a", Fields: in("?k")}, core.Event{Channel: "a"}, false},
		"a field of the other direction":  {core.Event{Channel: "a", Fields: in("?k")}, core.Event{Channel: "a", Fields: in("!k")}, false},
		"the directions in another order": {core.Event{Channel: "a", Fields: in("?k", "!n")}, core.Event{Channel: "a", Fields: in("!n", "?k")}, false},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if got := tc.A.SameShape(tc.B); got != tc.Want {
				t.Errorf("SameShape = %v, want %v", got, tc.Want)
			}
		})
	}
}

func TestFieldOfDirReadsWhatDirWrites(t *testing.T) {
	for _, f := range []core.Field{{Name: "x"}, {Send: true, Name: "y"}} {
		t.Run(f.Dir()+f.Name, func(t *testing.T) {
			// Execute
			got, err := core.FieldOfDir(f.Dir(), f.Name)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if got != f {
				t.Errorf("FieldOfDir = %+v, want %+v", got, f)
			}
		})
	}
}

func TestFieldOfDirRefusesAnotherDirection(t *testing.T) {
	// Execute
	_, err := core.FieldOfDir("=", "x")

	// Assert
	if err == nil || !strings.Contains(err.Error(), `direction "="`) {
		t.Errorf("FieldOfDir error = %v, want one naming the direction", err)
	}
}
