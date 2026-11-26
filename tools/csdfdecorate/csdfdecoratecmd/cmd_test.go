package csdfdecoratecmd

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v5/cli"
	"github.com/Kuniwak/puml-parallel/v5/tools"
)

const diagram = `@startuml
state "idle" as idle
idle : n ; N
[*] --> idle : $n' == 0
idle --> idle : tick ; ; @eq(n', n)
@enduml
`

func TestMainPrintsTheDiagramDecoratedInTheLanguageGiven(t *testing.T) {
	// Setup
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(diagram))

	// Execute
	exitStatus := cmdFunc([]string{"-lang", "ja"}, spy.New())

	// Assert
	if exitStatus != 0 {
		t.Fatalf("exit status = %d, stderr = %s", exitStatus, spy.Stderr.String())
	}
	if want := `idle --> idle : <color:#888>イベント:</color> <b>tick</b>\l<color:#888>維持:</color> <color:#0b57d0>n</color>\l`; !strings.Contains(spy.Stdout.String(), want) {
		t.Errorf("stdout = %q, want it to hold %q", spy.Stdout.String(), want)
	}
}

func TestMainRefusesALanguageItHasNoWordsFor(t *testing.T) {
	// Setup
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	spy.Stdin = cli.StubStdin(strings.NewReader(diagram))

	// Execute
	exitStatus := cmdFunc([]string{"-lang", "fr"}, spy.New())

	// Assert
	if exitStatus == 0 || !strings.Contains(spy.Stderr.String(), "en or ja") {
		t.Errorf("exit status = %d, stderr = %q, want a failure saying the languages", exitStatus, spy.Stderr.String())
	}
}
