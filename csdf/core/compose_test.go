package core

import (
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/Kuniwak/puml-parallel/csdf/lts"
	"github.com/Kuniwak/puml-parallel/csdf/types"
)

// oneState is a diagram of one state holding vars, each of the type Bit,
// as is every field of its edges.
func oneState(t *testing.T, state string, vars []string, init string, edges ...Edge) *Diagram {
	d := &Diagram{States: []string{state}, Vars: map[string][]string{state: vars}, Init: state, InitPost: mustParse(t, init), Edges: edges}
	for _, v := range vars {
		if d.Types == nil {
			d.Types = map[string]types.Type{}
		}
		d.Types[v] = types.Named("Bit")
	}
	for _, e := range edges {
		if len(e.Event.Fields) == 0 {
			continue
		}
		if d.Channels == nil {
			d.Channels = map[string][]types.Type{}
		}
		d.Channels[e.Event.Channel] = slices.Repeat([]types.Type{types.Named("Bit")}, len(e.Event.Fields))
	}
	return d
}

func loop(t *testing.T, state string, e Event, guard, post string) Edge {
	return Edge{Src: state, Dst: state, Event: e, Guard: mustParse(t, guard), Post: mustParse(t, post)}
}

func ev(channel string, fields ...Field) Event { return Event{Channel: channel, Fields: fields} }
func recv(name string) Field                   { return Field{Name: name} }
func send(name string) Field                   { return Field{Send: true, Name: name} }

func bits() Env {
	return Env{
		Types: map[string][]Value{"Bit": {0, 1}},
		Atoms: map[string]func([]Value) bool{
			"#0 = 0":      func(a []Value) bool { return a[0] == 0 },
			"#0 = #1":     func(a []Value) bool { return a[0] == a[1] },
			"#0 = 1 - #1": func(a []Value) bool { return a[0] == 1-a[1].(int) },
			"#0 = 1":      func(a []Value) bool { return a[0] == 1 },
		},
	}
}

// lts written by hand: each transition is src, label, dst.
type tr struct {
	src   int
	label string
	dst   int
}

func handLTS(ts ...tr) *lts.LTS {
	l := lts.New(0)
	for _, t := range ts {
		l.Add(t.src, t.label, t.dst)
	}
	return l
}

func TestComposeIsTheParallelOfTheMeanings(t *testing.T) {
	producer := oneState(t, "p", []string{"n"}, "$n' = 0",
		loop(t, "p", ev("put", send("n")), "", "$n' = 1 - $n"),
		loop(t, "p", ev("idle"), "", "$n' = $n"))
	consumer := oneState(t, "c", []string{"got"}, "$got' = 0",
		loop(t, "c", ev("put", recv("v")), "", "$got' = $v"))
	pickyConsumer := oneState(t, "c", []string{"got"}, "$got' = 0",
		loop(t, "c", ev("put", recv("v")), "$v = 1", "$got' = $v"))
	listener := oneState(t, "l", []string{"m"}, "$m' = 0",
		loop(t, "l", ev("put", recv("w")), "", "$m' = $w"))
	echo := oneState(t, "e", []string{"m"}, "$m' = 0",
		loop(t, "e", ev("put", send("m")), "", "$m' = $m"))
	// Two switches each turning their own n on once: n is two variables.
	switchA := oneState(t, "a", []string{"n"}, "$n' = 0", loop(t, "a", ev("onA"), "$n = 0", "$n' = 1"))
	switchB := oneState(t, "b", []string{"n"}, "$n' = 0", loop(t, "b", ev("onB"), "$n = 0", "$n' = 1"))
	// The producer alternates put.0 and put.1, idling at will.
	alternating := handLTS(tr{0, "put.0", 1}, tr{0, "idle", 0}, tr{1, "put.1", 0}, tr{1, "idle", 1})
	cases := []struct {
		name string
		l, r *Diagram
		want *lts.LTS
	}{
		{"a send meets a receive", producer, consumer, alternating},
		{"a receive meets a send", consumer, producer, alternating},
		{"the receiver's guard reads the sent value", producer, pickyConsumer, handLTS(tr{0, "idle", 0})},
		{"two receives agree on one value", consumer, listener, handLTS(tr{0, "put.0", 0}, tr{0, "put.1", 0})},
		{"two sends agree on their values", producer, echo, handLTS(tr{0, "put.0", 1}, tr{0, "idle", 0}, tr{1, "idle", 1})},
		{"a state variable both sides hold is two variables", switchA, switchB, handLTS(tr{0, "onA", 1}, tr{0, "onB", 2}, tr{1, "onB", 3}, tr{2, "onA", 3})},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Setup: the right side's n is renamed n2, of n's type.
			env := bits()
			env.Types["n2"] = env.Types["n"]

			// Execute
			composite, err := Compose(c.l, c.r, []string{"put"})
			if err != nil {
				t.Fatal(err)
			}
			got, err := Expand(composite, env)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if !lts.StableFailuresEqual(got, c.want) {
				t.Errorf("not stable-failures equivalent:\ngot  %#v\nwant %#v", got.Trans, c.want.Trans)
			}
		})
	}
}

