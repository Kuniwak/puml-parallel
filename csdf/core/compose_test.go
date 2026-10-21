package core

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/lts"
)

// channel is the channel of a label of Expand.
func channel(label string) string {
	c, _, _ := strings.Cut(label, ".")
	return c
}

func alphabet(channels ...string) func(string) bool {
	return func(label string) bool {
		for _, c := range channels {
			if channel(label) == c {
				return true
			}
		}
		return false
	}
}

func oneState(t *testing.T, state string, vars []string, init string, edges ...Edge) *Diagram {
	return &Diagram{Vars: map[string][]string{state: vars}, Init: state, InitPost: mustParse(t, init), Edges: edges}
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
	echo := oneState(t, "e", []string{"m"}, "$m' = 1",
		loop(t, "e", ev("put", send("m")), "", "$m' = $m"))
	cases := []struct {
		name   string
		l, r   *Diagram
		la, ra []string
	}{
		{"a send meets a receive", producer, consumer, []string{"put", "idle"}, []string{"put"}},
		{"a receive meets a send", consumer, producer, []string{"put"}, []string{"put", "idle"}},
		{"the receiver's guard reads the sent value", producer, pickyConsumer, []string{"put", "idle"}, []string{"put"}},
		{"two receives agree on one value", consumer, listener, []string{"put"}, []string{"put"}},
		{"two sends agree on their values", producer, echo, []string{"put", "idle"}, []string{"put"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Setup
			env := bits()
			l, err := Expand(c.l, env)
			if err != nil {
				t.Fatal(err)
			}
			r, err := Expand(c.r, env)
			if err != nil {
				t.Fatal(err)
			}
			want := lts.Parallel([]*lts.LTS{l, r}, []func(string) bool{alphabet(c.la...), alphabet(c.ra...)})

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
			if !lts.StableFailuresEqual(got, want) {
				t.Errorf("not stable-failures equivalent:\ngot  %#v\nwant %#v", got.Trans, want.Trans)
			}
		})
	}
}

func TestComposeRefusesACapture(t *testing.T) {
	// Setup: both receive; renaming the right side's w to the left side's
	// v would make it read its own state variable v.
	left := oneState(t, "l", nil, "",
		loop(t, "l", ev("put", recv("v")), "", ""))
	right := oneState(t, "r", []string{"v"}, "$v' = 0",
		loop(t, "r", ev("put", recv("w")), "", "$v' = $w"))

	// Execute
	_, err := Compose(left, right, []string{"put"})

	// Assert
	if err == nil || !strings.Contains(err.Error(), "capture") {
		t.Errorf("err = %v, want a capture refused", err)
	}
}
