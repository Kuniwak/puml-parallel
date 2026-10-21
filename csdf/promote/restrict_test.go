package promote_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/promote"
)

func restrict(t *testing.T, global string, locals map[string]string, keep ...string) (*promote.Expansion, []promote.Diagnostic) {
	t.Helper()
	x, diags := expand(t, global, locals)
	if x == nil {
		t.Fatalf("promote.Expand() expansion = nil, diagnostics = %v", diags)
	}
	r, rdiags := promote.Restrict(x, keep)
	if r == nil {
		t.Fatalf("promote.Restrict() expansion = nil, diagnostics = %v", rdiags)
	}
	return r, rdiags
}

func TestRestrictLeavesOutTheMapsAndEdgesOfTheDroppedFamilies(t *testing.T) {
	x, _ := restrict(t, syncGlobal, syncLocals, "buys")

	for _, v := range x.Diagram.States["running"].Vars {
		if v.Name == "cycles" {
			t.Errorf("running still holds %q", v.Name)
		}
	}
	got := x.String()
	for _, unwanted := range []string{"REPORT", "tau", "cycles'"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("want no %q in the restriction, got:\n%s", unwanted, got)
		}
	}
	for _, p := range x.Promotes {
		if p.Map == "cycles" {
			t.Errorf("the restriction still promotes %q", p.Map)
		}
	}
	if _, ok := x.Locals["local/CYCLE.puml"]; ok {
		t.Error("the restriction still carries the local diagram of cycles")
	}
}

// On an edge a kept family syncs with a dropped one, the kept side stays and
// the event keeps every argument: the dropped side is the environment, which
// offers it. What the dropped side's guard read is listed, not lost.
func TestRestrictKeepsTheKeptSideOfASyncAndListsTheDroppedGuard(t *testing.T) {
	x, diags := restrict(t, syncGlobal, syncLocals, "buys")

	var books []promote.ExpandedEdge
	for _, e := range x.Edges {
		if strings.HasPrefix(string(e.Edge.Event), "BOOK?") {
			books = append(books, e)
		}
	}
	if len(books) != 2 {
		t.Fatalf("want the 2 BOOK edges, got %d:\n%s", len(books), x.String())
	}
	for _, e := range books {
		if e.Edge.Event != "BOOK?約定ID?基準日?数量" {
			t.Errorf("event = %q, want every argument kept", e.Edge.Event)
		}
		if strings.Contains(string(e.Edge.Guard), "cycles") || strings.Contains(string(e.Edge.Post), "cycles") {
			t.Errorf("edge still reads cycles: %v", e.Edge)
		}
		var dropped []promote.ExpandedClause
		for _, c := range e.Guard {
			if c.Kind == promote.ClauseDropped {
				dropped = append(dropped, c)
			}
		}
		if len(dropped) == 0 {
			t.Errorf("no dropped clause listed on %v", e.Edge)
		}
		for _, c := range dropped {
			if len(c.Maps) != 1 || c.Maps[0] != "cycles" {
				t.Errorf("dropped clause %q reads %v, want [cycles]", c.Text, c.Maps)
			}
		}
		for _, c := range e.Post {
			if c.Map == "cycles" || c.Var == "cycles" {
				t.Errorf("post clause %q about cycles is kept", c.Text)
			}
		}
	}
	if !promote.HasSeverity(diags, promote.SeverityWarning) {
		t.Errorf("want a warning for each dropped clause, got %v", diags)
	}
}

// A constrain is written in a glue's language, which no tool here reads, so it
// cannot be told to read a dropped map or not: it stays, and the author is
// told so once.
func TestRestrictSaysWhenItCannotTellWhatAConstrainReads(t *testing.T) {
	global := strings.Replace(syncGlobal, "@enduml", `note bottom of runningBuys
  constrain BUY?約定ID?数量 ; 基準日の集計が開いている
end note
@enduml`, 1)
	x, diags := restrict(t, global, syncLocals, "buys")

	for _, e := range x.Edges {
		if e.Edge.Event.Channel() == "BUY" && !strings.Contains(string(e.Edge.Guard), "基準日の集計が開いている") {
			t.Errorf("BUY lost its constrain: %v", e.Edge)
		}
	}
	n := 0
	for _, d := range diags {
		if strings.Contains(d.Message, "基準日の集計が開いている") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("want the constrain named once, got %d in %v", n, diags)
	}
}

// An opaque constrain says nothing a tool can read, so it stays and is not
// reported as one whose reads are unknown.
func TestRestrictKeepsAnOpaqueConstrainQuietly(t *testing.T) {
	global := strings.Replace(syncGlobal, "@enduml", `note bottom of runningBuys
  constrain BUY?約定ID?数量 ; @opaque
end note
@enduml`, 1)
	x, diags := restrict(t, global, syncLocals, "buys")

	kept := false
	for _, e := range x.Edges {
		kept = kept || e.Edge.Event.Channel() == "BUY" && strings.Contains(string(e.Edge.Guard), "@opaque")
	}
	if !kept {
		t.Errorf("BUY lost its opaque constrain:\n%s", x)
	}
	for _, d := range diags {
		if strings.Contains(d.Message, "cannot tell") {
			t.Errorf("want no warning about the opaque constrain, got %v", d)
		}
	}
}

// A family can be named by its type as well as by its map.
func TestRestrictNamesAFamilyByItsType(t *testing.T) {
	x, _ := restrict(t, syncGlobal, syncLocals, "Buy")
	if strings.Contains(x.String(), "REPORT") {
		t.Errorf("Cycle is not dropped:\n%s", x.String())
	}
}

func TestRestrictRefusesAnUnknownFamily(t *testing.T) {
	x, _ := expand(t, syncGlobal, syncLocals)
	r, diags := promote.Restrict(x, []string{"sells"})
	if r != nil || !promote.HasError(diags) {
		t.Errorf("promote.Restrict() = %v, %v; want a refusal", r, diags)
	}
}

// The restriction leaves the full expansion alone.
func TestRestrictDoesNotChangeItsInput(t *testing.T) {
	x, _ := expand(t, syncGlobal, syncLocals)
	before := x.String()
	_, _ = promote.Restrict(x, []string{"buys"})
	if x.String() != before {
		t.Error("promote.Restrict() changed the expansion it was given")
	}
	if !holds(x.Diagram.States["running"], "cycles") {
		t.Error("promote.Restrict() removed cycles from the input's state")
	}
}

func holds(s csdf.State, v csdf.Var) bool {
	for _, sv := range s.Vars {
		if sv.Name == v {
			return true
		}
	}
	return false
}

// A hand-written edge is opaque text, so it is kept; on a state that held a
// left-out map it may still name the map, and the author is told so.
func TestRestrictWarnsOfAHandWrittenEdgeOnAStateThatHeldALeftOutMap(t *testing.T) {
	global := strings.Replace(syncGlobal, "@enduml", "running --> running : 締め ; ; buys' = buys ∧ cycles' = cycles\n@enduml", 1)
	_, diags := restrict(t, global, syncLocals, "buys")
	found := false
	for _, d := range diags {
		found = found || (d.Severity == promote.SeverityWarning && strings.Contains(d.Message, "hand-written") && strings.Contains(d.Message, "cycles"))
	}
	if !found {
		t.Errorf("want a warning naming the hand-written edge and cycles, got %v", diags)
	}
}
