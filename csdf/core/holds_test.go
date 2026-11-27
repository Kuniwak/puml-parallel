package core

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestHoldsReadsPartialMaps(t *testing.T) {
	a := Rec{State: "open", Fields: map[string]Value{"stock": 1}}
	cases := map[string]struct {
		clause        string
		before, after map[string]Value
		want          bool
	}{
		"a key in the map is not out of it":          {clause: "@notin(k, m)", before: map[string]Value{"k": "a", "m": Map{Key("a"): a}}, want: false},
		"a key not in the map is out of it":          {clause: "@notin(k, m)", before: map[string]Value{"k": "b", "m": Map{Key("a"): a}}, want: true},
		"a map without entries is empty":             {clause: "@empty(m')", after: map[string]Value{"m": Map{}}, want: true},
		"a map with an entry is not empty":           {clause: "@empty(m')", after: map[string]Value{"m": Map{Key("a"): a}}, want: false},
		"an instance that exists is at its state":    {clause: "@at(m[k], open)", before: map[string]Value{"k": "a", "m": Map{Key("a"): a}}, want: true},
		"an instance is at one of several states":    {clause: "@at(m[k], shut, open)", before: map[string]Value{"k": "a", "m": Map{Key("a"): a}}, want: true},
		"an instance is at none of several states":   {clause: "@at(m[k], shut, gone)", before: map[string]Value{"k": "a", "m": Map{Key("a"): a}}, want: false},
		"a builtin reading a missing entry is false": {clause: "@at(m[k], open)", before: map[string]Value{"k": "b", "m": Map{Key("a"): a}}, want: false},
		"an atom reading a missing entry is false":   {clause: "${m[k].stock} = 1", before: map[string]Value{"k": "b", "m": Map{Key("a"): a}}, want: false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Setup
			f := mustParse(t, c.clause)

			// Execute
			got, err := Holds(f, Env{Atoms: map[string]func([]Value) bool{"#0 = 1": func(a []Value) bool { return a[0] == 1 }}}, nil, c.before, c.after)

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

func TestHoldsComparesLiteralsAsValuesOfTheirKind(t *testing.T) {
	cases := map[string]struct {
		clause string
		x      Value
		want   bool
	}{
		"an integer is the integer":            {clause: "@eq(x, 0)", x: 0, want: true},
		"an integer is not the float":          {clause: "@eq(x, 1.0)", x: 1, want: false},
		"a float is the float":                 {clause: "@eq(x, 1.0)", x: 1.0, want: true},
		"a float is compared by its value":     {clause: "@eq(x, 0.0)", x: 0.0, want: true},
		"a string is not the number it spells": {clause: `@ne(x, "1")`, x: 1, want: true},
		"a truth value is not its name":        {clause: `@eq(x, "true")`, x: true, want: false},
		"a truth value is itself":              {clause: "@eq(x, true)", x: true, want: true},
		"a JSON number is the integer":         {clause: "@eq(x, 7)", x: json.Number("7"), want: true},
		"an array is its items in order":       {clause: `@eq(x, [1, "a"])`, x: []any{1, "a"}, want: true},
		"an object is a map of its keys":       {clause: `@eq(x, {"b": 2, "a": 1})`, x: Map{Key("a"): 1, Key("b"): 2}, want: true},
		"a literal key is in the map":          {clause: `@in("a", x)`, x: Map{Key("a"): 1}, want: true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Setup
			f := mustParse(t, c.clause)

			// Execute
			got, err := Holds(f, Env{}, nil, map[string]Value{"x": c.x}, nil)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("Holds(%q) of %#v = %v, want %v", c.clause, c.x, got, c.want)
			}
		})
	}
}

func TestHoldsReadsAFieldAnInstanceDoesNotObserveAtItsStateAsMissing(t *testing.T) {
	// m's instances observe x and y; y is held at b alone, and h, held at
	// every state, is observed at none.
	families := []FamilyStates{{Map: "m", Key: "k", Init: "none", States: []string{"none", "a", "b"},
		Vars:     map[string][]string{"none": nil, "a": {"x", "h"}, "b": {"x", "y", "h"}},
		Observes: &Observation{Observables: []Observable{{Name: "x"}, {Name: "y"}}}}}
	atA := Map{Key(1): Rec{State: "a", Fields: map[string]Value{"x": 0, "h": 0}}}
	atB := Map{Key(1): Rec{State: "b", Fields: map[string]Value{"x": 0, "y": 1, "h": 0}}}
	cases := map[string]struct {
		clause string
		m      Map
		want   bool
		err    string
	}{
		"a field the state does not observe is false":         {clause: "@eq(v, m[k].y) ∧ @at(m[k], b)", m: atA, want: false},
		"a field the state observes is read":                  {clause: "@eq(v, m[k].y) ∧ @at(m[k], b)", m: atB, want: true},
		"a field no state observes, though held, is an error": {clause: "@eq(v, m[k].h)", m: atB, err: "has no field h"},
		"a field no state has is an error":                    {clause: "@eq(v, m[k].z)", m: atB, err: "has no field z"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Setup
			f := mustParse(t, c.clause)

			// Execute
			got, err := Holds(f, Env{}, families, map[string]Value{"k": 1, "v": 1, "m": c.m}, nil)

			// Assert
			if c.err != "" {
				if err == nil || !strings.Contains(err.Error(), c.err) {
					t.Fatalf("Holds(%q) = %v, %v, want an error saying %q", c.clause, got, err, c.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("Holds(%q) = %v, want %v", c.clause, got, c.want)
			}
		})
	}
}
