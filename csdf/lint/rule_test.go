package lint_test

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/lint"
)

func TestRunAllKeepsTheInputsInTheOrderTheyWereGiven(t *testing.T) {
	// Arrange: two files that each trip one rule.
	first := lint.NewInput("first.puml", []byte("@startuml\nstate \"s0\" as s0\n[*] --> s0\ns0 --> s0 : a\n@enduml\n"))
	second := lint.NewInput("second.puml", []byte("@startuml\nbogus\n@enduml\n"))

	// Act
	findings := lint.RunAll(lint.DefaultRules(), []*lint.Input{first, second})

	// Assert
	var files []string
	for _, f := range findings {
		files = append(files, f.File)
	}
	if got := strings.Join(files, ","); got != "first.puml,second.puml" {
		t.Errorf("want first.puml,second.puml, got %s", got)
	}
}
