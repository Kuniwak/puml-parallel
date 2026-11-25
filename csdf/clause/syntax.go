package clause

// The concrete syntax tree of a clause: what ParseSyntax reads, the names
// as written and nothing resolved. Formula turns it into the formula the
// tools work with. Each node holds Pos, the offset in runes into the clause
// text where it starts.

// Syntax is a clause as written: its conjuncts, joined by " ∧ ".
type Syntax struct{ Conjuncts []SynConjunct }

// SynConjunct is a conjunct as written: a *SynAtom, a *SynExists or a
// *SynBuiltin.
type SynConjunct interface{ synConjunct() }

// SynAtom is text and holes.
type SynAtom struct {
	Parts []SynPart
	Pos   int
}

// SynExists is @exists(Name : Type) (Body), the type as written.
type SynExists struct {
	Name string
	Type string
	Body Syntax
	Pos  int
}

// SynBuiltin is @Op(Args, States).
type SynBuiltin struct {
	Op     Op
	Args   []SynTerm
	States []string
	Pos    int
}

func (*SynAtom) synConjunct()    {}
func (*SynExists) synConjunct()  {}
func (*SynBuiltin) synConjunct() {}

// SynPart is a part of an atom: a *SynText or a *SynHole.
type SynPart interface{ synPart() }

// SynText is text of an atom, its escapes undone.
type SynText struct{ Text string }

// SynHole is a hole: $name, $name' (Braced false, Term a *SynVar), or
// ${term} (Braced true).
type SynHole struct {
	Term   SynTerm
	Braced bool
	Pos    int
}

func (*SynText) synPart() {}
func (*SynHole) synPart() {}

// SynTerm is a term as written: *SynVar, *SynLit, *SynIndex, *SynField,
// *SynApp or *SynPrime.
type SynTerm interface{ synTerm() }

// SynVar is a variable by its name, primed when a prime follows the name.
type SynVar struct {
	Name   string
	Primed bool
	Pos    int
}

// SynLit is a literal as written.
type SynLit struct {
	Text string
	Pos  int
}

// SynIndex is Map[Key]; Pos is where the whole path starts.
type SynIndex struct {
	Map, Key SynTerm
	Pos      int
}

// SynField is Of.Name; Pos is where the whole path starts.
type SynField struct {
	Of   SynTerm
	Name string
	Pos  int
}

// SynApp is Fn(Args).
type SynApp struct {
	Fn   string
	Args []SynTerm
	Pos  int
}

// SynPrime is a path with a prime after it, Of; Pos is where the path
// starts.
type SynPrime struct {
	Of  SynTerm
	Pos int
}

func (*SynVar) synTerm()   {}
func (*SynLit) synTerm()   {}
func (*SynIndex) synTerm() {}
func (*SynField) synTerm() {}
func (*SynApp) synTerm()   {}
func (*SynPrime) synTerm() {}
