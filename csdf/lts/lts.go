// Package lts is finite labelled transition systems and their equivalence
// in the stable failures model of CSP (docs/CLAUSE_DESIGN.md), the meaning
// the constructors on clauses are tested against.
package lts

import (
	"sort"
	"strconv"
	"strings"
)

// Tau is the label of an internal step.
const Tau = "τ"

// State names a state of an LTS.
type State = int

// LTS is a finite labelled transition system.
type LTS struct {
	Init  State
	Trans map[State]map[string][]State
}

// New is the LTS with initial state init and no transitions.
func New(init State) *LTS {
	return &LTS{Init: init, Trans: map[State]map[string][]State{init: {}}}
}

// Add adds the transition src -label-> dst.
func (l *LTS) Add(src State, label string, dst State) {
	if l.Trans[src] == nil {
		l.Trans[src] = map[string][]State{}
	}
	if l.Trans[dst] == nil {
		l.Trans[dst] = map[string][]State{}
	}
	for _, d := range l.Trans[src][label] {
		if d == dst {
			return
		}
	}
	l.Trans[src][label] = append(l.Trans[src][label], dst)
}

// set is a set of states, written canonically so that it can be a key.
type set map[State]bool

func (s set) key() string {
	ids := make([]int, 0, len(s))
	for id := range s {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	var b strings.Builder
	for _, id := range ids {
		b.WriteString(strconv.Itoa(id))
		b.WriteByte(',')
	}
	return b.String()
}

func (l *LTS) tauClosure(s set) set {
	out := set{}
	stack := []State{}
	for id := range s {
		out[id] = true
		stack = append(stack, id)
	}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, d := range l.Trans[id][Tau] {
			if !out[d] {
				out[d] = true
				stack = append(stack, d)
			}
		}
	}
	return out
}

func (l *LTS) after(s set, label string) set {
	next := set{}
	for id := range s {
		for _, d := range l.Trans[id][label] {
			next[d] = true
		}
	}
	return l.tauClosure(next)
}

func (l *LTS) visible(s set) map[string]bool {
	out := map[string]bool{}
	for id := range s {
		for label, ds := range l.Trans[id] {
			if label != Tau && len(ds) > 0 {
				out[label] = true
			}
		}
	}
	return out
}

// acceptances are the initials of the stable states of s: a stable state
// refuses exactly what it cannot do.
func (l *LTS) acceptances(s set) []map[string]bool {
	var out []map[string]bool
	for id := range s {
		if len(l.Trans[id][Tau]) > 0 {
			continue
		}
		out = append(out, l.visible(set{id: true}))
	}
	return out
}

func subset(a, b map[string]bool) bool {
	for x := range a {
		if !b[x] {
			return false
		}
	}
	return true
}

// coveredBy says every refusal of an acceptance in as is a refusal of one
// in bs: for each a in as some b in bs accepts no more than a does.
func coveredBy(as, bs []map[string]bool) bool {
	for _, a := range as {
		ok := false
		for _, b := range bs {
			if subset(b, a) {
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

// StableFailuresEqual says a and b have the same traces and the same stable
// failures.
func StableFailuresEqual(a, b *LTS) bool {
	type pair struct{ a, b set }
	start := pair{a.tauClosure(set{a.Init: true}), b.tauClosure(set{b.Init: true})}
	seen := map[string]bool{}
	queue := []pair{start}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		k := p.a.key() + "|" + p.b.key()
		if seen[k] {
			continue
		}
		seen[k] = true
		va, vb := a.visible(p.a), b.visible(p.b)
		if !subset(va, vb) || !subset(vb, va) {
			return false
		}
		aa, ab := a.acceptances(p.a), b.acceptances(p.b)
		if !coveredBy(aa, ab) || !coveredBy(ab, aa) {
			return false
		}
		for label := range va {
			queue = append(queue, pair{a.after(p.a, label), b.after(p.b, label)})
		}
	}
	return true
}
