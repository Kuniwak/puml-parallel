package core

import (
	"testing"
)

func TestHoldsReadsTheBuiltinsOfMaps(t *testing.T) {
	a := Rec{State: "open", Fields: map[string]Value{"stock": 1}}
	cases := map[string]struct {
		clause        string
		before, after map[string]Value
		want          bool
	}{
		"a key in the map is not out of it": {clause: "@notin(k, m)", before: map[string]Value{"k": "a", "m": Map{"a": a}}, want: false},
		"a key not in the map is out of it": {clause: "@notin(k, m)", before: map[string]Value{"k": "b", "m": Map{"a": a}}, want: true},
		"a map without entries is empty":    {clause: "@empty(m')", after: map[string]Value{"m": Map{}}, want: true},
		"a map with an entry is not empty":  {clause: "@empty(m')", after: map[string]Value{"m": Map{"a": a}}, want: false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Setup
			f := mustParse(t, c.clause)

			// Execute
			got, err := Holds(f, Env{}, c.before, c.after)

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
