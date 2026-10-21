package clause

// Formula is a clause: a conjunction of atoms, builtins and ∃.
type Formula interface{ isFormula() }

// And is the conjunction of its conjuncts; none is true.
type And struct{ Conjuncts []Formula }

// Atom is text that a language interprets, with holes for the values it
// reads.
type Atom struct{ Parts []Part }

// Part is a piece of an atom: Text, or a Hole.
type Part interface{ isPart() }

// Text is literal text of an atom, without escapes.
type Text string

// Hole is a term an atom reads.
type Hole struct{ Term Term }

func (Text) isPart() {}
func (Hole) isPart() {}

func (And) isFormula()  {}
func (Atom) isFormula() {}
