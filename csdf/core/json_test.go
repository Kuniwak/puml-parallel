package core

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/clause"

	"github.com/google/go-cmp/cmp"
)

// sameJSON reports the difference between the JSON got and want, compared
// as values.
func sameJSON(t *testing.T, want string, got []byte) string {
	t.Helper()
	var g, w any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatal(err)
	}
	return cmp.Diff(w, g)
}

// sameClause compares clauses up to α-equivalence.
var sameClause = cmp.Comparer(func(a, b clause.And) bool { return clause.AlphaEqual(a, b) })

func TestFormulaDocOf(t *testing.T) {
	tests := map[string]struct {
		text string
		want string
	}{
		"atoms, builtins and ∃": {
			text: "${m[x].stock'} = $stock - $q ∧ @ne(from, to) ∧ @exists(m2 : locations) (@at(m2[x], open))",
			want: `[
{"kind":"atom","pieces":[""," = "," - ",""],"holes":[0,1,2],
 "args":[{"kind":"field","of":{"kind":"index","map":{"kind":"free","name":"m","primed":true},"key":{"kind":"free","name":"x"}},"name":"stock"},{"kind":"free","name":"stock"},{"kind":"free","name":"q"}],
 "text":[""," = "," - ",""],"primed":[true,false,false],"function":"$_0 = $_1 - $_2"},
{"kind":"builtin","op":"ne","args":[{"kind":"free","name":"from"},{"kind":"free","name":"to"}]},
{"kind":"exists","type":"locations","body":[{"kind":"builtin","op":"at","args":[{"kind":"index","map":{"kind":"bound","index":0},"key":{"kind":"free","name":"x"}}],"state":"open"}]}
]`,
		},
		"an atom under a binder": {
			text: "@exists(v : T) ($v > 0)",
			want: `[{"kind":"exists","type":"T","body":[{"kind":"atom","pieces":[""," > 0"],"holes":[0],"args":[{"kind":"bound","index":0}],"text":[""," > 0"],"primed":[false],"function":"$_0 > 0"}]}]`,
		},
		"text after a hole that could continue its name, spaced in the derived text only": {
			text: "$x が多い",
			want: `[{"kind":"atom","pieces":["","が多い"],"holes":[0],"args":[{"kind":"free","name":"x"}],"text":[""," が多い"],"primed":[false],"function":"$_0 が多い"}]`,
		},
		"an atom with no hole": {
			text: "a < b",
			want: `[{"kind":"atom","pieces":["a < b"],"holes":[],"args":[],"text":["a < b"],"primed":[],"function":"a < b"}]`,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			f := mustParse(t, tt.text)

			// Execute
			docs, err := FormulaDocOf(f)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			got, err := json.Marshal(docs)
			if err != nil {
				t.Fatal(err)
			}
			if d := sameJSON(t, tt.want, got); d != "" {
				t.Errorf("FormulaDocOf mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestFormulaDocOfRefuses(t *testing.T) {
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
			_, err := FormulaDocOf(tt.f)

			// Assert
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("FormulaDocOf error = %v, want one mentioning %q", err, tt.want)
			}
		})
	}
}

func TestDocsOfMoreThanOneKindAreNotWritten(t *testing.T) {
	tests := map[string]any{
		"a conjunct of two kinds": FormulaDoc{Builtin: &BuiltinDoc{Op: "empty"}, Exists: &ExistsDoc{Type: "T"}},
		"a conjunct of no kind":   FormulaDoc{},
		"a term of two kinds":     TermDoc{Free: &FreeDoc{Name: "x"}, Bound: &BoundDoc{}},
		"a term of no kind":       TermDoc{},
	}
	for name, doc := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			_, err := json.Marshal(doc)

			// Assert
			if err == nil || !strings.Contains(err.Error(), "exactly one") {
				t.Errorf("json.Marshal error = %v, want one saying exactly one kind is set", err)
			}
		})
	}
}

