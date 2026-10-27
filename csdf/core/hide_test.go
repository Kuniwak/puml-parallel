package core

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/csdf/lts"
)

func TestHideIsTheHidingOfTheMeaning(t *testing.T) {
	cases := []struct {
		name string
		d    *Diagram
		want *lts.LTS
	}{
		{"a received field is chosen inside", oneState(t, "c", []string{"got"}, "$got' = 0",
			loop(t, "c", ev("put", recv("v")), "", "$got' = $v"),
			loop(t, "c", ev("show", send("got")), "", "$got' = $got")),
			// got is 0 at 0 and 1 at 1; τ sets it to either.
			handLTS(tr{0, "show.0", 0}, tr{0, lts.Tau, 0}, tr{0, lts.Tau, 1}, tr{1, "show.1", 1}, tr{1, lts.Tau, 0}, tr{1, lts.Tau, 1})},
		{"the guard and the post agree on the value chosen", oneState(t, "c", []string{"got"}, "$got' = 0",
			loop(t, "c", ev("put", recv("v")), "$v = 1", "$got' = $v"),
			loop(t, "c", ev("show", send("got")), "", "$got' = $got")),
			handLTS(tr{0, "show.0", 0}, tr{0, lts.Tau, 1}, tr{1, "show.1", 1}, tr{1, lts.Tau, 1})},
		{"a sent field is dropped", oneState(t, "p", []string{"n"}, "$n' = 0",
			loop(t, "p", ev("put", send("n")), "", "$n' = 1 - $n"),
			loop(t, "p", ev("show", send("n")), "", "$n' = $n")),
			handLTS(tr{0, "show.0", 0}, tr{0, lts.Tau, 1}, tr{1, "show.1", 1}, tr{1, lts.Tau, 0})},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Setup
			env := bits()

			// Execute
			hidden, err := Hide(c.d, []string{"put"})
			if err != nil {
				t.Fatal(err)
			}
			got, err := Expand(hidden, env)

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
