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