// jsonDiagrams are diagrams with the JSON MarshalDiagram writes of them.
func jsonDiagrams(t *testing.T) map[string]struct {
	d    *Diagram
	want string
} {
	return map[string]struct {
		d    *Diagram
		want string
	}{
		"an edge with its event, guard and post": {
			d: &Diagram{
				Name: "D", States: []string{"s0", "s1"}, Vars: map[string][]string{"s1": {"n", "m"}},
				Init: "s0", InitPost: clause.And{}, InitLine: 3,
				Edges: []Edge{{
					Src: "s0", Dst: "s1",
					Event: Event{Channel: "c", Fields: []Field{{Name: "x"}, {Send: true, Name: "y"}}},
					Guard: mustParse(t, "$x > 0"), Post: mustParse(t, "@eq(n', x)"), Line: 4,
				}},
				Assumes: []Assumption{{ID: "fair", Text: "いつか確定する"}},
			},
			want: `{
"version": 1, "name": "D", "states": ["s0", "s1"], "labels": {}, "vars": {"s0": [], "s1": ["n", "m"]},
"init": "s0", "init_post": [], "init_line": 3,
"edges": [{"src": "s0", "dst": "s1", "channel": "c",
  "fields": [{"dir": "?", "name": "x"}, {"dir": "!", "name": "y"}], "label": "c?x!y",
  "guard": [{"kind": "atom", "pieces": ["", " > 0"], "holes": [0], "args": [{"kind": "free", "name": "x"}], "text": ["", " > 0"], "primed": [false], "function": "$_0 > 0"}],
  "post": [{"kind": "builtin", "op": "eq", "args": [{"kind": "free", "name": "n", "primed": true}, {"kind": "free", "name": "x"}]}],
  "line": 4}],
"ends": [], "assumes": [{"id": "fair", "text": "いつか確定する"}]
}`,
		},
		"a start edge with a post": {
			d: &Diagram{States: []string{"s0"}, Vars: map[string][]string{"s0": {"m"}}, Init: "s0", InitPost: mustParse(t, "@empty(m')")},
			want: `{"version": 1, "name": "", "states": ["s0"], "labels": {}, "vars": {"s0": ["m"]},
"init": "s0", "init_post": [{"kind": "builtin", "op": "empty", "args": [{"kind": "free", "name": "m", "primed": true}]}], "init_line": 0,
"edges": [], "ends": [], "assumes": []}`,
		},
		"an end edge with a guard": {
			d: &Diagram{States: []string{"s0"}, Vars: map[string][]string{"s0": {"n"}}, Init: "s0", InitPost: clause.And{},
				Ends: []End{{Src: "s0", Guard: mustParse(t, "$n == 0")}}},
			want: `{"version": 1, "name": "", "states": ["s0"], "labels": {}, "vars": {"s0": ["n"]},
"init": "s0", "init_post": [], "init_line": 0, "edges": [],
"ends": [{"src": "s0", "guard": [{"kind": "atom", "pieces": ["", " == 0"], "holes": [0], "args": [{"kind": "free", "name": "n"}], "text": ["", " == 0"], "primed": [false], "function": "$_0 == 0"}]}],
"assumes": []}`,
		},
		"labels only of the states labelled apart from their IDs": {
			d: &Diagram{States: []string{"s0", "s1"}, Labels: map[string]string{"s1": "完了"}, Vars: map[string][]string{}, Init: "s0", InitPost: clause.And{}},
			want: `{"version": 1, "name": "", "states": ["s0", "s1"], "labels": {"s1": "完了"}, "vars": {"s0": [], "s1": []},
"init": "s0", "init_post": [], "init_line": 0, "edges": [], "ends": [], "assumes": []}`,
		},
	}
}

func TestMarshalDiagram(t *testing.T) {
	for name, tt := range jsonDiagrams(t) {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := MarshalDiagram(tt.d)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if d := sameJSON(t, tt.want, got); d != "" {
				t.Errorf("MarshalDiagram mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestUnmarshalDiagram(t *testing.T) {
	for name, tt := range jsonDiagrams(t) {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := UnmarshalDiagram([]byte(tt.want))

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if d := cmp.Diff(tt.d, got, sameClause); d != "" {
				t.Errorf("UnmarshalDiagram mismatch (-want +got):\n%s", d)
			}
		})
	}
}

func TestUnmarshalDiagramIgnoresWhatADocumentDerives(t *testing.T) {
	// Setup
	doc := validDoc(t)
	doc.Edges[0].Label = "something else"
	doc.Edges[0].Guard[0].Atom.Text = []string{"other"}
	doc.Edges[0].Guard[0].Atom.Primed = []bool{true}
	doc.Edges[0].Guard[0].Atom.Function = "other"
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}

	// Execute
	got, err := UnmarshalDiagram(raw)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	want := mustParse(t, "$n > 0")
	if d := cmp.Diff(want, got.Edges[0].Guard, sameClause); d != "" {
		t.Errorf("guard mismatch (-want +got):\n%s", d)
	}
	if got.Edges[0].Event.Label() != "c?x" {
		t.Errorf("label = %q, want c?x", got.Edges[0].Event.Label())
	}
}

