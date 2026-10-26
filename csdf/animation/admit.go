package animation

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/core"
)

// AdmitByBuiltins refuses values entered for p's target when a builtin
// conjunct of p's guard or post (@eq, @ne, @in, @notin, @empty, @update,
// @frame, @at) is false of them and of the previous state's values: what
// Go decides without a glue. A builtin it cannot decide, one reading an
// event's field or an environment function, is left alone; so is every
// atom, which only a glue reads, and every ∃. A refusal is a
// *RefusedValuesError.
func AdmitByBuiltins(d *core.Diagram, p Pending, values []Value) error {
	before := map[string]core.Value{}
	if p.Prev != nil {
		for _, v := range p.Prev.Values {
			before[v.Name] = coreValue(d, v)
		}
	}
	after := map[string]core.Value{}
	for _, v := range values {
		after[v.Name] = coreValue(d, v)
	}
	for _, c := range []struct {
		what   string
		clause clause.And
	}{{"guard", p.Guard}, {"post", p.Post}} {
		for _, b := range builtins(c.clause) {
			ok, err := core.Holds(b, core.Env{}, before, after)
			if err != nil || ok {
				continue // undecided without a glue, or true
			}
			return refused("the %s does not hold of these values: %s", c.what, ClauseText(clause.And{Conjuncts: []clause.Formula{b}}))
		}
	}
	return nil
}

// builtins are the builtin conjuncts of c, conjunctions flattened; what
// lies under an ∃ is left out.
func builtins(c clause.And) []clause.Builtin {
	var out []clause.Builtin
	for _, f := range c.Conjuncts {
		switch f := f.(type) {
		case clause.Builtin:
			out = append(out, f)
		case clause.And:
			out = append(out, builtins(f)...)
		}
	}
	return out
}

// coreValue is v as core evaluates it: a map of a family's instances a
// core.Map of core.Recs, the rest as core.JSONValue makes it.
func coreValue(d *core.Diagram, v Value) core.Value {
	if _, ok := family(d, v.Name); ok {
		if entries, ok := v.Value.(map[string]any); ok {
			m := core.Map{}
			for id, e := range entries {
				at, fields, _ := instance(e)
				rec := core.Rec{State: at, Fields: map[string]core.Value{}}
				for name, f := range fields {
					rec.Fields[name] = core.JSONValue(f)
				}
				m[core.Key(id)] = rec
			}
			return m
		}
	}
	return core.JSONValue(v.Value)
}

// AmongSuggestions refuses values unless they are, as JSON values, those
// of one of suggestions: what a checker admits. Values are compared as
// literals are (clause.CanonicalValue): a number is told from a truth
// value and an integer from a float; negative zero is zero.
func AmongSuggestions(suggestions []Suggestion, values []Value) error {
	entered := make([]any, len(values))
	for i, v := range values {
		entered[i] = v.Value
	}
	got, err := clause.CanonicalValue(entered)
	if err != nil {
		return refused("the values cannot be compared with the checker's: %v", err)
	}
	for _, s := range suggestions {
		dec := json.NewDecoder(bytes.NewReader(s.Values))
		dec.UseNumber()
		var want any
		if err := dec.Decode(&want); err != nil {
			return fmt.Errorf("animation.AmongSuggestions: a suggestion is no JSON: %w", err)
		}
		w, err := clause.CanonicalValue(want)
		if err != nil {
			return fmt.Errorf("animation.AmongSuggestions: a suggestion is no value: %w", err)
		}
		if got == w {
			return nil
		}
	}
	return refused("the checker admits none of these values; enter one of its suggestions")
}
