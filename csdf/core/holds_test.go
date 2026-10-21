package core

import (
	"testing"
)

func TestHoldsReadsPartialMaps(t *testing.T) {
	a := Rec{State: "open", Fields: map[string]Value{"stock": 1}}
	cases := map[string]struct {
		clause        string
		before, after map[string]Value
		want          bool
	}{
		"a key in the map is not out of it":          {clause: "@notin(k, m)", before: map[string]Value{"k": "a", "m": Map{"a": a}}, want: false},
		"a key not in the map is out of it":          {clause: "@notin(k, m)", before: map[string]Value{"k": "b", "m": Map{"a": a}}, want: true},
		"a map without entries is empty":             {clause: "@empty(m')", after: map[string]Value{"m": Map{}}, want: true},
		"a map with an entry is not empty":           {clause: "@empty(m')", after: map[string]Value{"m": Map{"a": a}}, want: false},
		"an instance that exists is at its state":    {clause: "@at(m[k], open)", before: map[string]Value{"k": "a", "m": Map{"a": a}}, want: true},
		"a builtin reading a missing entry is false": {clause: "@at(m[k], open)", before: map[string]Value{"k": "b", "m": Map{"a": a}}, want: false},
		"an atom reading a missing entry is false":   {clause: "${m[k].stock} = 1", before: map[string]Value{"k": "b", "m": Map{"a": a}}, want: false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Setup
			f := mustParse(t, c.clause)

			// Execute
			got, err := Holds(f, Env{Atoms: map[string]func([]Value) bool{"#0 = 1": func(a []Value) bool { return a[0] == 1 }}}, c.before, c.after)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("Holds(%q) = %v, want %v", c.clause, got, c.want)
			}
		})
	}
}
