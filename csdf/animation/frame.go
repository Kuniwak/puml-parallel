package animation

import "github.com/Kuniwak/puml-parallel/v5/csdf/clause"

// Frame is a post split for people: the variables its conjuncts @empty(x')
// empty, those its conjuncts @eq(x', x), either way round, keep, and the
// rest of its conjuncts as ClauseText writes them.
type Frame struct {
	Rest      string   `json:"rest"`
	Emptied   []string `json:"emptied,omitempty"`
	Unchanged []string `json:"unchanged,omitempty"`
}

// FrameOf is post split into its Frame, conjuncts of nested ∧ included.
func FrameOf(post clause.And) Frame {
	var f Frame
	var rest clause.And
	for _, c := range conjuncts(post) {
		b, ok := c.(clause.Builtin)
		switch {
		case ok && b.Op == clause.OpEmpty && isPrimed(b.Args[0]):
			f.Emptied = append(f.Emptied, b.Args[0].(clause.Free).Name)
		case ok && b.Op == clause.OpEq && keeps(b.Args[0], b.Args[1]):
			f.Unchanged = append(f.Unchanged, b.Args[0].(clause.Free).Name)
		default:
			rest.Conjuncts = append(rest.Conjuncts, c)
		}
	}
	f.Rest = ClauseText(rest)
	return f
}

// conjuncts are the conjuncts of c, those of nested ∧ in their place.
func conjuncts(c clause.And) []clause.Formula {
	var out []clause.Formula
	for _, f := range c.Conjuncts {
		if inner, ok := f.(clause.And); ok {
			out = append(out, conjuncts(inner)...)
			continue
		}
		out = append(out, f)
	}
	return out
}

func isPrimed(t clause.Term) bool {
	x, ok := t.(clause.Free)
	return ok && x.Primed
}

// keeps says whether a and b are x and x', either way round.
func keeps(a, b clause.Term) bool {
	x, ok1 := a.(clause.Free)
	y, ok2 := b.(clause.Free)
	return ok1 && ok2 && x.Name == y.Name && x.Primed != y.Primed
}
