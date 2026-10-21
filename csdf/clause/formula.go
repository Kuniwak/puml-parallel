package clause

// Formula is a clause: a conjunction of atoms, builtins and ∃.
type Formula interface{ isFormula() }

// And is the conjunction of its conjuncts; none is true.
type And struct{ Conjuncts []Formula }

// Exists binds one variable in Body, where it is Bound{0}. Only tools write
// it (docs/CLAUSE_DESIGN.md). Hint is the name the variable is printed with
// when that captures nothing; it carries no meaning, and two formulas that
// differ only in hints are α-equivalent. Type names the type the variable
// ranges over (docs/CLAUSE_DESIGN.md, P16); it carries meaning.
type Exists struct {
	Hint string
	Type string
	Body And
}

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

func (And) isFormula()    {}
func (Exists) isFormula() {}
func (Atom) isFormula()   {}
