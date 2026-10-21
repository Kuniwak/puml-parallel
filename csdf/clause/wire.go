package clause

import "reflect"

// AtomWire is an atom as a glue receives it (docs/CLAUSE_DESIGN.md, P8): the
// text without its holes, and for each hole the argument it reads. Text has
// one more piece than Holes: Text[0], the value of Holes[0], Text[1], and so
// on. Holes[i] indexes the atom's arguments, the distinct terms its holes
// read in order of first appearance; Primed says which of them are values
// after the step. A glue never sees a name: a value comes with the index of
// its argument.
type AtomWire struct {
	Text   []string `json:"text"`
	Holes  []int    `json:"holes"`
	Primed []bool   `json:"primed"`
}

// Wire is a as a glue receives it, with the terms its arguments stand for,
// in the order of their indices.
func Wire(a Atom) (AtomWire, []Term) {
	w := AtomWire{Text: []string{""}, Holes: []int{}, Primed: []bool{}}
	var terms []Term
	for _, p := range Normalize(a).Conjuncts[0].(Atom).Parts {
		switch p := p.(type) {
		case Text:
			w.Text[len(w.Text)-1] += string(p)
		case Hole:
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
	return w, terms
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
