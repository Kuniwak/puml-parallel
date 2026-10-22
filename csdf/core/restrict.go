package core

import (
	"fmt"
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
)

// Restrict is the promoted diagram d kept to the families of the maps keep,
// maps being every map d's promotion holds (docs/CLAUSE_DESIGN.md: an
// approximation kept for safety specifications). The left-out maps go from every state;
// an edge that moves only left-out maps goes; every clause that reads a
// left-out map is taken as true, and each clause dropped so, guard or
// post, is reported. An edge the kept families take with a left-out one keeps every
// field: the left-out side is the environment, offering any value. A
// clause is dropped whole: a post that chains two moves of a left-out map
// through an intermediate bound by ∃ is one clause with the kept maps'
// moves inside it, so they go too, and the kept maps are left free there,
// which a safety specification allows. The first warning says what a
// result about the restriction does not say about d.
func Restrict(d *Diagram, maps, keep []string) (*Diagram, []Warning, error) {
	if len(keep) == 0 {
		return nil, nil, fmt.Errorf("core.Restrict: it keeps no family")
	}
	for _, m := range keep {
		if !slices.Contains(maps, m) {
			return nil, nil, fmt.Errorf("core.Restrict: %s is no promoted map", m)
		}
	}
	gone := map[string]bool{}
	for _, m := range maps {
		if !slices.Contains(keep, m) {
			gone[m] = true
		}
	}
	reads := func(c clause.Formula) bool {
		return slices.ContainsFunc(clause.FreeVars(clause.And{Conjuncts: []clause.Formula{c}}), func(v clause.Free) bool { return gone[v.Name] })
	}
	warnings := []Warning{{Text: Approximation}}
	keepOf := func(f clause.And, event string) (clause.And, error) {
		var out []clause.Formula
		for _, c := range clause.Normalize(f).Conjuncts {
			if !reads(c) {
				out = append(out, c)
				continue
			}
			text, err := clause.Print(clause.And{Conjuncts: []clause.Formula{c}})
			if err != nil {
				return clause.And{}, fmt.Errorf("core.Restrict: %s: %w", event, err)
			}
			warnings = append(warnings, Warning{Event: event, Text: "dropped, as it reads a left-out map: " + text})
		}
		return clause.And{Conjuncts: out}, nil
	}
	out := &Diagram{Name: d.Name, Labels: d.Labels, States: slices.Clone(d.States), Init: d.Init, Vars: map[string][]string{}, Assumes: slices.Clone(d.Assumes), Ends: slices.Clone(d.Ends)}
	for s, vars := range d.Vars {
		for _, v := range vars {
			if !gone[v] {
				out.Vars[s] = append(out.Vars[s], v)
			}
		}
	}
	init, err := keepOf(d.InitPost, "[*]")
	if err != nil {
		return nil, nil, err
	}
	out.InitPost = init
	hidden := map[string]bool{}
	for _, e := range d.Edges {
		movesKept, movesGone := moves(e.Post, maps, keep)
		if movesGone && !movesKept {
			if err := onlyLeftOut(e, d, maps); err != nil {
				return nil, nil, fmt.Errorf("core.Restrict: %w", err)
			}
			if e.Event.Channel != Tau {
				// τ is no event to hide: a kept family's τ is its own.
				hidden[e.Event.Channel] = true
			}
			continue
		}
		if e.Guard, err = keepOf(e.Guard, e.Event.Channel); err != nil {
			return nil, nil, err
		}
		if e.Post, err = keepOf(e.Post, e.Event.Channel); err != nil {
			return nil, nil, err
		}
		out.Edges = append(out.Edges, e)
	}
	for _, e := range out.Edges {
		if hidden[e.Event.Channel] {
			return nil, nil, fmt.Errorf("core.Restrict: %s is taken by a kept family and by a left-out one alone, so it is not an event only the left-out families take", e.Event.Channel)
		}
	}
	return out, warnings, nil
}

// onlyLeftOut reports, as an error, an edge e of d moving only left-out
// maps that is not a step of the left-out instances alone: a self-loop of
// one mode leaving that mode's controller variables unchanged, which is
// what Promote generates. Only then is its event one the restriction may
// hide.
func onlyLeftOut(e Edge, d *Diagram, maps []string) error {
	name := fmt.Sprintf("%s --> %s : %s", e.Src, e.Dst, e.Event.Channel)
	if e.Src != e.Dst {
		return fmt.Errorf("%s moves only left-out maps but is no self-loop", name)
	}
	post := clause.Normalize(e.Post).Conjuncts
	for _, v := range d.Vars[e.Src] {
		if slices.Contains(maps, v) {
			continue
		}
		frame := clause.Normalize(Unchanged([]string{v})).Conjuncts[0]
		if !slices.ContainsFunc(post, func(c clause.Formula) bool {
			return clause.AlphaEqual(clause.And{Conjuncts: []clause.Formula{c}}, clause.And{Conjuncts: []clause.Formula{frame}})
		}) {
			return fmt.Errorf("%s moves only left-out maps but does not leave the controller variable %s unchanged", name, v)
		}
	}
	return nil
}

// Approximation is what a result about a restricted promotion does not
// say about the full one (docs/CLAUSE_DESIGN.md): a tool that checks a
// restriction says it with its result.
const Approximation = "a restriction keeps only safety: a deadlock or another refusal it shows is no conclusion about the full promotion, and a counterexample may be spurious"

// moves says whether post changes a kept map, and a left-out one: whether
// it primes the map other than in its frame m' = m.
func moves(post clause.And, maps, keep []string) (kept, gone bool) {
	for _, c := range clause.Normalize(post).Conjuncts {
		one := clause.And{Conjuncts: []clause.Formula{c}}
		for _, v := range clause.FreeVars(one) {
			if !v.Primed || !slices.Contains(maps, v.Name) {
				continue
			}
			if clause.AlphaEqual(one, clause.Normalize(Unchanged([]string{v.Name}))) {
				continue
			}
			if slices.Contains(keep, v.Name) {
				kept = true
			} else {
				gone = true
			}
		}
	}
	return kept, gone
}
