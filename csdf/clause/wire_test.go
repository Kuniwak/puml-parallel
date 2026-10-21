package clause

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestWireOfAnAtom(t *testing.T) {
	tests := map[string]struct {
		text      string
		wantWire  AtomWire
		wantTerms []Term
	}{
		"text between the holes, a term read twice is one argument": {
			text:      "$x' = $x + $x",
			wantWire:  AtomWire{Text: []string{"", " = ", " + ", ""}, Holes: []int{0, 1, 1}, Primed: []bool{true, false}},
			wantTerms: []Term{post("x"), pre("x")},
		},
		"a path is primed by its root": {
			text:      "${m'[k].f} > 0",
			wantWire:  AtomWire{Text: []string{"", " > 0"}, Holes: []int{0}, Primed: []bool{true}},
			wantTerms: []Term{Field{Of: Index{Map: post("m"), Key: pre("k")}, Name: "f"}},
		},
		"no holes": {
			text:      "it rains",
			wantWire:  AtomWire{Text: []string{"it rains"}, Holes: []int{}, Primed: []bool{}},
			wantTerms: nil,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			f, err := Parse(tt.text)
			if err != nil {
				t.Fatal(err)
			}
			a := f.Conjuncts[0].(Atom)

			// Execute
			gotWire, gotTerms := Wire(a)

			// Assert
			if diff := cmp.Diff(tt.wantWire, gotWire); diff != "" {
				t.Errorf("Wire(%q) wire mismatch (-want +got):\n%s", tt.text, diff)
			}
			if diff := cmp.Diff(tt.wantTerms, gotTerms); diff != "" {
				t.Errorf("Wire(%q) terms mismatch (-want +got):\n%s", tt.text, diff)
			}
		})
	}
}

func TestWireIsTheJSONOfTheProtocol(t *testing.T) {
	// Setup
	w := AtomWire{Text: []string{"", " > 0"}, Holes: []int{0}, Primed: []bool{false}}

	// Execute
	got, err := json.Marshal(w)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(got, &keys); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"text", "holes", "primed"} {
		if _, ok := keys[k]; !ok {
			t.Errorf("json %s has no %q", got, k)
		}
	}
}
