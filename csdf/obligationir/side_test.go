package obligationir

import "testing"

func TestSideSingleKeepsUnqualifiedNames(t *testing.T) {
	// The single-diagram obligation must keep the spelling it had before sides
	// existed, or every livelock golden file changes.
	s := SideSingle
	if got := s.Ctor("a"); got != "a" {
		t.Errorf("Ctor = %q, want a", got)
	}
	if got := s.Qualify("step"); got != "step" {
		t.Errorf("Qualify = %q, want step", got)
	}
	if got := s.GuardName(5); got != "guard_L5" {
		t.Errorf("GuardName = %q, want guard_L5", got)
	}
	if got := s.PostName(5); got != "post_L5" {
		t.Errorf("PostName = %q, want post_L5", got)
	}
}

func TestSideSpecAndImplQualifyEveryLocationDerivedName(t *testing.T) {
	// Line numbers and state ids collide across the two input files, so both
	// sides qualify; the Spec is S and the Impl is I.
	for _, tc := range []struct {
		name  string
		side  Side
		ctor  string
		qual  string
		guard string
		post  string
	}{
		{"spec", SideSpec, "S_a", "step_S", "guard_S_L5", "post_S_L5"},
		{"impl", SideImpl, "I_a", "step_I", "guard_I_L5", "post_I_L5"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.side.Ctor("a"); got != tc.ctor {
				t.Errorf("Ctor = %q, want %q", got, tc.ctor)
			}
			if got := tc.side.Qualify("step"); got != tc.qual {
				t.Errorf("Qualify = %q, want %q", got, tc.qual)
			}
			if got := tc.side.GuardName(5); got != tc.guard {
				t.Errorf("GuardName = %q, want %q", got, tc.guard)
			}
			if got := tc.side.PostName(5); got != tc.post {
				t.Errorf("PostName = %q, want %q", got, tc.post)
			}
		})
	}
}
