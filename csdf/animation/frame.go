package animation

import "github.com/Kuniwak/puml-parallel/v6/csdf/clause"

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
	emptied, unchanged, rest := SplitFrame(post)
	return Frame{Rest: ClauseText(rest), Emptied: emptied, Unchanged: unchanged}
}

// SplitFrame is post split as FrameOf splits it: the variables emptied, the
// variables kept, and the rest of its conjuncts as a clause.
func SplitFrame(post clause.And) (emptied, unchanged []string, rest clause.And) {
	for _, c := range conjuncts(post) {
		b, ok := c.(clause.Builtin)
		switch {
		case ok && b.Op == clause.OpEmpty && isPrimed(b.Args[0]):
			emptied = append(emptied, b.Args[0].(clause.Free).Name)
		case ok && b.Op == clause.OpEq && keeps(b.Args[0], b.Args[1]):
			unchanged = append(unchanged, b.Args[0].(clause.Free).Name)
		default:
			rest.Conjuncts = append(rest.Conjuncts, c)
		}
	}
	return emptied, unchanged, rest
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
