package animation

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/v6/csdf/clause"
	"github.com/google/go-cmp/cmp"
)

func post(t *testing.T, text string) clause.And {
	t.Helper()
	c, err := clause.Parse(text)
	if err != nil {
		t.Fatalf("Setup: clause.Parse(%q) error = %v", text, err)
	}
	return c
}

func TestDeterminedShouldGiveAnEmptyMapOfEmpty(t *testing.T) {
	// Setup
	p := Pending{Target: Target{ID: "G", Vars: []string{"members"}}, Post: post(t, "@empty(members')")}

	// Execute
	got := Determined(p)

	// Assert
	want := []Value{{Name: "members", Value: map[string]any{}}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Determined() mismatch (-want +got):\n%s", diff)
	}
}

func TestDeterminedShouldKeepAnUnchangedVariable(t *testing.T) {
	// Setup
	prev := &State{ID: "A", Values: []Value{{Name: "x", Value: jsonNumber("3")}, {Name: "y", Value: "s"}}}
	p := Pending{Target: Target{ID: "B", Vars: []string{"x", "y"}}, Prev: prev, Post: post(t, "@eq(x', x) ∧ @eq(y, y')")}

	// Execute
	got := Determined(p)

	// Assert
	want := []Value{{Name: "x", Value: jsonNumber("3")}, {Name: "y", Value: "s"}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Determined() mismatch (-want +got):\n%s", diff)
	}
}

func TestDeterminedShouldGiveALiteral(t *testing.T) {
	// Setup
	p := Pending{Target: Target{ID: "B", Vars: []string{"x"}}, Post: post(t, `@eq(x', "on")`)}

	// Execute
	got := Determined(p)

	// Assert
	want := []Value{{Name: "x", Value: "on"}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Determined() mismatch (-want +got):\n%s", diff)
	}
}

func TestDeterminedShouldLeaveOutWhatOnlyAGlueDecides(t *testing.T) {
	// Setup
	p := Pending{Target: Target{ID: "B", Vars: []string{"x", "y"}}, Post: post(t, "@empty(y') ∧ @ne(x', 0)")}

	// Execute
	got := Determined(p)

	// Assert
	want := []Value{{Name: "y", Value: map[string]any{}}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Determined() mismatch (-want +got):\n%s", diff)
	}
}

func TestDeterminedShouldLeaveOutAVariableTwoConjunctsDisagreeOn(t *testing.T) {
	// Setup
	p := Pending{Target: Target{ID: "B", Vars: []string{"x"}}, Post: post(t, "@eq(x', 1) ∧ @eq(x', 2)")}

	// Execute
	got := Determined(p)

	// Assert
	if len(got) != 0 {
		t.Errorf("Determined() = %v, want none", got)
	}
}
