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
		"literals, each its JSON value": {
			text: `@eq(n', 0) ∧ @ne(x, -2.5) ∧ @eq(r, {"b": [1.5, true], "a": "x;y"})`,
			want: `[{"kind":"builtin","op":"eq","args":[{"kind":"free","name":"n","primed":true},{"kind":"lit","value":0}]},
{"kind":"builtin","op":"ne","args":[{"kind":"free","name":"x"},{"kind":"lit","value":-2.5}]},
{"kind":"builtin","op":"eq","args":[{"kind":"free","name":"r"},{"kind":"lit","value":{"a":"x;y","b":[1.5,true]}}]}]`,
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
	tests := map[string]struct {
		doc  any
		want string
	}{
		"a conjunct of two kinds": {FormulaDoc{Builtin: &BuiltinDoc{Op: "empty"}, Exists: &ExistsDoc{Type: "T"}}, "more than one is set"},
		"a conjunct of no kind":   {FormulaDoc{}, "not one is set"},
		"a term of two kinds":     {TermDoc{Free: &FreeDoc{Name: "x"}, Bound: &BoundDoc{}}, "more than one is set"},
		"a term of no kind":       {TermDoc{}, "not one is set"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			_, err := json.Marshal(tt.doc)

			// Assert
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("json.Marshal error = %v, want one saying %q", err, tt.want)
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
"version": 3, "name": "D", "states": ["s0", "s1"], "labels": {}, "vars": {"s0": [], "s1": ["n", "m"]},
"init": "s0", "init_post": [], "init_line": 3,
"edges": [{"src": "s0", "dst": "s1", "channel": "c",
  "fields": [{"dir": "?", "name": "x"}, {"dir": "!", "name": "y"}], "label": "c?x!y",
  "guard": [{"kind": "atom", "pieces": ["", " > 0"], "holes": [0], "args": [{"kind": "free", "name": "x"}], "text": ["", " > 0"], "primed": [false], "function": "$_0 > 0"}],
  "post": [{"kind": "builtin", "op": "eq", "args": [{"kind": "free", "name": "n", "primed": true}, {"kind": "free", "name": "x"}]}],
  "line": 4}],
"ends": [], "assumes": [{"id": "fair", "text": "いつか確定する"}], "families": []
}`,
		},
		"a start edge with a post": {
			d: &Diagram{States: []string{"s0"}, Vars: map[string][]string{"s0": {"m"}}, Init: "s0", InitPost: mustParse(t, "@empty(m')")},
			want: `{"version": 3, "name": "", "states": ["s0"], "labels": {}, "vars": {"s0": ["m"]},
"init": "s0", "init_post": [{"kind": "builtin", "op": "empty", "args": [{"kind": "free", "name": "m", "primed": true}]}], "init_line": 0,
"edges": [], "ends": [], "assumes": [], "families": []}`,
		},
		"an end edge with a guard": {
			d: &Diagram{States: []string{"s0"}, Vars: map[string][]string{"s0": {"n"}}, Init: "s0", InitPost: clause.And{},
				Ends: []End{{Src: "s0", Guard: mustParse(t, "$n == 0")}}},
			want: `{"version": 3, "name": "", "states": ["s0"], "labels": {}, "vars": {"s0": ["n"]},
"init": "s0", "init_post": [], "init_line": 0, "edges": [],
"ends": [{"src": "s0", "guard": [{"kind": "atom", "pieces": ["", " == 0"], "holes": [0], "args": [{"kind": "free", "name": "n"}], "text": ["", " == 0"], "primed": [false], "function": "$_0 == 0"}]}],
"assumes": [], "families": []}`,
		},
		"a promoted diagram with the local states of its family": {
			d: &Diagram{States: []string{"G"}, Vars: map[string][]string{"G": {"m"}}, Init: "G", InitPost: clause.And{},
				Families: []FamilyStates{{Map: "m", Key: "k", Init: "none", States: []string{"none", "open"}, Vars: map[string][]string{"open": {"bal"}}}}},
			want: `{"version": 3, "name": "", "states": ["G"], "labels": {}, "vars": {"G": ["m"]},
"init": "G", "init_post": [], "init_line": 0, "edges": [], "ends": [], "assumes": [],
"families": [{"map": "m", "key": "k", "init": "none", "states": ["none", "open"], "vars": {"none": [], "open": ["bal"]}}]}`,
		},
		"labels only of the states labelled apart from their IDs": {
			d: &Diagram{States: []string{"s0", "s1"}, Labels: map[string]string{"s1": "完了"}, Vars: map[string][]string{}, Init: "s0", InitPost: clause.And{}},
			want: `{"version": 3, "name": "", "states": ["s0", "s1"], "labels": {"s1": "完了"}, "vars": {"s0": [], "s1": []},
"init": "s0", "init_post": [], "init_line": 0, "edges": [], "ends": [], "assumes": [], "families": []}`,
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
		"a family of a map no state holds": {
			d:    &Diagram{States: []string{"s0"}, Init: "s0", Families: []FamilyStates{{Map: "m", Key: "k", Init: "none", States: []string{"none"}}}},
			want: "the family of m: m is no variable of any state",
		},
		"a family without its key": {
			d:    &Diagram{States: []string{"s0"}, Vars: map[string][]string{"s0": {"m"}}, Init: "s0", Families: []FamilyStates{{Map: "m", Init: "none", States: []string{"none"}}}},
			want: "the family of m: no key names the IDs of its map",
		},
		"two families of one map": {
			d: &Diagram{States: []string{"s0"}, Vars: map[string][]string{"s0": {"m"}}, Init: "s0", Families: []FamilyStates{
				{Map: "m", Key: "k", Init: "none", States: []string{"none"}}, {Map: "m", Key: "k", Init: "none", States: []string{"none"}}}},
			want: "two families of m",
		},
		"a family whose start state is none of its states": {
			d:    &Diagram{States: []string{"s0"}, Vars: map[string][]string{"s0": {"m"}}, Init: "s0", Families: []FamilyStates{{Map: "m", Key: "k", Init: "none", States: []string{"open"}}}},
			want: "the family of m: its start state none is none of its states",
		},
		"a family with the variables of no state of it": {
			d: &Diagram{States: []string{"s0"}, Vars: map[string][]string{"s0": {"m"}}, Init: "s0", Families: []FamilyStates{
				{Map: "m", Key: "k", Init: "none", States: []string{"none"}, Vars: map[string][]string{"open": {"bal"}}}}},
			want: "the family of m: variables of open, which is none of its states",
		},
		"a family whose start state holds variables": {
			d: &Diagram{States: []string{"s0"}, Vars: map[string][]string{"s0": {"m"}}, Init: "s0", Families: []FamilyStates{
				{Map: "m", Key: "k", Init: "none", States: []string{"none"}, Vars: map[string][]string{"none": {"bal"}}}}},
			want: "the family of m: its start state none, which is absence, holds variables",
		},
		"an @at of an entry at no state of its family": {
			d: &Diagram{States: []string{"s0"}, Vars: map[string][]string{"s0": {"m", "k"}}, Init: "s0", InitPost: clause.And{},
				Edges:    []Edge{{Src: "s0", Dst: "s0", Event: Event{Channel: "c"}, Post: mustParse(t, "@at(m'[k], gone)")}},
				Families: []FamilyStates{{Map: "m", Key: "k", Init: "none", States: []string{"none", "open"}}}},
			want: "s0 --> s0 : c: @at(m'[…], gone), but gone is no state of the family of m",
		},
		"an @at of an entry at the start state of its family": {
			d: &Diagram{States: []string{"s0"}, Vars: map[string][]string{"s0": {"m", "k"}}, Init: "s0",
				InitPost: clause.And{Conjuncts: []clause.Formula{clause.Exists{Hint: "k", Type: "K", Body: clause.And{Conjuncts: []clause.Formula{
					clause.Builtin{Op: clause.OpAt, Args: []clause.Term{clause.Index{Map: clause.Free{Name: "m", Primed: true}, Key: clause.Bound{Index: 0}}}, State: "none"}}}}}},
				Families: []FamilyStates{{Map: "m", Key: "k", Init: "none", States: []string{"none", "open"}}}},
			want: "the start edge: none is the start state of the family of m, which is absence; say @notin",
		},
		"a guard reading a name nothing binds": {
			d: &Diagram{States: []string{"s0"}, Vars: map[string][]string{"s0": {"n"}}, Init: "s0",
				Edges: []Edge{{Src: "s0", Dst: "s0", Event: Event{Channel: "c", Fields: []Field{{Name: "x"}}}, Guard: mustParse(t, "@eq(n, x) ∧ @eq(n, y)")}}},
			want: "s0 --> s0 : c: the guard reads y, which is no state variable of s0, no field of the event and bound by no ∃",
		},
		"a post giving a value after the step to a name the target does not hold": {
			d: &Diagram{States: []string{"s0", "s1"}, Vars: map[string][]string{"s0": {"n"}}, Init: "s0",
				Edges: []Edge{{Src: "s0", Dst: "s1", Event: Event{Channel: "c"}, Post: mustParse(t, "@eq(n', 0)")}}},
			want: "s0 --> s1 : c: the post reads n', which is no state variable of s1",
		},
		"the start post reading a name before the step": {
			d:    &Diagram{States: []string{"s0"}, Vars: map[string][]string{"s0": {"n"}}, Init: "s0", InitPost: mustParse(t, "@eq(n', m)")},
			want: "the start edge: the post reads m, which is no state variable before the start, no field of the event and bound by no ∃",
		},
		"an end guard reading a name its state does not hold": {
			d:    &Diagram{States: []string{"s0"}, Init: "s0", Ends: []End{{Src: "s0", Guard: mustParse(t, "@eq(z, 0)")}}},
			want: "the end edge from s0: the guard reads z, which is no state variable of s0, no field of the event and bound by no ∃",
		},
		"an @at in a guard at no state of its family": {
			d: &Diagram{States: []string{"s0"}, Vars: map[string][]string{"s0": {"m", "k"}}, Init: "s0", InitPost: clause.And{},
				Ends:     []End{{Src: "s0", Guard: mustParse(t, "@at(m[k], gone)")}},
				Families: []FamilyStates{{Map: "m", Key: "k", Init: "none", States: []string{"none", "open"}}}},
			want: "the end edge from s0: @at(m[…], gone), but gone is no state of the family of m",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			_, err := MarshalDiagram(tt.d)

			// Assert
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("MarshalDiagram error = %v, want one mentioning %q", err, tt.want)
			}
			if n := strings.Count(err.Error(), "core."); n != 1 {
				t.Errorf("MarshalDiagram error = %v, want the package named once, not %d times", err, n)
			}
		})
	}
}

// validDoc is the document of a well-formed diagram: one state s0 holding
// n, a start post, an edge c?x guarded by an atom, and an end edge.
func validDoc(t *testing.T) DiagramDoc {
	t.Helper()
	doc, err := DiagramDocOf(&Diagram{
		States: []string{"s0"}, Vars: map[string][]string{"s0": {"n"}}, Init: "s0", InitPost: mustParse(t, "@eq(n', 0)"),
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
			edit: func(v map[string]any) { v["version"] = 1 },
			want: "version 1, but this reads version 3",
		},
		"an unknown key of a family": {
			edit: func(v map[string]any) {
				v["families"] = []any{map[string]any{"map": "m", "key": "k", "init": "none", "states": []any{"none"}, "vars": map[string]any{"none": []any{}}, "ids": "k"}}
			},
			want: `unknown field "ids"`,
		},
		"a state that is a reserved word": {
			edit: func(v map[string]any) { v["states"] = append(v["states"].([]any), "true") },
			want: "a state: true is a reserved word",
		},
		"a channel starting with a digit": {
			edit: func(v map[string]any) {
				v["edges"].([]any)[0].(map[string]any)["channel"] = "9c"
				v["edges"].([]any)[0].(map[string]any)["label"] = "9c?x"
			},
			want: "does not start with a digit, as 9c",
		},
		"a family without its key": {
			edit: func(v map[string]any) {
				v["vars"] = map[string]any{"s0": []any{"n", "m"}}
				v["families"] = []any{map[string]any{"map": "m", "init": "none", "states": []any{"none"}, "vars": map[string]any{"none": []any{}}}}
			},
			want: "the family of m: no key names the IDs of its map",
		},
		"a family of a map no state holds": {
			edit: func(v map[string]any) {
				v["families"] = []any{map[string]any{"map": "m", "key": "k", "init": "none", "states": []any{"none"}, "vars": map[string]any{"none": []any{}}}}
			},
			want: "the family of m: m is no variable of any state",
		},
		"a family without the variables of one of its states": {
			edit: func(v map[string]any) {
				v["vars"] = map[string]any{"s0": []any{"n", "m"}}
				v["families"] = []any{map[string]any{"map": "m", "key": "k", "init": "none", "states": []any{"none", "open"}, "vars": map[string]any{"none": []any{}}}}
			},
			want: "the family of m: its variables are of the states [none], not of each of its states [none open]",
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
			want: `the kind "var" is none of free, bound, index, field, app and lit`,
		},
		"a term with a key of another kind": {
			edit: func(v map[string]any) { initPost(v)["args"].([]any)[0].(map[string]any)["index"] = 0 },
			want: `a term of kind free: json: unknown field "index"`,
		},
		"a literal that is null": {
			edit: func(v map[string]any) { initPost(v)["args"].([]any)[0] = map[string]any{"kind": "lit", "value": nil} },
			want: "null is no value",
		},
		"a literal float beyond binary64": {
			edit: func(v map[string]any) {
				initPost(v)["args"].([]any)[0] = map[string]any{"kind": "lit", "value": json.Number("1e400")}
			},
			want: "beyond the floats",
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
		"a label its channel and fields do not make": {
			edit: func(v map[string]any) { v["edges"].([]any)[0].(map[string]any)["label"] = "c?y" },
			want: `labelled "c?y", but its channel and fields make "c?x"`,
		},
		"an atom's text its pieces do not make": {
			edit: func(v map[string]any) { guard(v)["text"] = []any{"", " > 1"} },
			want: "the atom $_0 > 0: its text, primed or function is not what its pieces, holes and arguments make",
		},
		"an atom's primed its arguments do not make": {
			edit: func(v map[string]any) { guard(v)["primed"] = []any{true} },
			want: "the atom $_0 > 0: its text, primed or function",
		},
		"an atom's function its pieces do not make": {
			edit: func(v map[string]any) { guard(v)["function"] = "$_0 > 1" },
			want: "the atom $_0 > 0: its text, primed or function",
		},
		"an atom of one argument twice": {
			edit: func(v map[string]any) {
				g := guard(v)
				g["pieces"], g["holes"], g["args"] = []any{"", " > ", ""}, []any{0, 1}, []any{g["args"].([]any)[0], g["args"].([]any)[0]}
				g["text"], g["primed"], g["function"] = []any{"", " > ", ""}, []any{false, false}, "$_0 > $_1"
			},
			want: "its holes and arguments are not those it reads",
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
				t.Fatalf("UnmarshalDiagram error = %v, want one mentioning %q", err, tt.want)
			}
			if n := strings.Count(err.Error(), "core."); n != 1 {
				t.Errorf("UnmarshalDiagram error = %v, want the package named once, not %d times", err, n)
			}
		})
	}
}
