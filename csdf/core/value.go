package core

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
)

// Value is a value a variable or a field holds: a scalar the environment
// gives (an int, a string, …), a Rec or a Map.
type Value any

// Rec is the value of one instance of a family: the state of its local
// diagram, and the values of that state's variables.
type Rec struct {
	State  string
	Fields map[string]Value
}

// Map is a map from instance IDs, keyed by Key, to the instances' Recs.
type Map map[string]Value

// Key is v written canonically: two values are equal when their keys are.
// A JSON value is written as the literal it is (clause.CanonicalValue), so
// that a string is told from a number and a truth value, an integer from
// a float, and 0.0 from -0.0.
func Key(v Value) string {
	switch v := v.(type) {
	case Rec:
		names := make([]string, 0, len(v.Fields))
		for n := range v.Fields {
			names = append(names, n)
		}
		sort.Strings(names)
		parts := make([]string, len(names))
		for i, n := range names {
			parts[i] = n + "=" + Key(v.Fields[n])
		}
		return "⟨" + v.State + "|" + strings.Join(parts, ",") + "⟩"
	case Map:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, len(keys))
		for i, k := range keys {
			parts[i] = k + "↦" + Key(v[k])
		}
		return "{" + strings.Join(parts, ",") + "}"
	case []any:
		parts := make([]string, len(v))
		for i, x := range v {
			parts[i] = Key(x)
		}
		return "[" + strings.Join(parts, ",") + "]"
	}
	if s, err := clause.CanonicalValue(v); err == nil {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// JSONValue is v, a value as encoding/json decodes it, as core holds it:
// an object a Map of its keys, an array of such values, the rest as it is.
func JSONValue(v any) Value {
	switch v := v.(type) {
	case map[string]any:
		m := Map{}
		for k, x := range v {
			m[Key(k)] = JSONValue(x)
		}
		return m
	case []any:
		out := make([]any, len(v))
		for i, x := range v {
			out[i] = JSONValue(x)
		}
		return out
	}
	return v
}
