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
// as clauses whose meaning is built in, for the maps the start state holds.
// The text of the start edge is the author's alone, so a tool reading the
// expansion as a plain diagram sees what it saw before.
func TestTheStartEdgeStartsEveryKeptMapEmpty(t *testing.T) {
	author := promote.ExpandedClause{Kind: promote.ClauseLocal, Text: "buys と cycles は空", Line: 14}
	empty := func(m string) promote.ExpandedClause {
		return promote.ExpandedClause{Kind: promote.ClauseEmpty, Text: m + " = ∅", Map: m}
	}
	notHeld := strings.Replace(syncGlobal, "[*] --> running : buys と cycles は空",
		"state \"待機中\" as idle\n[*] --> idle\nidle --> running : OPEN ; true ; buys と cycles は空", 1)
	cases := map[string]struct {
		global   string
		keep     []string // nil: no -only
		want     []promote.ExpandedClause
		wantPost csdf.Predicate
	}{
		"every map":                    {global: syncGlobal, want: []promote.ExpandedClause{author, empty("buys"), empty("cycles")}, wantPost: "buys と cycles は空"},
		"with -only buys":              {global: syncGlobal, keep: []string{"buys"}, want: []promote.ExpandedClause{author, empty("buys")}, wantPost: "buys と cycles は空"},
		"with -only cycles":            {global: syncGlobal, keep: []string{"cycles"}, want: []promote.ExpandedClause{author, empty("cycles")}, wantPost: "buys と cycles は空"},
		"a start state holding no map": {global: notHeld, want: []promote.ExpandedClause{}, wantPost: csdf.PredicateTrue},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			x, diags := expand(t, c.global, syncLocals)
			if x == nil {
				t.Fatalf("promote.Expand() expansion = nil, diagnostics = %v", diags)
			}
			if c.keep != nil {
				if x, diags = promote.Restrict(x, c.keep); x == nil {
					t.Fatalf("promote.Restrict() expansion = nil, diagnostics = %v", diags)
				}
			}
			if !reflect.DeepEqual(x.Start, c.want) {
				t.Errorf("Start = %+v, want %+v", x.Start, c.want)
			}
			if got := x.Diagram.StartEdge.Post; got != c.wantPost {
				t.Errorf("StartEdge.Post = %q, want the author's text %q", got, c.wantPost)
			}
		})
	}
}

// The start edge's clauses go into the JSON whether or not the origins do,
// and as a list even when there are none.
func TestTheJSONCarriesTheStartEdgeClauses(t *testing.T) {
	cases := map[string]struct {
		comments bool
		noStart  bool
		want     int
	}{
		"with origins":         {comments: true, want: 3},
		"without origins":      {comments: false, want: 3},
		"with no start clause": {comments: true, noStart: true, want: 0},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			x, diags := expand(t, syncGlobal, syncLocals)
			if x == nil {
				t.Fatalf("promote.Expand() expansion = nil, diagnostics = %v", diags)
			}
			if c.noStart {
				x.Start = nil
			}
			var buf bytes.Buffer
			if err := promote.Write(&buf, x, promote.OutputJSON, c.comments); err != nil {
				t.Fatalf("promote.Write() error = %v", err)
			}
			var got struct {
				Start *[]promote.ExpandedClause `json:"start"`
			}
			if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if got.Start == nil || len(*got.Start) != c.want {
				t.Errorf("start = %v, want a list of %d clauses", got.Start, c.want)
			}
		})
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
