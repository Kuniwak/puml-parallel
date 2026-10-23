package isabelle

import "testing"

func TestRootShouldExtendTheParentWithTheLibrarySession(t *testing.T) {
	// Execute
	got := Root("Csdf_Obligation", "CSP_F", "Refinement_Obligation")

	// Assert
	want := "session Csdf_Obligation = CSP_F +\n  sessions\n    \"HOL-Library\"\n  theories\n    Refinement_Obligation\n"
	if got != want {
		t.Errorf("Root() = %q, want %q", got, want)
	}
}
