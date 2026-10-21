package core

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/lts"
)

func oneState(t *testing.T, state string, vars []string, init string, edges ...Edge) *Diagram {
	return &Diagram{States: []string{state}, Vars: map[string][]string{state: vars}, Init: state, InitPost: mustParse(t, init), Edges: edges}
}

func loop(t *testing.T, state string, e Event, guard, post string) Edge {
	return Edge{Src: state, Dst: state, Event: e, Guard: mustParse(t, guard), Post: mustParse(t, post)}
}

func ev(channel string, fields ...Field) Event { return Event{Channel: channel, Fields: fields} }
func recv(name string) Field                   { return Field{Name: name} }
func send(name string) Field                   { return Field{Send: true, Name: name} }

func bits() Env {
	return Env{
		Types: map[string][]Value{"n": {0, 1}, "got": {0, 1}, "v": {0, 1}, "w": {0, 1}, "m": {0, 1}},
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
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Setup
			env := bits()

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
		"a state variable both sides hold": {
			l:    oneState(t, "l", []string{"v"}, "$v' = 0"),
			r:    oneState(t, "r", []string{"v"}, "$v' = 0"),
			want: "hold v",
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
