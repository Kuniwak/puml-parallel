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
		"the space that ends a hole before a name is the glue's too": {
			text:      "$x' in $xs",
			wantWire:  AtomWire{Text: []string{"", " in ", ""}, Holes: []int{0, 1}, Primed: []bool{true, false}},
			wantTerms: []Term{post("x"), pre("xs")},
		},
		"no space is given back after a hole of a term that is no name": {
			text:      "${m[k]}foo",
			wantWire:  AtomWire{Text: []string{"", "foo"}, Holes: []int{0}, Primed: []bool{false}},
			wantTerms: []Term{Index{Map: pre("m"), Key: pre("k")}},
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
			gotWire, gotTerms, err := Wire(a)

			// Assert
			if err != nil {
				t.Fatalf("Wire(%q): %v", tt.text, err)
			}
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

func TestWireRefuses(t *testing.T) {
	tests := map[string]struct {
		atom Atom
		want string
	}{
		"a prime on a key, not on the root": {
			atom: Atom{Parts: []Part{Hole{Term: Index{Map: pre("m"), Key: post("k")}}, Text(" > 0")}},
			want: "only the variable a term starts from may be primed",
		},
		"a prime on a field's record below an index": {
			atom: Atom{Parts: []Part{Hole{Term: Field{Of: Index{Map: post("m"), Key: post("k")}, Name: "f"}}}},
			want: "only the variable a term starts from may be primed",
		},
		"a part that is neither text nor a hole": {
			atom: Atom{Parts: []Part{nil}},
			want: "clause.Wire",
		},
		"a prime inside an argument of a function": {
			atom: Atom{Parts: []Part{Hole{Term: App{Fn: "f", Args: []Term{post("x")}}}}},
			want: "only the variable a term starts from may be primed",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			_, _, err := Wire(tt.atom)

			// Assert
			assertErrorContains(t, err, tt.want)
		})
	}
}

func TestWireOfAnAtomUnderABinder(t *testing.T) {
	// Setup
	a := Atom{Parts: []Part{Hole{Term: Bound{Index: 0}}, Text(" > 0")}}
	wantWire := AtomWire{Text: []string{"", " > 0"}, Holes: []int{0}, Primed: []bool{false}}

	// Execute
	gotWire, gotTerms, err := Wire(a)

	// Assert
	if err != nil {
		t.Fatalf("Wire: %v", err)
	}
	if diff := cmp.Diff(wantWire, gotWire); diff != "" {
		t.Errorf("Wire wire mismatch (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]Term{Bound{Index: 0}}, gotTerms); diff != "" {
		t.Errorf("Wire terms mismatch (-want +got):\n%s", diff)
	}
}
