package core

import (
	"fmt"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/lts"
)

// Expand is the meaning of d (docs/CLAUSE_DESIGN.md): the graph without
// state variables, over the values env gives. A label is the channel and
// the values of the event's fields, joined by "."; an internal event is τ.
func Expand(d *Diagram, env Env) (*lts.LTS, error) {
	x := &expander{d: d, env: env, ids: map[string]lts.State{}}
	inits, err := x.posts(d.Init, valuation{before: map[string]Value{}}, d.InitPost)
	if err != nil {
		return nil, fmt.Errorf("core.Expand: the start edge: %w", err)
	}
	// A start edge with several initial values is an internal choice
	// among them.
	root := x.fresh()
	out := lts.New(root)
	for _, v := range inits {
		id, _ := x.id(d.Init, v)
		out.Add(root, lts.Tau, id)
	}
	for len(x.queue) > 0 {
		cur := x.queue[0]
		x.queue = x.queue[1:]
		if err := x.step(out, cur); err != nil {
			return nil, fmt.Errorf("core.Expand: %w", err)
		}
	}
	return out, nil
}

type node struct {
	state string
	vars  map[string]Value
}

type expander struct {
	d     *Diagram
	env   Env
	ids   map[string]lts.State
	nodes []node
	queue []lts.State
	next  int
}

func (x *expander) fresh() lts.State {
	id := x.next
	x.next++
	x.nodes = append(x.nodes, node{})
	return id
}

func (x *expander) id(state string, vars map[string]Value) (lts.State, bool) {
	k := state + "|" + Key(Map(vars))
	if id, ok := x.ids[k]; ok {
		return id, false
	}
	id := x.fresh()
	x.nodes[id] = node{state: state, vars: vars}
	x.ids[k] = id
	x.queue = append(x.queue, id)
	return id, true
}

func (x *expander) step(out *lts.LTS, cur lts.State) error {
	n := x.nodes[cur]
	for _, e := range x.d.Edges {
		if e.Src != n.state {
			continue
		}
		for _, fields := range x.offers(e.Event) {
			before := map[string]Value{}
			for k, v := range n.vars {
				before[k] = v
			}
			for k, v := range fields {
				before[k] = v
			}
			label, err := x.label(e.Event, before)
			if err != nil {
				return err
			}
			ev := evaluator{env: x.env, val: valuation{before: before}}
			ok, err := ev.holds(e.Guard, nil)
			if err != nil {
				return fmt.Errorf("%s --> %s : %s: the guard: %w", e.Src, e.Dst, e.Event.Channel, err)
			}
			if !ok {
				continue
			}
			nexts, err := x.posts(e.Dst, valuation{before: before}, e.Post)
			if err != nil {
				return fmt.Errorf("%s --> %s : %s: the post: %w", e.Src, e.Dst, e.Event.Channel, err)
			}
			for _, v := range nexts {
				id, _ := x.id(e.Dst, v)
				out.Add(cur, label, id)
			}
		}
	}
	return nil
}

// offers are the values of the fields an event receives, every combination
// of their types.
func (x *expander) offers(e Event) []map[string]Value {
	out := []map[string]Value{{}}
	for _, f := range e.Fields {
		if f.Send {
			continue
		}
		var grown []map[string]Value
		for _, o := range out {
			for _, v := range x.env.Types[f.Name] {
				m := map[string]Value{}
				for k, w := range o {
					m[k] = w
				}
				m[f.Name] = v
				grown = append(grown, m)
			}
		}
		out = grown
	}
	return out
}

func (x *expander) label(e Event, vals map[string]Value) (string, error) {
	if e.Channel == Tau {
		return lts.Tau, nil
	}
	parts := []string{e.Channel}
	for _, f := range e.Fields {
		v, ok := vals[f.Name]
		if !ok {
			return "", fmt.Errorf("%s sends %s, which is bound neither by the state nor by the event", e.Channel, f.Name)
		}
		parts = append(parts, Key(v))
	}
	return strings.Join(parts, "."), nil
}

// posts are the values of dst's variables that post allows after a step
// from before.
func (x *expander) posts(dst string, before valuation, post clause.And) ([]map[string]Value, error) {
	cands := []map[string]Value{{}}
	for _, v := range x.d.Vars[dst] {
		var grown []map[string]Value
		for _, c := range cands {
			for _, val := range x.env.Types[v] {
				m := map[string]Value{}
				for k, w := range c {
					m[k] = w
				}
				m[v] = val
				grown = append(grown, m)
			}
		}
		cands = grown
	}
	var out []map[string]Value
	for _, c := range cands {
		ev := evaluator{env: x.env, val: valuation{before: before.before, after: c}}
		ok, err := ev.holds(post, nil)
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, c)
		}
	}
	return out, nil
}
