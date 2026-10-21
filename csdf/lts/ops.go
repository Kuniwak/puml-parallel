package lts

import (
	"fmt"
	"strconv"
	"strings"
)

// Hide is l \ C, where hidden says which labels are in C.
func Hide(l *LTS, hidden func(string) bool) *LTS {
	out := New(l.Init)
	for src, byLabel := range l.Trans {
		for label, dsts := range byLabel {
			if hidden(label) {
				label = Tau
			}
			for _, d := range dsts {
				out.Add(src, label, d)
			}
		}
	}
	return out
}

// Rename is l[[R]], where to gives the labels a label is renamed to; it may
// give several, a one-to-many renaming, and τ is never renamed. CSP's
// renaming is total, so a label to gives no image is an error.
func Rename(l *LTS, to func(string) []string) (*LTS, error) {
	out := New(l.Init)
	for src, byLabel := range l.Trans {
		for label, dsts := range byLabel {
			images := []string{Tau}
			if label != Tau {
				images = to(label)
			}
			if len(images) == 0 {
				return nil, fmt.Errorf("lts.Rename: %s has no image", label)
			}
			for _, image := range images {
				for _, d := range dsts {
					out.Add(src, image, d)
				}
			}
		}
	}
	return out, nil
}

// Parallel is the alphabetised parallel of ps, where alphabets[i] says which
// labels ps[i] takes part in: a label happens when every process whose
// alphabet holds it takes it together, and τ is each process's own.
func Parallel(ps []*LTS, alphabets []func(string) bool) *LTS {
	ids := map[string]State{}
	var states [][]State
	id := func(s []State) (State, bool) {
		k := stateKey(s)
		if i, ok := ids[k]; ok {
			return i, false
		}
		i := len(states)
		ids[k] = i
		states = append(states, append([]State{}, s...))
		return i, true
	}
	init := make([]State, len(ps))
	for i, p := range ps {
		init[i] = p.Init
	}
	start, _ := id(init)
	out := New(start)
	queue := []State{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		s := states[cur]
		add := func(label string, next []State) {
			n, fresh := id(next)
			out.Add(cur, label, n)
			if fresh {
				queue = append(queue, n)
			}
		}
		labels := map[string]bool{}
		for i, p := range ps {
			for label, dsts := range p.Trans[s[i]] {
				if label == Tau {
					for _, d := range dsts {
						next := append([]State{}, s...)
						next[i] = d
						add(Tau, next)
					}
					continue
				}
				labels[label] = true
			}
		}
		for label := range labels {
			for _, next := range joint(ps, alphabets, s, label) {
				add(label, next)
			}
		}
	}
	return out
}

// joint are the states ps reach from s by taking label together.
func joint(ps []*LTS, alphabets []func(string) bool, s []State, label string) [][]State {
	nexts := [][]State{append([]State{}, s...)}
	any := false
	for i, p := range ps {
		if !alphabets[i](label) {
			continue
		}
		any = true
		dsts := p.Trans[s[i]][label]
		var grown [][]State
		for _, n := range nexts {
			for _, d := range dsts {
				m := append([]State{}, n...)
				m[i] = d
				grown = append(grown, m)
			}
		}
		nexts = grown
	}
	if !any {
		return nil
	}
	return nexts
}

func stateKey(s []State) string {
	parts := make([]string, len(s))
	for i, x := range s {
		parts[i] = strconv.Itoa(x)
	}
	return strings.Join(parts, ",")
}
