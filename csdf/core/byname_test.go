package core

import (
	"maps"

	"github.com/Kuniwak/puml-parallel/csdf/types"
)

// The tests of promotion write their diagrams without types, and type
// each name by itself, as every name was typed before diagrams declared
// their types: byName declares them so.

// declaredByName is d with each variable declared of the type named like
// it, and each channel with fields the types of the fields of its first
// edge, each named like the field or, sent, like the variable's type.
func declaredByName(d *Diagram) *Diagram {
	out := *d
	out.Types = maps.Clone(d.Types)
	if out.Types == nil {
		out.Types = map[string]types.Type{}
	}
	for _, vs := range d.Vars {
		for _, v := range vs {
			if _, ok := out.Types[v]; !ok {
				out.Types[v] = types.Named(v)
			}
		}
	}
	out.Channels = maps.Clone(d.Channels)
	for _, e := range d.Edges {
		if _, ok := out.Channels[e.Event.Channel]; ok || len(e.Event.Fields) == 0 || e.Event.Channel == Tau {
			continue
		}
		if out.Channels == nil {
			out.Channels = map[string][]types.Type{}
		}
		var ts []types.Type
		for _, f := range e.Event.Fields {
			if t, ok := out.Types[f.Name]; ok && f.Send {
				ts = append(ts, t)
			} else {
				ts = append(ts, types.Named(f.Name))
			}
		}
		out.Channels[e.Event.Channel] = ts
	}
	return &out
}

// familyByName is f with its IDs of the type named like its ID variable,
// an instance of the type named like its map, and its local diagram
// declared by name.
func familyByName(f Family) Family {
	if f.Key.IsZero() {
		f.Key = types.Named(f.ID)
	}
	if f.Val.IsZero() {
		f.Val = types.Named(f.Map)
	}
	if f.Local != nil {
		f.Local = declaredByName(f.Local)
	}
	return f
}

// byName is p with its families, its controller variables and its
// hand-written edges' channels declared by name.
func byName(p Promotion) Promotion {
	fs := make([]Family, len(p.Families))
	for i, f := range p.Families {
		fs[i] = familyByName(f)
	}
	p.Families = fs
	controller := declaredByName(&Diagram{Vars: p.Vars, Types: p.Types, Channels: p.Channels, Edges: p.Edges})
	if len(controller.Types) > 0 {
		p.Types = controller.Types
	}
	p.Channels = controller.Channels
	return p
}

// promote is Promote(byName(p)).
func promote(p Promotion) (*Diagram, []Warning, error) { return Promote(byName(p)) }
