package replweb

import (
	"os"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v5/csdf/animation"
	"github.com/Kuniwak/puml-parallel/v5/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v5/csdf/core"
	"github.com/Kuniwak/puml-parallel/v5/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/v5/usererr"
	"github.com/google/go-cmp/cmp"
)

// trayFormal is a diagram whose clauses read as Python.
const trayFormal = `@startuml
state "Idle" as idle
idle : paper ; Nat
state "Busy" as busy
busy : paper ; Nat
[*] --> idle : $paper' == 0
idle --> busy : print?n ; $n <= $paper ; $paper' == $paper - $n
idle --> idle : refill ; ; $paper' == $paper + 1
busy --> idle : done
busy --> [*] : $paper == 0
note as channels
  channel print : Nat
end note
@enduml
`

// trayProse is trayFormal with clauses written for people.
const trayProse = `@startuml
state "Idle" as idle
idle : paper ; Nat
state "Busy" as busy
busy : paper ; Nat
[*] --> idle : the tray is empty
idle --> busy : print?n ; enough paper for n sheets ; n sheets fewer ∧ none jammed
idle --> idle : refill ; ; one sheet more
busy --> idle : done
busy --> [*] : the tray is empty
note as channels
  channel print : Nat
end note
@enduml
`

func readTestDiagram(t *testing.T, source string) *core.Diagram {
	t.Helper()
	d, _, err := animation.ReadDiagram([]byte(source), func(path string) (string, string, error) {
		t.Fatalf("no include expected: %s", path)
		return "", "", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestProseOfGivesTheClausesOfTheOtherVersionByEdge(t *testing.T) {
	// Setup
	d, prose := readTestDiagram(t, trayFormal), readTestDiagram(t, trayProse)

	// Execute
	got, err := ProseOf(d, prose)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	want := &Prose{
		InitPost: "the tray is empty",
		Edges: map[string][]Clauses{
			"idle": {{Guard: "enough paper for n sheets", Post: "n sheets fewer ∧ none jammed"}, {Guard: "(none)", Post: "one sheet more"}},
			"busy": {{Guard: "(none)", Post: "(none)"}},
		},
		Ends: map[string][]string{"busy": {"the tray is empty"}},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("ProseOf mismatch (-want +got):\n%s", diff)
	}
}

func TestProseOfRefusesAnotherDiagram(t *testing.T) {
	tests := map[string]struct {
		old, new string
		want     string
	}{
		"another edge": {
			old:  "busy --> idle : done",
			new:  "busy --> busy : done",
			want: `the 3rd edge goes busy --> idle : done here, and no edge of the prose goes so`,
		},
		"another state": {
			old:  "state \"Busy\" as busy\nbusy : paper ; Nat\n",
			new:  "state \"Busy\" as busy\nbusy : paper ; Nat\nstate \"Off\" as off\n",
			want: `the states are idle, busy here, idle, busy, off in the prose`,
		},
		"another variable": {
			old:  "busy : paper ; Nat\n",
			new:  "busy : paper ; Nat\nbusy : jams ; Nat\n",
			want: `the variables of busy are paper here, paper, jams in the prose`,
		},
		"another channel": {
			old:  "channel print : Nat",
			new:  "channel print : Bool",
			want: `the channel print carries Nat here, Bool in the prose`,
		},
		"a channel more": {
			old:  "channel print : Nat",
			new:  "channel print : Nat\n  channel jam : Nat",
			want: `the channels are print here, jam, print in the prose`,
		},
		"another start state": {
			old:  "[*] --> idle",
			new:  "[*] --> busy",
			want: `the start state is idle here, busy in the prose`,
		},
		"another end edge": {
			old:  "busy --> [*]",
			new:  "idle --> [*]",
			want: `the 1st end edge leaves busy here, idle in the prose`,
		},
		"an end edge fewer": {
			old:  "busy --> [*] : the tray is empty\n",
			new:  "",
			want: `the diagram has 1 end edges, the prose 0`,
		},
		"an edge fewer": {
			old:  "busy --> idle : done\n",
			new:  "",
			want: `the diagram has 3 edges, the prose 2`,
		},
		"an edge more": {
			old:  "busy --> idle : done\n",
			new:  "busy --> idle : done\nbusy --> busy : done\n",
			want: `the diagram has 3 edges, the prose 4`,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Setup
			if !strings.Contains(trayProse, tt.old) {
				t.Fatalf("the prose holds no %q", tt.old)
			}
			d, prose := readTestDiagram(t, trayFormal), readTestDiagram(t, strings.Replace(trayProse, tt.old, tt.new, 1))

			// Execute
			_, err := ProseOf(d, prose)

			// Assert
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want one saying %q", err, tt.want)
			}
			if _, ok := err.(usererr.UserFacing); !ok {
				t.Errorf("err = %T, want one facing the user", err)
			}
		})
	}
}

func TestProseOfRefusesAnEdgeMovingOtherLocalEdges(t *testing.T) {
	// Setup
	edge := func(local int) core.Edge {
		return core.Edge{Src: "a", Dst: "a", Event: core.Event{Channel: "tick"}, Moves: []core.Move{{Map: "items", Edge: local, Src: "x", Dst: "y"}}}
	}
	d := &core.Diagram{States: []string{"a"}, Init: "a", Edges: []core.Edge{edge(0)}}
	prose := &core.Diagram{States: []string{"a"}, Init: "a", Edges: []core.Edge{edge(1)}}

	// Execute
	_, err := ProseOf(d, prose)

	// Assert
	if want := "the 1st edge goes a --> a : tick moving items#0 here, and no edge of the prose goes so"; err == nil || !strings.Contains(err.Error(), want) {
		t.Errorf("err = %v, want one saying %q", err, want)
	}
}

func TestProseOfTakesTheExpansionOfAPromotion(t *testing.T) {
	// Setup
	read := func() *core.Diagram {
		source, err := os.ReadFile("../../examples/promote/MEMBERS.puml")
		if err != nil {
			t.Fatal(err)
		}
		d, _, err := animation.ReadDiagram(source, coreio.FileIncluder("../../examples/promote"))
		if err != nil {
			t.Fatal(err)
		}
		return d
	}
	d, prose := read(), read()

	// Execute
	got, err := ProseOf(d, prose)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if n := len(got.Edges["running"]); n != len(d.Edges) {
		t.Errorf("%d clauses of running, want %d", n, len(d.Edges))
	}
}

func TestProseOfPairsTheEdgesOfAPromotionByTheirMovesWhateverTheirOrder(t *testing.T) {
	// Setup: the canonical order of an expansion sorts the edges with the
	// same source, event and target by their clauses, so the two versions
	// may list them in another order.
	ev := core.Event{Channel: "RELINK"}
	edge := func(local int, guard string) core.Edge {
		return core.Edge{Src: "run", Dst: "run", Event: ev, Guard: clauseOf(t, guard),
			Moves: []core.Move{{Map: "items", Edge: local}}}
	}
	d := &core.Diagram{States: []string{"run"}, Init: "run", Edges: []core.Edge{edge(0, "@eq(1, 1)"), edge(1, "@eq(2, 2)")}}
	prose := &core.Diagram{States: []string{"run"}, Init: "run", Edges: []core.Edge{edge(1, "second"), edge(0, "first")}}

	// Execute
	got, err := ProseOf(d, prose)

	// Assert
	if err != nil {
		t.Fatalf("ProseOf() error = %v", err)
	}
	want := []Clauses{{Guard: "first", Post: "(none)"}, {Guard: "second", Post: "(none)"}}
	if diff := cmp.Diff(want, got.Edges["run"]); diff != "" {
		t.Errorf("Edges[run] mismatch (-want +got):\n%s", diff)
	}
}

func clauseOf(t *testing.T, text string) clause.And {
	t.Helper()
	c, err := clause.Parse(text)
	if err != nil {
		t.Fatalf("clause.Parse(%q) error = %v", text, err)
	}
	return c
}

func TestLanguageOfShouldReadTheSuffixOfTheFileName(t *testing.T) {
	tests := map[string]string{
		"examples/vending_ja.puml": "ja",
		"vending_en.puml":          "en",
		"vending.puml":             "",
		"vending_formal.puml":      "",
	}
	for path, want := range tests {
		t.Run(path, func(t *testing.T) {
			// Execute
			got := LanguageOf(path)

			// Assert
			if got != want {
				t.Errorf("LanguageOf(%q) = %q, want %q", path, got, want)
			}
		})
	}
}
