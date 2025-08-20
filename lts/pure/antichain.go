package pure

import (
	"fmt"
	"github.com/Kuniwak/puml-parallel/sets"
	"io"
	"strings"
)

type Antichain []AntichainElement

func (a Antichain) PanicIfInvalid() {
	for _, x := range a {
		for _, y := range a {
			if Equal(x, y) {
				continue
			}

			if LessOrEqual(x, y) {
				panic(fmt.Sprintf("antichain is not valid: %s %s ≦ %s %s", x.Impl, sets.String(x.SpecSet), y.Impl, sets.String(y.SpecSet)))
			}
			if LessOrEqual(y, x) {
				panic(fmt.Sprintf("antichain is not valid: %s %s ≧ %s %s", x.Impl, sets.String(x.SpecSet), y.Impl, sets.String(y.SpecSet)))
			}
		}
	}
}

type AntichainElement struct {
	SpecSet sets.Set[State]
	Impl    State
}

// InDownwardClosure is antichain membership test: (spec', impl') ∈↓A ?
func (x AntichainElement) InDownwardClosure(A Antichain) bool {
	for _, y := range A {
		if LessOrEqual(y, x) {
			return true
		}
	}
	return false
}

func (x AntichainElement) WriteTo(w io.Writer) (int64, error) {
	var n int64
	n2, err := x.Impl.WriteTo(w)
	n += n2
	if err != nil {
		return n, err
	}

	n2, err = sets.WriteTo(w, x.SpecSet)
	n += n2
	return n, err
}

func (x AntichainElement) String() string {
	sb := &strings.Builder{}
	if _, err := x.WriteTo(sb); err != nil {
		panic(fmt.Errorf("AntichainElement#String: %w", err))
	}
	return sb.String()
}

func Equal(x, y AntichainElement) bool {
	return x.Impl == y.Impl && sets.Equal(x.SpecSet, y.SpecSet)
}

// LessOrEqual is y ≤ x <=> x.Impl == y.Impl && y.SpecSet ⊆ x.SpecSet
func LessOrEqual(y, x AntichainElement) bool {
	return x.Impl == y.Impl && sets.IsSubset(y.SpecSet, x.SpecSet)
}

// Insert inserts to the antichain
func (a Antichain) Insert(x AntichainElement) Antichain {
	if x.InDownwardClosure(a) {
		return a
	}

	out := make([]AntichainElement, 0, len(a)+1)
	for _, y := range a {
		if !LessOrEqual(x, y) {
			out = append(out, y)
		}
	}
	return append(out, x)
}
