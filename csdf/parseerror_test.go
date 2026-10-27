package csdf_test

import (
	"errors"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/csdf"
)

func TestParseReportsWhereTheSyntaxErrorIs(t *testing.T) {
	// Arrange: a line the core grammar has no rule for, on the third line.
	source := "@startuml\nstate \"s0\" as s0\nbogus\n@enduml\n"

	// Act
	_, err := csdf.ParseSource(source, func(csdf.Predicate) error { return nil })

	// Assert
	var parseErr *csdf.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("want a *csdf.ParseError, got %v", err)
	}
	if parseErr.Line != 3 {
		t.Errorf("want line 3, got %d", parseErr.Line)
	}
}
