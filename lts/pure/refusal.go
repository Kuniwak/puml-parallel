package pure

import (
	"github.com/Kuniwak/puml-parallel/sets"
)

// RefusalsIncluded is true iff refusals(impl) ⊆ refusals(spec)
func RefusalsIncluded(specSet sets.Set[State], impl State, specLTS, implLTS *LTS) bool {
	// 仕様側 ε-閉包（集合版）
	specStableEnSets := make([]sets.Set[Event], 0)
	seen := sets.NewSet[State]()
	for s := range specSet {
		for t := range specLTS.EpsilonClosure(s) {
			if seen.Contains(t) {
				continue
			}
			seen.Add(t)
			if specLTS.IsStable(t) {
				specStableEnSets = append(specStableEnSets, specLTS.VisibleEvents(t))
			}
		}
	}

	for t := range implLTS.EpsilonClosure(impl) {
		if implLTS.IsUnstable(t) {
			continue
		}
		enT := implLTS.VisibleEvents(t)
		ok := false
		for _, enS := range specStableEnSets {
			if sets.IsSubset(enS, enT) {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}
