package core

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/clause"

	"github.com/google/go-cmp/cmp"
)

func TestFormulaJSONCarriesAtomsAsAGlueReceivesThem(t *testing.T) {
	// Setup
	f := mustParse(t, "${m[x].stock'} = $stock - $q ∧ @ne(from, to) ∧ @exists(m2 : locations) (@at(m2[x], open))")
	want := `[
{"atom":{"text":[""," = "," - ",""],"holes":[0,1,2],"primed":[true,false,false]},
 "function":"$_0 = $_1 - $_2",
 "args":[{"field":{"of":{"index":{"map":{"free":"m","primed":true},"key":{"free":"x"}}},"name":"stock"}},{"free":"stock"},{"free":"q"}]},
{"builtin":"ne","args":[{"free":"from"},{"free":"to"}]},
{"exists":"locations","body":[{"builtin":"at","args":[{"index":{"map":{"bound":0},"key":{"free":"x"}}}],"state":"open"}]}
]`

	// Execute
	clauses, err := FormulaJSON(f)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(clauses)
	if err != nil {
		t.Fatal(err)
	}
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(w, g); d != "" {
		t.Errorf("FormulaJSON mismatch (-want +got):\n%s", d)
	}
}

func TestFormulaJSONOfAnAtomUnderABinder(t *testing.T) {
	// Setup
	f := mustParse(t, "@exists(v : T) ($v > 0)")
	want := `[{"exists":"T","body":[{"atom":{"text":[""," > 0"],"holes":[0],"primed":[false]},"function":"$_0 > 0","args":[{"bound":0}]}]}]`

	// Execute
	clauses, err := FormulaJSON(f)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(clauses)
	if err != nil {
		t.Fatal(err)
	}
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(w, g); d != "" {
		t.Errorf("FormulaJSON mismatch (-want +got):\n%s", d)
	}
}

func TestFormulaJSONRefuses(t *testing.T) {
	tests := map[string]struct {
		f    clause.And
		want string
	}{
		"an index no binder binds": {
			f:    clause.And{Conjuncts: []clause.Formula{clause.Atom{Parts: []clause.Part{clause.Hole{Term: clause.Bound{Index: 0}}, clause.Text(" > 0")}}}},
			want: "index",
		},
		"a builtin with too few arguments": {
			f:    clause.And{Conjuncts: []clause.Formula{clause.Builtin{Op: clause.OpEq, Args: []clause.Term{clause.Free{Name: "x"}}}}},
			want: "eq",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			_, err := FormulaJSON(tt.f)

			// Assert
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("FormulaJSON error = %v, want one mentioning %q", err, tt.want)
			}
		})
	}
}

func TestDiagramJSONCarriesTheDiagramWithItsFormulas(t *testing.T) {
	// Setup
	d := &Diagram{
		Name:     "D",
		Labels:   map[string]string{"s0": "待機中"},
		States:   []string{"s0", "s1"},
		Vars:     map[string][]string{"s1": {"n", "m"}},
		Init:     "s0",
		InitPost: clause.And{},
		InitLine: 3,
		Edges: []Edge{{
			Src: "s0", Dst: "s1",
			Event: Event{Channel: "c", Fields: []Field{{Name: "x"}, {Send: true, Name: "y"}}},
			Guard: mustParse(t, "$x > 0"),
			Post:  mustParse(t, "@eq(n', x)"),
			Line:  4,
		}},
		Ends:    []End{{Src: "s1"}},
		Assumes: []Assumption{{ID: "fair", Text: "いつか確定する"}},
	}
	want := `{
"version": 1,
"name": "D",
"states": ["s0", "s1"],
"labels": {"s0": "待機中"},
"vars": {"s0": [], "s1": ["n", "m"]},
"init": "s0", "init_post": [], "init_line": 3,
"edges": [{"src": "s0", "dst": "s1", "channel": "c",
  "fields": [{"dir": "?", "name": "x"}, {"dir": "!", "name": "y"}], "label": "c?x!y",
  "guard": [{"atom": {"text": ["", " > 0"], "holes": [0], "primed": [false]}, "function": "$_0 > 0", "args": [{"free": "x"}]}],
  "post": [{"builtin": "eq", "args": [{"free": "n", "primed": true}, {"free": "x"}]}],
  "line": 4}],
"ends": [{"src": "s1", "guard": []}],
"assumes": [{"id": "fair", "text": "いつか確定する"}]
}`

	// Execute
	got, err := DiagramJSON(d)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var g, w any
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(w, g); d != "" {
		t.Errorf("DiagramJSON mismatch (-want +got):\n%s", d)
	}
}

func TestDiagramJSONRefusesADiagramThatIsNotWellFormed(t *testing.T) {
	tests := map[string]struct {
		d    *Diagram
		want string
	}{
		"an undeclared state": {
			d:    &Diagram{States: []string{"s0"}, Init: "s9"},
			want: "s9",
		},
		"a clause that is not well formed": {
			d: &Diagram{States: []string{"s0"}, Init: "s0", Edges: []Edge{{Src: "s0", Dst: "s0", Event: Event{Channel: "c"},
				Guard: clause.And{Conjuncts: []clause.Formula{clause.Builtin{Op: clause.OpEq, Args: []clause.Term{clause.Free{Name: "x"}}}}}}}},
			want: "eq",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			_, err := DiagramJSON(tt.d)

			// Assert
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("DiagramJSON error = %v, want one mentioning %q", err, tt.want)
			}
		})
	}
}
