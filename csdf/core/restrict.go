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
// left-out map is taken as true, and each guard clause dropped so is
// reported. An edge the kept families take with a left-out one keeps every
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
	keepOf := func(f clause.And, event string, report bool) clause.And {
		var out []clause.Formula
		for _, c := range clause.Normalize(f).Conjuncts {
			if !reads(c) {
				out = append(out, c)
				continue
			}
			if report {
				text, err := clause.Print(clause.And{Conjuncts: []clause.Formula{c}})
				if err != nil {
					text = fmt.Sprintf("%v", c)
				}
				warnings = append(warnings, Warning{Event: event, Text: "dropped, as it reads a left-out map: " + text})
			}
		}
		return clause.And{Conjuncts: out}
	}
	out := &Diagram{States: slices.Clone(d.States), Init: d.Init, Vars: map[string][]string{}, Assumes: slices.Clone(d.Assumes), Ends: slices.Clone(d.Ends)}
	for s, vars := range d.Vars {
		for _, v := range vars {
			if !gone[v] {
				out.Vars[s] = append(out.Vars[s], v)
			}
		}
	}
	out.InitPost = keepOf(d.InitPost, "[*]", false)
	for _, e := range d.Edges {
		movesKept, movesGone := moves(e.Post, maps, keep)
		if movesGone && !movesKept {
			continue
		}
		e.Guard = keepOf(e.Guard, e.Event.Channel, true)
		e.Post = keepOf(e.Post, e.Event.Channel, false)
		out.Edges = append(out.Edges, e)
	}
	return out, warnings, nil
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
