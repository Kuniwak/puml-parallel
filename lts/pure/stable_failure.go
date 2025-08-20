package pure

import (
	"container/list"
	"github.com/Kuniwak/puml-parallel/sets"
)

func RefinesStableFailures(spec, impl *LTS) bool {
	// 初期 ( { s | ι1 ε=⇒ s }, ι2 )
	initSpec := sets.NewSet[State]()
	for s := range spec.EpsilonClosure(spec.Init) {
		initSpec.Add(s)
	}
	initPair := AntichainElement{SpecSet: initSpec, Impl: impl.Init}

	working := list.New() // stack (DFS)
	working.PushBack(initPair)
	antichain := Antichain{}

	for working.Len() > 0 {
		// pop
		elem := working.Back()
		working.Remove(elem)
		curr := elem.Value.(AntichainElement)

		// antichain := antichain ⊎ (spec, impl)
		antichain = antichain.Insert(curr)

		// refusals(impl) ⊆ refusals(spec) ?
		if !RefusalsIncluded(curr.SpecSet, curr.Impl, spec, impl) {
			return false
		}

		for _, e := range impl.EdgesMap[curr.Impl] {
			var specPrime sets.Set[State]
			if e.Event.IsTau() {
				specPrime = curr.SpecSet
			} else {
				specPrime = spec.WeakPost(curr.SpecSet, e.Event)
			}

			// NOTE: specPrime is empty iff spec does not have a stable state.
			if len(specPrime) == 0 {
				return false
			}

			next := AntichainElement{SpecSet: specPrime, Impl: e.State}
			// NOTE: search not needed if (spec', impl') is already in the antichain.
			if !next.InDownwardClosure(antichain) {
				working.PushBack(next)
			}
		}
	}
	return true
}
