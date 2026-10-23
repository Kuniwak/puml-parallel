package clause

import (
	"fmt"
	"reflect"

	"github.com/Kuniwak/puml-parallel/csdf/ident"
)

// AtomWire is an atom as a glue receives it (docs/CLAUSE_DESIGN.md, P8): the
// text without its holes, and for each hole the argument it reads. Text has
// one more piece than Holes: Text[0], the value of Holes[0], Text[1], and so
// on. Holes[i] indexes the atom's arguments, the distinct terms its holes
// read in order of first appearance; Primed says which of them are values
// after the step. A glue never sees a name: a value comes with the index of
// its argument. Text after a hole that starts with a character that could
// continue a name starts with a space, the one that ended the hole: the glue
// reads a hole as something of its own, which would run into the text.
type AtomWire struct {
	Text   []string `json:"text"`
	Holes  []int    `json:"holes"`
	Primed []bool   `json:"primed"`
}

// Wire is a as a glue receives it, with the terms its arguments stand for,
// in the order of their indices. a may sit under binders: a hole may read an
// index, whose value the caller gives with the others.
func Wire(a Atom) (AtomWire, []Term, error) {
	if err := shaped(a); err != nil {
		return AtomWire{}, nil, fmt.Errorf("clause.Wire: %w", err)
	}
	if err := wellFormed(a); err != nil {
		return AtomWire{}, nil, fmt.Errorf("clause.Wire: %w", err)
	}
	for _, p := range a.Parts {
		if h, ok := p.(Hole); ok && primedBelowRoot(h.Term) {
			return AtomWire{}, nil, fmt.Errorf("clause.Wire: %#v: only the variable a term starts from may be primed", h.Term)
		}
	}
	w := AtomWire{Text: []string{""}, Holes: []int{}, Primed: []bool{}}
	var terms []Term
	afterHole := false
	for _, p := range normalAtom(a).Parts {
		switch p := p.(type) {
		case Text:
			if afterHole && ident.IsRune(firstRune(string(p))) {
				p = " " + p
			}
			w.Text[len(w.Text)-1] += string(p)
			afterHole = false
		case Hole:
			afterHole = true
			i := indexOf(terms, p.Term)
			if i < 0 {
				i = len(terms)
				terms = append(terms, p.Term)
				w.Primed = append(w.Primed, isPrimed(p.Term))
			}
			w.Holes = append(w.Holes, i)
			w.Text = append(w.Text, "")
		}
	}
	return w, terms, nil
}

func indexOf(terms []Term, t Term) int {
	for i, u := range terms {
		if reflect.DeepEqual(u, t) {
			return i
		}
	}
	return -1
}

// isPrimed says whether t is a value after the step: its root is a primed
// free variable.
func isPrimed(t Term) bool {
	v, ok := Root(t).(Free)
	return ok && v.Primed
}

// primedBelowRoot says whether t reads a value after the step anywhere but
// at its root: in a key, or in an argument of a function.
func primedBelowRoot(t Term) bool {
	switch t := t.(type) {
	case Index:
		return primedBelowRoot(t.Map) || hasPrimed(t.Key)
	case Field:
		return primedBelowRoot(t.Of)
	case App:
		return hasPrimed(t)
	}
	return false
}

func hasPrimed(t Term) bool {
	found := false
	MapTerm(t, 0, func(v Term, _ int) Term {
		if f, ok := v.(Free); ok && f.Primed {
			found = true
		}
		return v
	})
	return found
}

// Pieces are the text of a in normal form as a holds it, without escapes:
// one piece before its first hole and one after each, so one more than its
// holes. Unlike the text of Wire, a piece after a hole is not spaced.
func Pieces(a Atom) []string {
	pieces := []string{""}
	for _, p := range normalAtom(a).Parts {
		switch p := p.(type) {
		case Text:
			pieces[len(pieces)-1] += string(p)
		case Hole:
			pieces = append(pieces, "")
		}
	}
	return pieces
}

// AtomOf is the atom whose text is pieces, the hole between pieces[i] and
// pieces[i+1] reading args[holes[i]]: the inverse of Pieces with the holes
// and terms of Wire. pieces must be one more than holes, and every hole must
// read an argument.
func AtomOf(pieces []string, holes []int, args []Term) (Atom, error) {
	if len(pieces) != len(holes)+1 {
		return Atom{}, fmt.Errorf("clause.AtomOf: an atom has one piece more than holes, not %d pieces for %d holes", len(pieces), len(holes))
	}
	parts := []Part{Text(pieces[0])}
	for i, h := range holes {
		if h < 0 || h >= len(args) {
			return Atom{}, fmt.Errorf("clause.AtomOf: hole %d reads argument %d, but the atom has %d", i, h, len(args))
		}
		parts = append(parts, Hole{Term: args[h]}, Text(pieces[i+1]))
	}
	return normalAtom(Atom{Parts: parts}), nil
}
