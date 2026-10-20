package csdf

import (
	"sort"
	"strings"
)

// ClauseConnective joins the clauses of one predicate. A predicate is a
// conjunction of clauses, and the clause is the unit the tools treat as opaque
// text: the connective, with the spaces around it, is grammar, not wording, so
// a clause must not contain it (docs/SYNTAX.md).
const ClauseConnective = " ∧ "

func Conjunction(g1, g2 Predicate) Predicate {
	if IsTrue(g1) {
		return g2
	}
	if IsTrue(g2) {
		return g1
	}
	return g1 + ClauseConnective + g2
}

// Clauses splits a predicate into the clauses it conjoins. The omitted
// predicate, and the literal true, have no clause.
func (p Predicate) Clauses() []Predicate {
	if IsTrue(p) {
		return nil
	}
	parts := strings.Split(string(p), ClauseConnective)
	clauses := make([]Predicate, 0, len(parts))
	for _, part := range parts {
		clauses = append(clauses, Predicate(strings.TrimSpace(part)))
	}
	return clauses
}

// ConjoinClauses is the inverse of Clauses: no clause is the literal true.
func ConjoinClauses(clauses []Predicate) Predicate {
	if len(clauses) == 0 {
		return PredicateTrue
	}
	parts := make([]string, len(clauses))
	for i, c := range clauses {
		parts[i] = string(c)
	}
	return Predicate(strings.Join(parts, ClauseConnective))
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
