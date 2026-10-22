package tracechk_test

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/csdf/tracechk"
	"github.com/google/go-cmp/cmp"
)

func read(t *testing.T, src string) *core.Diagram {
	t.Helper()
	d, err := coreio.Read(src)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestLabel(t *testing.T) {
	testCases := map[string]struct {
		Event core.Event
		Want  string
	}{
		"no field":             {core.Event{Channel: "tick"}, "tick"},
		"a received field":     {core.Event{Channel: "in", Fields: []core.Field{{Name: "k"}}}, "in?k"},
		"a sent field alone":   {core.Event{Channel: "out", Fields: []core.Field{{Send: true, Name: "n"}}}, "out!n"},
		"both, in their order": {core.Event{Channel: "pay", Fields: []core.Field{{Name: "coin"}, {Send: true, Name: "change"}}}, "pay?coin!change"},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			if got := tracechk.Label(tc.Event); got != tc.Want {
				t.Errorf("Label = %q, want %q", got, tc.Want)
			}
		})
	}
}

func TestCheck(t *testing.T) {
	type testCase struct {
		Match   tracechk.Match
		Diagram string
		Trace   []string
		// WantSteps is the accepted result, of the edges of the diagram in
		// order; nil when a rejection is wanted.
		WantSteps     func(es []core.Edge) []tracechk.Step
		WantRejection func(es []core.Edge) *tracechk.Rejection
	}

	linear := "@startuml\nstate \"a\" as a\nstate \"b\" as b\n[*] --> a\na --> b : x ; g1 ; p1\nb --> a : y ; g2 ; p2\n@enduml\n"
	forked := "@startuml\nstate \"A\" as A\nstate \"B\" as B\nstate \"C\" as C\nstate \"D\" as D\n[*] --> A\nA --> B : 1\nA --> C : 1\nC --> D : 2\n@enduml\n"
	tauThenX := "@startuml\nstate \"a\" as a\nstate \"b\" as b\nstate \"c\" as c\n[*] --> a\na --> b : tau ; gt ; pt\nb --> c : x\n@enduml\n"
	tauCycle := "@startuml\nstate \"a\" as a\nstate \"b\" as b\n[*] --> a\na --> b : tau\nb --> a : tau\na --> a : x\nb --> b : x\n@enduml\n"
	insert := "@startuml\nstate \"a\" as a\nstate \"b\" as b\n[*] --> a\na --> b : insert?coin\n@enduml\n"

	testCases := map[string]testCase{
		"a linear trace is accepted with one path per prefix": {
			Match: tracechk.MatchExact, Diagram: linear, Trace: []string{"x", "y"},
			WantSteps: func(es []core.Edge) []tracechk.Step {
				return []tracechk.Step{
					{Index: 0, Event: "x", Paths: []tracechk.PrefixPath{{Dst: "a", Next: []core.Edge{es[0]}}}},
					{Index: 1, Event: "y", Paths: []tracechk.PrefixPath{{Path: tracechk.Path{es[0]}, Dst: "b", Next: []core.Edge{es[1]}}}},
				}
			},
		},
		"the empty trace is accepted with no steps": {
			Match: tracechk.MatchExact, Diagram: linear,
			WantSteps: func([]core.Edge) []tracechk.Step { return []tracechk.Step{} },
		},
		"an event no edge carries is rejected there": {
			Match: tracechk.MatchExact, Diagram: linear, Trace: []string{"x", "w", "y"},
			WantRejection: func(es []core.Edge) *tracechk.Rejection {
				return &tracechk.Rejection{
					Index: 1, Event: "w", States: []string{"b"}, Refusing: []string{"b"}, Enabled: []string{"y"},
					Paths: []tracechk.PrefixPath{{Path: tracechk.Path{es[0]}, Dst: "b"}},
				}
			},
		},
		"a nondeterministic branch that refuses the next event rejects the trace": {
			// After 1 the diagram may be in B, which is stable and cannot do 2:
			// (<1>, {2}) is a failure, even though C can do 2.
			Match: tracechk.MatchExact, Diagram: forked, Trace: []string{"1", "2"},
			WantRejection: func(es []core.Edge) *tracechk.Rejection {
				return &tracechk.Rejection{
					Index: 1, Event: "2", States: []string{"B", "C"}, Refusing: []string{"B"},
					Paths: []tracechk.PrefixPath{{Path: tracechk.Path{es[0]}, Dst: "B"}},
				}
			},
		},
		"an unstable state does not refuse: the tau edge is part of the path": {
			Match: tracechk.MatchExact, Diagram: tauThenX, Trace: []string{"x"},
			WantSteps: func(es []core.Edge) []tracechk.Step {
				return []tracechk.Step{{Index: 0, Event: "x", Paths: []tracechk.PrefixPath{
					{Dst: "a", Taus: []core.Edge{es[0]}},
					{Path: tracechk.Path{es[0]}, Dst: "b", Next: []core.Edge{es[1]}},
				}}}
			},
		},
		"a tau cycle yields finitely many paths": {
			Match: tracechk.MatchExact, Diagram: tauCycle, Trace: []string{"x"},
			WantSteps: func(es []core.Edge) []tracechk.Step {
				return []tracechk.Step{{Index: 0, Event: "x", Paths: []tracechk.PrefixPath{
					{Dst: "a", Taus: []core.Edge{es[0]}, Next: []core.Edge{es[2]}},
					{Path: tracechk.Path{es[0]}, Dst: "b", Taus: []core.Edge{es[1]}, Next: []core.Edge{es[3]}},
				}}}
			},
		},
		"prefix match accepts a trace stripped of its fields": {
			Match: tracechk.MatchPrefix, Diagram: insert, Trace: []string{"insert"},
			WantSteps: func(es []core.Edge) []tracechk.Step {
				return []tracechk.Step{{Index: 0, Event: "insert", Paths: []tracechk.PrefixPath{{Dst: "a", Next: []core.Edge{es[0]}}}}}
			},
		},
		"exact match rejects a prefix": {
			Match: tracechk.MatchExact, Diagram: insert, Trace: []string{"insert"},
			WantRejection: func([]core.Edge) *tracechk.Rejection {
				return &tracechk.Rejection{
					Index: 0, Event: "insert", States: []string{"a"}, Refusing: []string{"a"}, Enabled: []string{"insert?coin"},
					Paths: []tracechk.PrefixPath{{Dst: "a"}},
				}
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			d := read(t, tc.Diagram)

			// Execute
			result := tracechk.Check(tc.Match, d, tracechk.Trace{Events: tc.Trace})

			// Assert
			if tc.WantSteps != nil {
				if result.Rejection != nil {
					t.Fatalf("want accepted, got %#v", result.Rejection)
				}
				if diff := cmp.Diff(tc.WantSteps(d.Edges), result.Steps); diff != "" {
					t.Error(diff)
				}
				return
			}
			if result.Steps != nil {
				t.Errorf("want no steps, got %#v", result.Steps)
			}
			if diff := cmp.Diff(tc.WantRejection(d.Edges), result.Rejection); diff != "" {
				t.Error(diff)
			}
		})
	}
}
