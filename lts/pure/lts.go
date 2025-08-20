package pure

import (
	"container/list"
	"github.com/Kuniwak/puml-parallel/sets"
)

type LTS struct {
	Init     State
	StateMap map[State]State
	EventMap map[Event]Event
	EdgesMap map[State][]Edge
	Events   []Event
}

func (l *LTS) VisibleEvents(s State) sets.Set[Event] {
	en := make(map[Event]struct{})
	for _, e := range l.EdgesMap[s] {
		if !e.Event.IsTau() {
			en[e.Event] = struct{}{}
		}
	}
	return en
}

func (l *LTS) IsStable(s State) bool {
	return !l.IsUnstable(s)
}

func (l *LTS) IsUnstable(s State) bool {
	for _, e := range l.EdgesMap[s] {
		if e.Event.IsTau() {
			return true
		}
	}
	return false
}

func (l *LTS) EpsilonClosure(from State) sets.Set[State] {
	seen := sets.NewSet[State]()
	queue := list.New()
	queue.PushBack(from)
	for queue.Len() > 0 {
		s := queue.Remove(queue.Front()).(State)
		for _, e := range l.EdgesMap[s] {
			if e.Event.IsTau() {
				if _, ok := seen[e.State]; !ok {
					seen[e.State] = struct{}{}
					queue.PushBack(e.State)
				}
			}
		}
	}
	return seen
}

func (l *LTS) WeakPost(spec sets.Set[State], a Event) sets.Set[State] {
	res := sets.NewSet[State]()
	starts := sets.NewSet[State]()
	for s := range spec {
		for t := range l.EpsilonClosure(s) {
			starts.Add(t)
		}
	}
	afterA := sets.NewSet[State]()
	for s := range starts {
		for _, e := range l.EdgesMap[s] {
			if e.Event == a {
				afterA.Add(e.State)
			}
		}
	}
	for t := range afterA {
		for u := range l.EpsilonClosure(t) {
			res.Add(u)
		}
	}
	return res
}
