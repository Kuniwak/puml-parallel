package nl

import (
	"github.com/Kuniwak/puml-parallel/lts/pure"
	"github.com/Kuniwak/puml-parallel/sets"
)

type LTS struct {
	Init         State
	InitPostCond string
	EdgesMap     map[State][]Edge
	Events       []pure.Event
}

type VisibleEvents struct {
	Events     sets.Set[pure.Event]
	Obligation Obligation
}

func (l *LTS) VisibleEvents(s pure.State) sets.Set[pure.Event] {
	en := make(map[pure.Event]struct{})
	for _, e := range l.EdgesMap[s] {
		if !e.EventID.IsTau() {
			en[e.EventID] = struct{}{}
		}
	}
	return en
}
