package animation

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFrameOfShouldSeparateEmptiedAndUnchangedVariables(t *testing.T) {
	// Setup
	c := post(t, `@empty(a') ∧ @eq(b', b) ∧ @eq(c, c') ∧ @eq(d', "on") ∧ @eq(e', f)`)

	// Execute
	got := FrameOf(c)

	// Assert
	want := Frame{Rest: `@eq(d', "on") ∧ @eq(e', f)`, Emptied: []string{"a"}, Unchanged: []string{"b", "c"}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("FrameOf() mismatch (-want +got):\n%s", diff)
	}
}

func TestFrameOfShouldLeaveNoneWhenAllIsFrame(t *testing.T) {
	// Setup
	c := post(t, `@empty(a') ∧ @eq(b', b)`)

	// Execute
	got := FrameOf(c)

	// Assert
	want := Frame{Rest: "(none)", Emptied: []string{"a"}, Unchanged: []string{"b"}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("FrameOf() mismatch (-want +got):\n%s", diff)
	}
}
