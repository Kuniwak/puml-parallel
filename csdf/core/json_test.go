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

func TestFormulaDocOf(t *testing.T) {
	tests := map[string]struct {
		text string
		want string
	}{
		"atoms as a glue receives them, builtins and ∃": {
			text: "${m[x].stock'} = $stock - $q ∧ @ne(from, to) ∧ @exists(m2 : locations) (@at(m2[x], open))",
			want: `[
{"atom":{"text":[""," = "," - ",""],"holes":[0,1,2],"primed":[true,false,false]},
 "pieces":[""," = "," - ",""],
 "function":"$_0 = $_1 - $_2",
 "args":[{"field":{"of":{"index":{"map":{"free":"m","primed":true},"key":{"free":"x"}}},"name":"stock"}},{"free":"stock"},{"free":"q"}]},
{"builtin":"ne","args":[{"free":"from"},{"free":"to"}]},
{"exists":"locations","body":[{"builtin":"at","args":[{"index":{"map":{"bound":0},"key":{"free":"x"}}}],"state":"open"}]}
]`,
		},
		"an atom under a binder": {
			text: "@exists(v : T) ($v > 0)",
			want: `[{"exists":"T","body":[{"atom":{"text":[""," > 0"],"holes":[0],"primed":[false]},"pieces":[""," > 0"],"function":"$_0 > 0","args":[{"bound":0}]}]}]`,
		},
		"text after a hole that could continue its name, spaced on the wire only": {
			text: "$x が多い",
			want: `[{"atom":{"text":[""," が多い"],"holes":[0],"primed":[false]},"pieces":["","が多い"],"function":"$_0 が多い","args":[{"free":"x"}]}]`,
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

// jsonDiagrams are diagrams with what MarshalDiagram writes of them.
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
				Init: "s0", InitLine: 3,
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
  "guard": [{"atom": {"text": ["", " > 0"], "holes": [0], "primed": [false]}, "pieces": ["", " > 0"], "function": "$_0 > 0", "args": [{"free": "x"}]}],
  "post": [{"builtin": "eq", "args": [{"free": "n", "primed": true}, {"free": "x"}]}],
  "line": 4}],
"ends": [], "assumes": [{"id": "fair", "text": "いつか確定する"}]
}`,
		},
		"a start edge with a post": {
			d: &Diagram{States: []string{"s0"}, Vars: map[string][]string{"s0": {"m"}}, Init: "s0", InitPost: mustParse(t, "@empty(m')")},
			want: `{"version": 1, "name": "", "states": ["s0"], "labels": {}, "vars": {"s0": ["m"]},
"init": "s0", "init_post": [{"builtin": "empty", "args": [{"free": "m", "primed": true}]}], "init_line": 0,
"edges": [], "ends": [], "assumes": []}`,
		},
		"an end edge with a guard": {
			d: &Diagram{States: []string{"s0"}, Vars: map[string][]string{"s0": {"n"}}, Init: "s0",
				Ends: []End{{Src: "s0", Guard: mustParse(t, "$n == 0")}}},
			want: `{"version": 1, "name": "", "states": ["s0"], "labels": {}, "vars": {"s0": ["n"]},
"init": "s0", "init_post": [], "init_line": 0, "edges": [],
"ends": [{"src": "s0", "guard": [{"atom": {"text": ["", " == 0"], "holes": [0], "primed": [false]}, "pieces": ["", " == 0"], "function": "$_0 == 0", "args": [{"free": "n"}]}]}],
"assumes": []}`,
		},
		"labels only of the states labelled apart from their IDs": {
			d: &Diagram{States: []string{"s0", "s1"}, Labels: map[string]string{"s1": "完了"}, Vars: map[string][]string{}, Init: "s0"},
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

func TestUnmarshalDiagramReadsWhatMarshalDiagramWrote(t *testing.T) {
	sameClause := cmp.Comparer(func(a, b clause.And) bool { return clause.AlphaEqual(a, b) })
	for name, tt := range jsonDiagrams(t) {
		t.Run(name, func(t *testing.T) {
			// Setup
			raw, err := MarshalDiagram(tt.d)
			if err != nil {
				t.Fatal(err)
			}

			// Execute
			got, err := UnmarshalDiagram(raw)

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
			_, err := MarshalDiagram(tt.d)

			// Assert
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("MarshalDiagram error = %v, want one mentioning %q", err, tt.want)
			}
		})
	}
}

func TestUnmarshalDiagramRefuses(t *testing.T) {
	const head = `"name": "", "states": ["s0"], "labels": {}, "vars": {"s0": ["n"]}, "init": "s0", "init_line": 0, "ends": [], "assumes": []`
	edge := func(fields, label, guard string) string {
		return `{"version": 1, ` + head + `, "init_post": [], "edges": [{"src": "s0", "dst": "s0", "channel": "c", "fields": ` + fields +
			`, "label": "` + label + `", "guard": ` + guard + `, "post": [], "line": 0}]}`
	}
	tests := map[string]struct {
		raw  string
		want string
	}{
		"another version": {
			raw:  `{"version": 2, ` + head + `, "init_post": [], "edges": []}`,
			want: "version 2",
		},
		"an unknown key": {
			raw:  `{"version": 1, ` + head + `, "init_post": [], "edges": [], "promotes": []}`,
			want: "promotes",
		},
		"a conjunct of two kinds": {
			raw:  `{"version": 1, ` + head + `, "init_post": [{"builtin": "empty", "exists": "T", "args": [{"free": "n", "primed": true}]}], "edges": []}`,
			want: "exactly one",
		},
		"a term of two kinds": {
			raw:  `{"version": 1, ` + head + `, "init_post": [{"builtin": "empty", "args": [{"free": "n", "bound": 0}]}], "edges": []}`,
			want: "exactly one",
		},
		"a field of no direction": {
			raw:  edge(`[{"dir": "=", "name": "x"}]`, "c=x", "[]"),
			want: "direction",
		},
		"a label the event does not make": {
			raw:  edge(`[{"dir": "?", "name": "x"}]`, "c?y", "[]"),
			want: "labelled",
		},
		"an atom whose wire its pieces do not make": {
			raw: edge(`[]`, "c",
				`[{"atom": {"text": ["", " > 1"], "holes": [0], "primed": [false]}, "pieces": ["", " > 0"], "function": "$_0 > 0", "args": [{"free": "n"}]}]`),
			want: "wire",
		},
		"an atom whose function its pieces do not make": {
			raw: edge(`[]`, "c",
				`[{"atom": {"text": ["", " > 0"], "holes": [0], "primed": [false]}, "pieces": ["", " > 0"], "function": "$_0 > 1", "args": [{"free": "n"}]}]`),
			want: "function",
		},
		"a diagram that is not well formed": {
			raw:  `{"version": 1, "name": "", "states": ["s0"], "labels": {}, "vars": {}, "init": "s9", "init_post": [], "init_line": 0, "edges": [], "ends": [], "assumes": []}`,
			want: "s9",
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Execute
			_, err := UnmarshalDiagram([]byte(tt.raw))

			// Assert
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("UnmarshalDiagram error = %v, want one mentioning %q", err, tt.want)
			}
		})
	}
}
