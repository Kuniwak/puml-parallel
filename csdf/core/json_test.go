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
