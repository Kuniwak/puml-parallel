package lts

import "testing"

func TestStableFailuresEqual(t *testing.T) {
	// a → (b → STOP ⊓ c → STOP), with ⊓ as two τ steps
	internalAfter := func() *LTS {
		l := New(0)
		l.Add(0, "a", 1)
		l.Add(1, Tau, 2)
		l.Add(1, Tau, 3)
		l.Add(2, "b", 4)
		l.Add(3, "c", 5)
		return l
	}
	// (a → b → STOP) ⊓ (a → c → STOP)
	internalBefore := func() *LTS {
		l := New(0)
		l.Add(0, Tau, 1)
		l.Add(0, Tau, 2)
		l.Add(1, "a", 3)
		l.Add(2, "a", 4)
		l.Add(3, "b", 5)
		l.Add(4, "c", 6)
		return l
	}
	// a → (b → STOP □ c → STOP)
	external := func() *LTS {
		l := New(0)
		l.Add(0, "a", 1)
		l.Add(1, "b", 2)
		l.Add(1, "c", 3)
		return l
	}
	// a → b → STOP
	onlyB := func() *LTS {
		l := New(0)
		l.Add(0, "a", 1)
		l.Add(1, "b", 2)
		return l
	}
	// a → b → STOP, where the state after a also has a τ self-loop: it is
	// never stable, so after a there is no stable failure
	diverging := func() *LTS {
		l := New(0)
		l.Add(0, "a", 1)
		l.Add(1, Tau, 1)
		l.Add(1, "b", 2)
		return l
	}
	cases := []struct {
		name string
		a, b *LTS
		want bool
	}{
		{"the law a → (P ⊓ Q) = (a → P) ⊓ (a → Q)", internalAfter(), internalBefore(), true},
		{"internal and external choice differ in failures", internalAfter(), external(), false},
		{"different traces differ", external(), onlyB(), false},
		{"a state that only diverges has no stable failure", diverging(), onlyB(), false},
		{"a process equals itself", external(), external(), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Execute
			got := StableFailuresEqual(c.a, c.b)

			// Assert
			if got != c.want {
				t.Errorf("StableFailuresEqual = %v, want %v", got, c.want)
			}
		})
	}
}

func TestRefines(t *testing.T) {
	// a → STOP □ b → STOP
	both := func() *LTS {
		l := New(0)
		l.Add(0, "a", 1)
		l.Add(0, "b", 2)
		return l
	}
	// a → STOP ⊓ b → STOP
	either := func() *LTS {
		l := New(0)
		l.Add(0, Tau, 1)
		l.Add(0, Tau, 2)
		l.Add(1, "a", 3)
		l.Add(2, "b", 4)
		return l
	}
	// a → STOP
	onlyA := func() *LTS {
		l := New(0)
		l.Add(0, "a", 1)
		return l
	}
	// STOP
	stop := func() *LTS { return New(0) }
	cases := []struct {
		name         string
		spec, impl   *LTS
		traces, fail bool
	}{
		{"a process refines itself", both(), both(), true, true},
		{"resolving an internal choice refines it", either(), onlyA(), true, true},
		{"an external choice refines the internal one", either(), both(), true, true},
		{"the internal choice refines the external one in traces only", both(), either(), true, false},
		{"fewer traces refine in traces, and refuse more", both(), onlyA(), true, false},
		{"more traces refine in neither", onlyA(), both(), false, false},
		{"STOP refines every process in traces", both(), stop(), true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Execute
			traces, fail := TracesRefines(c.spec, c.impl), StableFailuresRefines(c.spec, c.impl)

			// Assert
			if traces != c.traces || fail != c.fail {
				t.Errorf("TracesRefines = %v, StableFailuresRefines = %v, want %v, %v", traces, fail, c.traces, c.fail)
			}
		})
	}
}