func TestMarshalDiagramRefusesADiagramThatIsNotWellFormed(t *testing.T) {
	tests := map[string]struct {
		d    *Diagram
		want string
	}{
		"an undeclared state": {
			d:    &Diagram{States: []string{"s0"}, Init: "s9"},
			want: "no state s9",
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
			_, err := MarshalDiagram(tt.d)

			// Assert
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("MarshalDiagram error = %v, want one mentioning %q", err, tt.want)
			}
		})
	}
}

// validDoc is the document of a well-formed diagram: one state s0 holding
// n, a start post, an edge c?x guarded by an atom, and an end edge.
func validDoc(t *testing.T) DiagramDoc {
	t.Helper()
	doc, err := DiagramDocOf(&Diagram{
		States: []string{"s0"}, Vars: map[string][]string{"s0": {"n"}}, Init: "s0", InitPost: mustParse(t, "@eq(n', n)"),
		Edges: []Edge{{Src: "s0", Dst: "s0", Event: Event{Channel: "c", Fields: []Field{{Name: "x"}}}, Guard: mustParse(t, "$n > 0")}},
		Ends:  []End{{Src: "s0"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// rawOf is doc as JSON with edit applied to it as a generic value, for a
// document no DiagramDoc can hold.
func rawOf(t *testing.T, doc DiagramDoc, edit func(map[string]any)) []byte {
	t.Helper()
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	edit(v)
	raw, err = json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestUnmarshalDiagramRefuses(t *testing.T) {
	guard := func(v map[string]any) map[string]any {
		return v["edges"].([]any)[0].(map[string]any)["guard"].([]any)[0].(map[string]any)
	}
	initPost := func(v map[string]any) map[string]any {
		return v["init_post"].([]any)[0].(map[string]any)
	}
	tests := map[string]struct {
		edit func(map[string]any)
		want string
	}{
		"another version": {
			edit: func(v map[string]any) { v["version"] = 2 },
			want: "version 2, but this reads version 1",
		},
		"an unknown key": {
			edit: func(v map[string]any) { v["promotes"] = []any{} },
			want: `unknown field "promotes"`,
		},
		"a conjunct of no kind": {
			edit: func(v map[string]any) { delete(initPost(v), "kind") },
			want: "has no kind",
		},
		"a conjunct of a kind it does not know": {
			edit: func(v map[string]any) { initPost(v)["kind"] = "or" },
			want: `the kind "or" is none of atom, builtin and exists`,
		},
		"a conjunct with a key of another kind": {
			edit: func(v map[string]any) { initPost(v)["type"] = "T" },
			want: `a conjunct of kind builtin: json: unknown field "type"`,
		},
		"a term of a kind it does not know": {
			edit: func(v map[string]any) { initPost(v)["args"].([]any)[0].(map[string]any)["kind"] = "var" },
			want: `the kind "var" is none of free, bound, index, field and app`,
		},
		"a term with a key of another kind": {
			edit: func(v map[string]any) { initPost(v)["args"].([]any)[0].(map[string]any)["index"] = 0 },
			want: `a term of kind free: json: unknown field "index"`,
		},
		"a field of no direction": {
			edit: func(v map[string]any) {
				v["edges"].([]any)[0].(map[string]any)["fields"].([]any)[0].(map[string]any)["dir"] = "="
			},
			want: `the field x has the direction "="`,
		},
		"an atom whose pieces do not fit its holes": {
			edit: func(v map[string]any) { guard(v)["pieces"] = []any{"x"} },
			want: "one piece more than holes, not 1 pieces for 1 holes",
		},
		"an atom whose hole reads no argument": {
			edit: func(v map[string]any) { guard(v)["holes"] = []any{3} },
			want: "hole 0 reads argument 3, but the atom has 1",
		},
		"a diagram that is not well formed": {
			edit: func(v map[string]any) { v["init"] = "s9" },
			want: "no state s9",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			raw := rawOf(t, validDoc(t), tt.edit)

			// Execute
			_, err := UnmarshalDiagram(raw)

			// Assert
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("UnmarshalDiagram error = %v, want one mentioning %q", err, tt.want)
			}
		})
	}
}
