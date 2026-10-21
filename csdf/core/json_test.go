package core

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFormulaJSONCarriesAtomsAsAGlueReceivesThem(t *testing.T) {
	// Setup
	f := mustParse(t, "${m[x].stock'} = $stock - $q ∧ @ne(from, to) ∧ @exists(m2 : locations) (@at(m2[x], open))")
	want := `[
{"atom":{"text":[""," = "," - ",""],"holes":[0,1,2],"primed":[true,false,false]},
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
