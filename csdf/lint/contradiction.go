package lint

import (
	"reflect"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
)

// Contradictory reports whether f is false on its face: whether its
// builtins alone, read as the skeleton defines them, exclude every
// valuation. An atom is never evaluated, so a formula false only by what
// its atoms say is not Contradictory. The facts read are, among the
// conjuncts of f or of the body of one of its ∃:
//
//   - @ne(t, t);
//   - @eq(a, b) beside @ne(a, b) or @ne(b, a);
//   - @in(k, m) beside @notin(k, m);
//   - @empty(m) beside @in(k, m).
//
// Terms are the same when they are written the same, up to the names of
// bound variables; two terms written differently may still be equal, and
// then nothing is concluded.
func Contradictory(f clause.And) bool {
	f = clause.Normalize(f)
	var bs []clause.Builtin
	for _, c := range f.Conjuncts {
		switch c := c.(type) {
		case clause.Builtin:
			bs = append(bs, c)
		case clause.Exists:
			if Contradictory(c.Body) {
				return true
			}
		}
	}
	for _, b := range bs {
		if b.Op == clause.OpNe && same(b.Args[0], b.Args[1]) {
			return true
		}
		for _, o := range bs {
			if excludes(b, o) {
				return true
			}
		}
	}
	return false
}

// excludes reports whether the builtins b and o cannot hold together, by
// the facts Contradictory lists.
func excludes(b, o clause.Builtin) bool {
	switch {
	case b.Op == clause.OpEq && o.Op == clause.OpNe:
		return same(b.Args[0], o.Args[0]) && same(b.Args[1], o.Args[1]) ||
			same(b.Args[0], o.Args[1]) && same(b.Args[1], o.Args[0])
	case b.Op == clause.OpIn && o.Op == clause.OpNotIn:
		return same(b.Args[0], o.Args[0]) && same(b.Args[1], o.Args[1])
	case b.Op == clause.OpEmpty && o.Op == clause.OpIn:
		return same(b.Args[0], o.Args[1])
	}
	return false
}

func same(a, b clause.Term) bool { return reflect.DeepEqual(a, b) }
