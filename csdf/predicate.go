package csdf

import (
	"sort"
	"strings"
)

func Conjunction(g1, g2 Predicate) Predicate {
	if IsTrue(g1) {
		return g2
	}
	if IsTrue(g2) {
		return g1
	}
	return g1 + " ∧ " + g2
}

// disjoin combines predicates with a true-aware logical OR. "true" (or the empty
// default) is absorbing: if any disjunct is true the result is "true". Otherwise
// the distinct disjuncts are sorted and joined with " | " for stable output.
func DisjunctionAll(preds []Predicate) Predicate {
	seen := make(map[Predicate]struct{}, len(preds))
	disjuncts := make([]string, 0, len(preds))
	for _, p := range preds {
		if IsTrue(p) {
			return PredicateTrue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		disjuncts = append(disjuncts, string(p))
	}
	if len(disjuncts) == 0 {
		return PredicateTrue
	}
	sort.Strings(disjuncts)
	return Predicate(strings.Join(disjuncts, " ∨ "))
}
