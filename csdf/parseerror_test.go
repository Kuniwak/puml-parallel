package csdf_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v4/csdf"
)

func TestParseReportsWhereTheSyntaxErrorIs(t *testing.T) {
	// Arrange: a line the core grammar has no rule for, on the third line.
	source := "@startuml\nstate \"s0\" as s0\nbogus\n@enduml\n"

	// Act
	_, err := csdf.Parse("", source)

	// Assert
	var parseErr *csdf.ParseError
	if !errors.As(err, &parseErr) {
		t.Fatalf("want a *csdf.ParseError, got %v", err)
	}
	if parseErr.Line != 3 {
		t.Errorf("want line 3, got %d", parseErr.Line)
	}
}

func TestParseNamesTheDirectiveOfAMalformedGuard(t *testing.T) {
	cases := map[string]struct{ line, want string }{
		"a constrain": {"constrain ORDER?s ; @ne(s)", "constrain ORDER?s: "},
		"a sync":      {"sync SHIP : m(s) ; @ne(s)", "sync SHIP: "},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange: a note whose one line is the directive.
			source := "@startuml\nnote as n\n  " + c.line + "\nend note\n@enduml\n"

			// Act
			_, err := csdf.Parse("", source)

			// Assert
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("got %v, want an error containing %q", err, c.want)
			}
		})
	}
}
