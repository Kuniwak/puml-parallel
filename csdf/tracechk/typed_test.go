package tracechk

import (
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/csdf/logic"
	"github.com/google/go-cmp/cmp"
)

// TestTheValuesOfAPathAreOfTheTypesNamedLikeThem holds the quantifiers of a
// report to the convention of obligation.Enabled: a value is of the type named
// like its variable or field.
func TestTheValuesOfAPathAreOfTheTypesNamedLikeThem(t *testing.T) {
	// Setup
	d, err := coreio.Read("@startuml\nstate \"a\" as a\na : n\n[*] --> a : $n' = 0\na --> a : in?k ; ; $n' = $n + $k\n@enduml\n")
	if err != nil {
		t.Fatal(err)
	}
	along, err := pathClauses(d, Path{d.Edges[0]})
	if err != nil {
		t.Fatal(err)
	}

	// Execute
	tree := logic.TreeOf(some(along))

	// Assert
	if diff := cmp.Diff([]logic.Var{"n@0", "n@1", "k#1"}, tree.Vars); diff != "" {
		t.Error(diff)
	}
	if diff := cmp.Diff([]string{"n", "n", "k"}, tree.Types); diff != "" {
		t.Error(diff)
	}
}