func TestComposeRefuses(t *testing.T) {
	cases := map[string]struct {
		l, r *Diagram
		want string
	}{
		// Both receive; renaming the right side's w to the left side's v
		// would make it read its own state variable v.
		"a capture": {
			l:    oneState(t, "l", nil, "", loop(t, "l", ev("put", recv("v")), "", "")),
			r:    oneState(t, "r", []string{"v"}, "$v' = 0", loop(t, "r", ev("put", recv("w")), "", "$v' = $w")),
			want: "capture",
		},
		"two pairs of states with one ID": {
			l:    &Diagram{States: []string{"a_b", "a"}, Init: "a"},
			r:    &Diagram{States: []string{"c", "b_c"}, Init: "c"},
			want: "a_b_c",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Execute
			_, err := Compose(c.l, c.r, []string{"put"})

			// Assert
			assertErrorContains(t, err, c.want)
		})
	}
}

func TestComposeKeepsAStateWithoutVariables(t *testing.T) {
	// Setup: l steps from a to b on go while r, whose one state holds
	// nothing, stays.
	l := &Diagram{States: []string{"a", "b"}, Init: "a", Edges: []Edge{{Src: "a", Dst: "b", Event: ev("go")}}}
	r := &Diagram{States: []string{"r"}, Init: "r"}
	want := handLTS(tr{0, "go", 1})

	// Execute
	composite, err := Compose(l, r, nil)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Expand(composite, bits())

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if !lts.StableFailuresEqual(got, want) {
		t.Errorf("not stable-failures equivalent: %#v", got.Trans)
	}
}

func TestComposeNamesItsStatesAsCsdfparallelDid(t *testing.T) {
	// Setup: the right side labels its state; the left side does not.
	l := &Diagram{States: []string{"a", "b"}, Init: "a", Edges: []Edge{{Src: "a", Dst: "b", Event: ev("go")}}}
	r := &Diagram{States: []string{"r"}, Labels: map[string]string{"r": "待機"}, Init: "r"}
	wantStates := []string{"a_r", "b_r"}
	wantLabels := map[string]string{"a_r": "(a, 待機)", "b_r": "(b, 待機)"}

	// Execute
	got, err := Compose(l, r, nil)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(wantStates, got.States); diff != "" {
		t.Errorf("States (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantLabels, got.Labels); diff != "" {
		t.Errorf("Labels (-want +got):\n%s", diff)
	}
	if got.Init != "a_r" || got.Edges[0].Src != "a_r" || got.Edges[0].Dst != "b_r" {
		t.Errorf("Init %s, edge %s --> %s", got.Init, got.Edges[0].Src, got.Edges[0].Dst)
	}
}

func TestComposeCarriesTheAssumeNotesOfBoth(t *testing.T) {
	// Setup
	l := &Diagram{States: []string{"a"}, Init: "a", Assumes: []Assumption{{ID: "p", Text: "P"}}}
	r := &Diagram{States: []string{"b"}, Init: "b", Assumes: []Assumption{{ID: "q", Text: "Q"}}}
	want := []Assumption{{ID: "p", Text: "P"}, {ID: "q", Text: "Q"}}

	// Execute
	got, err := Compose(l, r, nil)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, got.Assumes); diff != "" {
		t.Errorf("Assumes (-want +got):\n%s", diff)
	}
}

func TestComposeRefusesByCaptureFieldsThatWouldMeetUnderOneName(t *testing.T) {
	// Setup: synchronising c?x!y with c!y?x would name both fields of the
	// composite y; renaming x to y is refused as capturing y.
	l := &Diagram{States: []string{"s0"}, Vars: map[string][]string{"s0": {"y"}}, Init: "s0", Edges: []Edge{{Src: "s0", Dst: "s0", Event: Event{Channel: "c", Fields: []Field{{Name: "x"}, {Send: true, Name: "y"}}}}}}
	r := &Diagram{States: []string{"t0"}, Vars: map[string][]string{"t0": {}}, Init: "t0", Edges: []Edge{{Src: "t0", Dst: "t0", Event: Event{Channel: "c", Fields: []Field{{Send: true, Name: "y"}, {Name: "x"}}}}}}

	// Execute
	_, err := Compose(l, r, []string{"c"})

	// Assert
	if err == nil || !strings.Contains(err.Error(), "renaming x to y would capture y") {
		t.Fatalf("Compose error = %v, want the renaming of x to y refused as capturing y", err)
	}
}

func TestTypesDeclaredRefusesAnEventThatRepeatsAFieldName(t *testing.T) {
	// Setup: a diagram no well-formedness check has read, as a promotion builds.
	d := &Diagram{States: []string{"s0"}, Vars: map[string][]string{"s0": {}}, Init: "s0",
		Channels: map[string][]types.Type{"c": {types.Named("Bit"), types.Named("Bit")}},
		Edges:    []Edge{{Src: "s0", Dst: "s0", Event: Event{Channel: "c", Fields: []Field{{Name: "x"}, {Send: true, Name: "x"}}}}}}

	// Execute
	err := TypesDeclared(d)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "c?x!x repeats the field x") {
		t.Fatalf("TypesDeclared error = %v, want c?x!x refused for repeating x", err)
	}
}
