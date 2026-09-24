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

// and returns c with the guard g conjoined; a true guard adds nothing. The
// result has no spare capacity, so appending to it - as the next step of a
// path does, or a caller of Build - never writes into c or into another
// result, and the conditions of two branches cannot overwrite each other.
func (c Cond) and(g csdf.Predicate) Cond {
	if csdf.IsTrue(g) {
		return slices.Clip(c)
	}
	return slices.Clip(append(slices.Clip(c), Literal{Pred: g}))
}

// andNot returns c with the negation of every guard in gs conjoined, with no
// spare capacity either.
func (c Cond) andNot(gs []csdf.Predicate) Cond {
	c = slices.Clip(c)
	for _, g := range gs {
		c = append(c, Literal{Pred: g, Negated: true})
	}
	return slices.Clip(c)
}
