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

// DiagramJSONVersion is the version of the shape DiagramJSON writes; a
// reader refuses another.
const DiagramJSONVersion = 1

// DiagramJSON is d in JSON, for a program such as csdfbmc: its states in
// order, with their labels and variables (a state with none has an empty
// list); the start state, its post and the line of the start edge; each
// edge with its channel, its fields in order (dir "?" for a field received,
// "!" for one sent), its label, its guard and post as FormulaJSON writes
// them, and its line; the end edges; and the assume notes. A diagram that
// is not well formed is an error.
func DiagramJSON(d *Diagram) (map[string]any, error) {
	if err := d.wellFormed(); err != nil {
		return nil, fmt.Errorf("core.DiagramJSON: %w", err)
	}
	labels := map[string]any{}
	vars := map[string]any{}
	for _, s := range d.States {
		if l, ok := d.Labels[s]; ok {
			labels[s] = l
		}
		vs := []any{}
		for _, v := range d.Vars[s] {
			vs = append(vs, v)
		}
		vars[s] = vs
	}
	states := []any{}
	for _, s := range d.States {
		states = append(states, s)
	}
	initPost, err := FormulaJSON(d.InitPost)
	if err != nil {
		return nil, fmt.Errorf("core.DiagramJSON: the start edge: %w", err)
	}
	edges := []any{}
	for _, e := range d.Edges {
		guard, err := FormulaJSON(e.Guard)
		if err != nil {
			return nil, fmt.Errorf("core.DiagramJSON: %s --> %s : %s: %w", e.Src, e.Dst, e.Event.Label(), err)
		}
		post, err := FormulaJSON(e.Post)
		if err != nil {
			return nil, fmt.Errorf("core.DiagramJSON: %s --> %s : %s: %w", e.Src, e.Dst, e.Event.Label(), err)
		}
		fields := []any{}
		for _, f := range e.Event.Fields {
			dir := "?"
			if f.Send {
				dir = "!"
			}
			fields = append(fields, map[string]any{"dir": dir, "name": f.Name})
		}
		edges = append(edges, map[string]any{
			"src": e.Src, "dst": e.Dst, "channel": e.Event.Channel, "fields": fields,
			"label": e.Event.Label(), "guard": guard, "post": post, "line": e.Line,
		})
	}
	ends := []any{}
	for _, e := range d.Ends {
		guard, err := FormulaJSON(e.Guard)
		if err != nil {
			return nil, fmt.Errorf("core.DiagramJSON: the end edge from %s: %w", e.Src, err)
		}
		ends = append(ends, map[string]any{"src": e.Src, "guard": guard})
	}
	assumes := []any{}
	for _, a := range d.Assumes {
		assumes = append(assumes, map[string]any{"id": a.ID, "text": a.Text})
	}
	return map[string]any{
		"version": DiagramJSONVersion, "name": d.Name, "states": states, "labels": labels, "vars": vars,
		"init": d.Init, "init_post": initPost, "init_line": d.InitLine,
		"edges": edges, "ends": ends, "assumes": assumes,
	}, nil
}
