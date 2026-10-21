package obligation

import (
	"fmt"
	"maps"
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
)

// Axioms are what the predicate symbols the map builtins become mean, for
// those the clauses fs use: an empty map has no key; an update maps its key to
// its value and keeps every other entry; a frame keeps every entry but at its
// keys, one axiom for each number of keys; an instance is at one state at a
// time, for each two states csdf_at names of one family (StateConstant). csdf_in itself needs none.
//
// An obligation is stated without them, and a target assumes them. A claim
// that a formula is valid stays sound without them, though some valid claims
// are then unprovable; a claim that a formula can be satisfied, such as that
// an edge can be taken (Enabled) in some state, is sound only with them.
func Axioms(fs ...clause.Formula) []logic.Formula {
	var empty, update bool
	var frames []int
	states := map[string][]string{}
	for _, f := range fs {
		eachBuiltin(f, func(b clause.Builtin) {
			switch b.Op {
			case clause.OpEmpty:
				empty = true
			case clause.OpUpdate:
				update = true
			case clause.OpFrame:
				if n := len(b.Args) - 2; !slices.Contains(frames, n) {
					frames = append(frames, n)
				}
			case clause.OpAt:
				family, _ := familyOf(b.Args[0])
				if c := StateConstant(b); !slices.Contains(states[family], c) {
					states[family] = append(states[family], c)
				}
			}
		})
	}
	var axioms []logic.Formula
	if empty {
		axioms = append(axioms, emptyAxiom())
	}
	if update {
		axioms = append(axioms, updateAxiom())
	}
	slices.Sort(frames)
	for _, n := range frames {
		axioms = append(axioms, frameAxiom(n))
	}
	families := slices.Sorted(maps.Keys(states))
	for _, family := range families {
		cs := states[family]
		slices.Sort(cs)
		for i, c := range cs {
			for _, d := range cs[i+1:] {
				axioms = append(axioms, atAxiom(c, d))
			}
		}
	}
	return axioms
}

func eachBuiltin(f clause.Formula, visit func(clause.Builtin)) {
	switch f := f.(type) {
	case clause.And:
		for _, c := range f.Conjuncts {
			eachBuiltin(c, visit)
		}
	case clause.Exists:
		eachBuiltin(f.Body, visit)
	case clause.Builtin:
		visit(f)
	}
}

var (
	axN = logic.Var("n")
	axM = logic.Var("m")
	axK = logic.Var("k")
	axV = logic.Var("v")
	axJ = logic.Var("j")
	axT = logic.Var("t")
)

func in(k, m logic.Term) logic.Formula { return logic.Pred(PredIn, false, k, m) }

func emptyAxiom() logic.Formula {
	return logic.Forall([]logic.Var{axM, axK},
		logic.Implies(logic.Pred(PredEmpty, false, axM), logic.Not(in(axK, axM))))
}

// kept is that n and m agree at the key j: j is in either just when it is
// in the other, with the same value.
func kept() logic.Formula {
	return logic.And(
		logic.Implies(in(axJ, axN), in(axJ, axM)),
		logic.Implies(in(axJ, axM), logic.And(in(axJ, axN), logic.Eq(logic.Index(axN, axJ), logic.Index(axM, axJ)))))
}

// keptBut is that n and m agree at every key but keys.
func keptBut(keys ...logic.Term) logic.Formula {
	apart := make([]logic.Formula, len(keys))
	for i, k := range keys {
		apart[i] = logic.Not(logic.Eq(axJ, k))
	}
	return logic.Forall([]logic.Var{axJ}, logic.Implies(logic.And(apart...), kept()))
}

func updateAxiom() logic.Formula {
	return logic.Forall([]logic.Var{axN, axM, axK, axV},
		logic.Implies(logic.Pred(PredUpdate, false, axN, axM, axK, axV),
			logic.And(in(axK, axN), logic.Eq(logic.Index(axN, axK), axV), keptBut(axK))))
}

func frameAxiom(n int) logic.Formula {
	vars := []logic.Var{axN, axM}
	args := []logic.Term{axN, axM}
	keys := make([]logic.Term, n)
	for i := range n {
		k := logic.Var(fmt.Sprintf("k%d", i+1))
		vars = append(vars, k)
		args = append(args, k)
		keys[i] = k
	}
	return logic.Forall(vars, logic.Implies(logic.Pred(PredFrame, false, args...), keptBut(keys...)))
}

func atAxiom(s, t string) logic.Formula {
	at := func(state string) logic.Formula {
		return logic.Pred(PredAt, false, axT, logic.Apply(state))
	}
	return logic.Forall([]logic.Var{axT}, logic.Not(logic.And(at(s), at(t))))
}
