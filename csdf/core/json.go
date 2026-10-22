package core

import (
	"fmt"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
)

// FormulaJSON is f as a list of its conjuncts in JSON: an atom in the shape
// a glue receives it (clause.AtomWire) with the terms its arguments stand
// for and the name of the environment function it denotes
// (clause.FunctionOf), a builtin by its op, and ∃ by the type it ranges over and its body;
// a hint carries no meaning and is not written. It is never nil; a formula
// that is not well formed (clause.WellFormed) is an error.
func FormulaJSON(f clause.And) ([]any, error) {
	if err := clause.WellFormed(f); err != nil {
		return nil, fmt.Errorf("core.FormulaJSON: %w", err)
	}
	return formulaJSON(f)
}

func formulaJSON(f clause.And) ([]any, error) {
	out := []any{}
	for _, c := range clause.Normalize(f).Conjuncts {
		switch c := c.(type) {
		case clause.Atom:
			w, terms, err := clause.Wire(c)
			if err != nil {
				return nil, err
			}
			fn, err := clause.FunctionOf(c)
			if err != nil {
				return nil, err
			}
			out = append(out, map[string]any{"atom": w, "function": fn.Name, "args": termsJSON(terms)})
		case clause.Builtin:
			b := map[string]any{"builtin": string(c.Op), "args": termsJSON(c.Args)}
			if c.State != "" {
				b["state"] = c.State
			}
			out = append(out, b)
		case clause.Exists:
			body, err := formulaJSON(c.Body)
			if err != nil {
				return nil, err
			}
			out = append(out, map[string]any{"exists": c.Type, "body": body})
		}
	}
	return out, nil
}

func termsJSON(ts []clause.Term) []any {
	out := make([]any, len(ts))
	for i, t := range ts {
		out[i] = termJSON(t)
	}
	return out
}

func termJSON(t clause.Term) any {
	switch t := t.(type) {
	case clause.Free:
		j := map[string]any{"free": t.Name}
		if t.Primed {
			j["primed"] = true
		}
		return j
	case clause.Bound:
		return map[string]any{"bound": t.Index}
	case clause.Index:
		return map[string]any{"index": map[string]any{"map": termJSON(t.Map), "key": termJSON(t.Key)}}
	case clause.Field:
		return map[string]any{"field": map[string]any{"of": termJSON(t.Of), "name": t.Name}}
	case clause.App:
		return map[string]any{"app": map[string]any{"fn": t.Fn, "args": termsJSON(t.Args)}}
	}
	return nil
}
