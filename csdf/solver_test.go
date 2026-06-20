package csdf

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/core"
)

func TestSolveJSON(t *testing.T) {
	// Arrange
	group := core.State{
		ID:   "s0",
		Name: "Initial",
		Vars: []core.StateVar{{Name: "a"}, {Name: "b"}},
	}

	// Act
	result := SolveJSON(PostSolverInput{
		StateGroup:    group,
		EncodedValues: `[1, {"nested": ["ok"]}]`,
	})

	// Assert
	if result.Kind != PostSolverResultOK {
		t.Fatalf("SolveJSON() kind = %v, want OK; err = %v", result.Kind, result.Err)
	}
	if len(result.State.Values) != 2 || result.State.Values[0].Name != "a" || result.State.Values[1].Name != "b" {
		t.Errorf("SolveJSON() state values = %#v", result.State.Values)
	}
}
