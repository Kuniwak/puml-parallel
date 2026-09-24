package transtable

import (
	"slices"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// Literal is a guard of the diagram, or its negation. The guard is natural
// language and is never evaluated here.
type Literal struct {
	Pred    csdf.Predicate
	Negated bool
}

// Cond is the conjunction of its literals, which come in the order of the path
// they were collected along. The empty Cond is true.
type Cond []Literal

// and returns c with the guard g conjoined. A true guard leaves c as it is.
// The result never shares its backing array with c, so the conditions of two
// branches of one path cannot overwrite each other.
func (c Cond) and(g csdf.Predicate) Cond {
	if csdf.IsTrue(g) {
		return c
	}
	return append(slices.Clip(c), Literal{Pred: g})
}

// andNot returns c with the negation of every guard in gs conjoined.
func (c Cond) andNot(gs []csdf.Predicate) Cond {
	c = slices.Clip(c)
	for _, g := range gs {
		c = append(c, Literal{Pred: g, Negated: true})
	}
	return c
}
