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

// Set is a set of states.
type Set map[State]bool

// key writes s canonically so that it can be a map key.
func (s Set) key() string {
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

// TauClosure is s with every state τ steps reach from it.
func (l *LTS) TauClosure(s Set) Set {
	out := Set{}
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

// After is the states l can be in after label from s, τ steps included:
// the τ closure of label's successors.
func (l *LTS) After(s Set, label string) Set {
	next := Set{}
	for id := range s {
		for _, d := range l.Trans[id][label] {
			next[d] = true
		}
	}
	return l.TauClosure(next)
}

// Initials are the visible labels some state of s can take.
func (l *LTS) Initials(s Set) map[string]bool {
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

// Acceptances are the initials of the stable states of s: a stable state
// refuses exactly what it cannot do.
func (l *LTS) Acceptances(s Set) []map[string]bool {
	var out []map[string]bool
	for id := range s {
		if len(l.Trans[id][Tau]) > 0 {
			continue
		}
		out = append(out, l.Initials(Set{id: true}))
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
	type pair struct{ a, b Set }
	start := pair{a.TauClosure(Set{a.Init: true}), b.TauClosure(Set{b.Init: true})}
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
		va, vb := a.Initials(p.a), b.Initials(p.b)
		if !subset(va, vb) || !subset(vb, va) {
			return false
		}
		aa, ab := a.Acceptances(p.a), b.Acceptances(p.b)
		if !coveredBy(aa, ab) || !coveredBy(ab, aa) {
			return false
		}
		for label := range va {
			queue = append(queue, pair{a.After(p.a, label), b.After(p.b, label)})
		}
	}
	return true
}

// TracesRefines says spec ⊑T impl: every trace of impl is a trace of spec.
func TracesRefines(spec, impl *LTS) bool { return refines(spec, impl, false) }

// StableFailuresRefines says spec ⊑SF impl: every trace and every stable
// failure of impl is one of spec.
func StableFailuresRefines(spec, impl *LTS) bool { return refines(spec, impl, true) }

// refines walks the traces of impl beside spec, as StableFailuresEqual
// walks both, checking inclusion one way only.
func refines(spec, impl *LTS, failures bool) bool {
	type pair struct{ s, i Set }
	queue := []pair{{spec.TauClosure(Set{spec.Init: true}), impl.TauClosure(Set{impl.Init: true})}}
	seen := map[string]bool{}
	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		k := p.s.key() + "|" + p.i.key()
		if seen[k] {
			continue
		}
		seen[k] = true
		vs, vi := spec.Initials(p.s), impl.Initials(p.i)
		if !subset(vi, vs) {
			return false
		}
		if failures && !coveredBy(impl.Acceptances(p.i), spec.Acceptances(p.s)) {
			return false
		}
		for label := range vi {
			queue = append(queue, pair{spec.After(p.s, label), impl.After(p.i, label)})
		}
	}
	return true
}
