package animation

import (
	"github.com/Kuniwak/puml-parallel/v9/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v9/csdf/core"
)

// Determined are the values of p's target variables that builtin conjuncts
// of p's post fix by themselves, in declaration order: @empty(x') fixes x
// to the empty map, and @eq(x', t), either way round, to t when t is a
// literal or a variable before the step. A variable two such conjuncts fix
// to different values, or none fixes, is left out; so is all that lies
// under an ∃ or that only a glue decides.
func Determined(p Pending) []Value {
	before := map[string]any{}
	if p.Prev != nil {
		for _, v := range p.Prev.Values {
			before[v.Name] = v.Value
		}
	}
	fixed := map[string]any{}
	conflicting := map[string]bool{}
	for _, b := range builtins(p.Post) {
		name, v, ok := fixes(b, before)
		if !ok {
			continue
		}
		if w, seen := fixed[name]; seen && core.Key(core.JSONValue(w)) != core.Key(core.JSONValue(v)) {
			conflicting[name] = true
		}
		fixed[name] = v
	}
	var out []Value
	for _, name := range p.Target.Vars {
		if v, ok := fixed[name]; ok && !conflicting[name] {
			out = append(out, Value{Name: name, Value: v})
		}
	}
	return out
}

// fixes is the variable b fixes after the step and the value it fixes it
// to, if b fixes one by itself.
func fixes(b clause.Builtin, before map[string]any) (string, any, bool) {
	switch b.Op {
	case clause.OpEmpty:
		if x, ok := b.Args[0].(clause.Free); ok && x.Primed {
			return x.Name, map[string]any{}, true
		}
	case clause.OpEq:
		for _, pair := range [][2]clause.Term{{b.Args[0], b.Args[1]}, {b.Args[1], b.Args[0]}} {
			x, ok := pair[0].(clause.Free)
			if !ok || !x.Primed {
				continue
			}
			if v, ok := valueOf(pair[1], before); ok {
				return x.Name, v, true
			}
		}
	}
	return "", nil, false
}

// valueOf is the value of t when it is a literal or a variable before the
// step whose value is known.
func valueOf(t clause.Term, before map[string]any) (any, bool) {
	switch t := t.(type) {
	case clause.Lit:
		v, err := t.Decode()
		if err != nil {
			return nil, false
		}
		return clause.UnsignedZeros(v), true
	case clause.Free:
		if t.Primed {
			return nil, false
		}
		v, ok := before[t.Name]
		return v, ok
	}
	return nil, false
}
