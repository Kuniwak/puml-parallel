package promote_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/promote"
)

// Every map starts empty, which the <<promote>> blocks already say: the
// expansion carries it on the start edge, after what the author wrote there,
// as clauses whose meaning is built in. The text of the start edge is the
// author's alone, so a tool reading the expansion as a plain diagram sees
// what it saw before.
func TestTheStartEdgeStartsEveryKeptMapEmpty(t *testing.T) {
	cases := map[string]struct {
		keep []string // nil: no -only
		want []string // the maps that start empty
	}{
		"every map":         {keep: nil, want: []string{"buys", "cycles"}},
		"with -only buys":   {keep: []string{"buys"}, want: []string{"buys"}},
		"with -only cycles": {keep: []string{"cycles"}, want: []string{"cycles"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			x, diags := expand(t, syncGlobal, syncLocals)
			if x == nil {
				t.Fatalf("promote.Expand() expansion = nil, diagnostics = %v", diags)
			}
			if c.keep != nil {
				if x, diags = promote.Restrict(x, c.keep); x == nil {
					t.Fatalf("promote.Restrict() expansion = nil, diagnostics = %v", diags)
				}
			}
			want := []promote.ExpandedClause{{Kind: promote.ClauseLocal, Text: "buys と cycles は空", Line: 14}}
			for _, m := range c.want {
				want = append(want, promote.ExpandedClause{Kind: promote.ClauseEmpty, Text: m + " = ∅", Map: m})
			}
			if !reflect.DeepEqual(x.Start, want) {
				t.Errorf("Start = %+v, want %+v", x.Start, want)
			}
			if got, want := x.Diagram.StartEdge.Post, csdf.Predicate("buys と cycles は空"); got != want {
				t.Errorf("StartEdge.Post = %q, want the author's text %q", got, want)
			}
		})
	}
}

// A map held by a state the start edge does not lead to is not said to start
// empty there: the start state has no such variable.
func TestAMapTheStartStateDoesNotHoldIsNotStartedEmpty(t *testing.T) {
	global := strings.Replace(syncGlobal, "[*] --> running : buys と cycles は空", "state \"待機中\" as idle\n[*] --> idle\nidle --> running : OPEN ; true ; buys と cycles は空", 1)
	x, diags := expand(t, global, syncLocals)
	if x == nil {
		t.Fatalf("promote.Expand() expansion = nil, diagnostics = %v", diags)
	}
	if len(x.Start) != 0 {
		t.Errorf("Start = %+v, want none", x.Start)
	}
}

// The start edge's clauses go into the JSON whether or not the origins do,
// and as an array even when there are none.
func TestTheJSONCarriesTheStartEdgeClauses(t *testing.T) {
	x, _ := expand(t, syncGlobal, syncLocals)
	for _, comments := range []bool{true, false} {
		var buf bytes.Buffer
		if err := promote.Write(&buf, x, promote.OutputJSON, comments); err != nil {
			t.Fatalf("promote.Write() error = %v", err)
		}
		var got struct {
			Start []promote.ExpandedClause `json:"start"`
		}
		if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if len(got.Start) != 3 {
			t.Errorf("comments=%v: start = %+v, want the author's clause and two empty ones", comments, got.Start)
		}
	}
	x.Start = nil
	var buf bytes.Buffer
	_ = promote.Write(&buf, x, promote.OutputJSON, true)
	if !strings.Contains(buf.String(), `"start":[]`) {
		t.Errorf("want start written as an empty array, got %s", buf.String())
	}
}

// -only keeps the author's start clauses as written, as it keeps a
// hand-written edge, and says they may still read a map it left out.
func TestRestrictWarnsThatTheAuthorStartClausesMayReadALeftOutMap(t *testing.T) {
	_, diags := restrict(t, syncGlobal, syncLocals, "buys")
	want := "the start edge's clauses are kept as written, and may still read cycles, which -only leaves out"
	if countDiagnostic(diags, "warning: line 14: "+want) == 0 {
		t.Errorf("want the warning %q, got %v", want, diags)
	}
}
