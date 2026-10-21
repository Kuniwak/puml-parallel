package core

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/lts"
)

func TestHideIsTheHidingOfTheMeaning(t *testing.T) {
	cases := []struct {
		name string
		d    *Diagram
	}{
		{"a received field is chosen inside", oneState(t, "c", []string{"got"}, "$got' = 0",
			loop(t, "c", ev("put", recv("v")), "", "$got' = $v"),
			loop(t, "c", ev("show", send("got")), "", "$got' = $got"))},
		{"the guard and the post agree on the value chosen", oneState(t, "c", []string{"got"}, "$got' = 0",
			loop(t, "c", ev("put", recv("v")), "$v = 1", "$got' = $v"),
			loop(t, "c", ev("show", send("got")), "", "$got' = $got"))},
		{"a sent field is dropped", oneState(t, "p", []string{"n"}, "$n' = 0",
			loop(t, "p", ev("put", send("n")), "", "$n' = 1 - $n"),
			loop(t, "p", ev("show", send("n")), "", "$n' = $n"))},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Setup
			env := bits()
			whole, err := Expand(c.d, env)
			if err != nil {
				t.Fatal(err)
			}
			want := lts.Hide(whole, alphabet("put"))

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
			if !lts.StableFailuresEqual(got, want) {
				t.Errorf("not stable-failures equivalent:\ngot  %#v\nwant %#v", got.Trans, want.Trans)
			}
		})
	}
}
