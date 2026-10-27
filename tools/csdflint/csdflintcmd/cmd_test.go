package csdflintcmd_test

import (
	"encoding/csv"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/Kuniwak/puml-parallel/v3/tools"
	"github.com/Kuniwak/puml-parallel/v3/tools/csdflint/csdflintcmd"
	"github.com/google/go-cmp/cmp"
)

func run(args ...string) (int, *cli.ProcInoutSpy) {
	cmdFunc := tools.NewCommandFunc(csdflintcmd.NewParseOptionsFunc(), csdflintcmd.NewMainFunc())
	spy := cli.SpyProcInout()
	return cmdFunc(args, spy.New()), spy
}

func TestNewMainFuncWritesOnlyTheHeaderForACleanDiagram(t *testing.T) {
	// Arrange & Act
	exitStatus, spy := run(filepath.Join("testdata", "clean.puml"))

	// Assert
	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Errorf("want 0, got %d", exitStatus)
	}
	want := "file\tstart_line\tend_line\trule\tseverity\tmessage\n"
	if diff := cmp.Diff(want, spy.Stdout.String()); diff != "" {
		t.Error(diff)
	}
}

func TestNewMainFuncReportsBothSelfTransitionFindingsAndSucceeds(t *testing.T) {
	// Arrange: the tau self-transition has both a guard and a post, so both
	// rules fire.
	path := filepath.Join("testdata", "selfloop.puml")

	// Act
	exitStatus, spy := run(path)

	// Assert: neither a warning nor a hint is a failure.
	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Errorf("want 0, got %d", exitStatus)
	}
	r := csv.NewReader(strings.NewReader(spy.Stdout.String()))
	r.Comma = '\t'
	r.LazyQuotes = true
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	var got [][]string
	for _, row := range rows {
		got = append(got, row[:5])
	}
	want := [][]string{
		{"file", "start_line", "end_line", "rule", "severity"},
		{path, "5", "5", "self-transition-divergence", "WARN"},
		{path, "5", "5", "self-transition-post-breaks-guard", "HINT"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Error(diff)
	}
}

func TestNewMainFuncFailsOnAnErrorButStillWritesTheFinding(t *testing.T) {
	// Arrange & Act
	exitStatus, spy := run(filepath.Join("testdata", "syntaxerror.puml"))

	// Assert
	if exitStatus != 1 {
		t.Errorf("want 1, got %d", exitStatus)
	}
	out := spy.Stdout.String()
	if !strings.Contains(out, "\tERROR\t") || !strings.Contains(out, "syntax-error") {
		t.Errorf("want the ERROR finding, got %q", out)
	}
	if !strings.Contains(spy.Stderr.String(), "Error:") {
		t.Errorf("want the failure said on stderr, got %q", spy.Stderr.String())
	}
}

func TestNewMainFuncRefusesAnOldClauseAsASyntaxError(t *testing.T) {
	// Arrange
	path := filepath.Join("testdata", "oldsyntax.puml")

	// Act
	exitStatus, spy := run(path)

	// Assert
	if exitStatus != 1 {
		t.Errorf("want 1, got %d", exitStatus)
	}
	want := "file\tstart_line\tend_line\trule\tseverity\tmessage\n" +
		path + "\t4\t4\tsyntax-error\tERROR\tThe parser stopped here: unknown tool word @opaque at 0. Fix the syntax before reading any other finding: every other rule is skipped while the file does not parse. The grammar is in docs/SYNTAX.md.\n"
	if diff := cmp.Diff(want, spy.Stdout.String()); diff != "" {
		t.Error(diff)
	}
}

func TestNewMainFuncLintsEveryFileGiven(t *testing.T) {
	// Arrange
	clean := filepath.Join("testdata", "clean.puml")
	selfLoop := filepath.Join("testdata", "selfloop.puml")

	// Act
	exitStatus, spy := run(clean, selfLoop)

	// Assert
	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Errorf("want 0, got %d", exitStatus)
	}
	if got := strings.Count(spy.Stdout.String(), selfLoop+"\t"); got != 2 {
		t.Errorf("want 2 findings of %s, got %d", selfLoop, got)
	}
}

func TestNewMainFuncNamesStandardInputAsDash(t *testing.T) {
	// Arrange
	cmdFunc := tools.NewCommandFunc(csdflintcmd.NewParseOptionsFunc(), csdflintcmd.NewMainFunc())
	spy := cli.SpyProcInout()
	spy.Stdin = strings.NewReader("@startuml\nstate \"s0\" as s0\n[*] --> s0\ns0 --> s0 : tau\n@enduml\n")

	// Act
	exitStatus := cmdFunc(nil, spy.New())

	// Assert
	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Errorf("want 0, got %d", exitStatus)
	}
	if !strings.Contains(spy.Stdout.String(), "-\t4\t4\tself-transition-divergence\tWARN\t") {
		t.Errorf("want a finding about standard input, got %q", spy.Stdout.String())
	}
}

func TestNewMainFuncListsTheRules(t *testing.T) {
	// Arrange & Act
	exitStatus, spy := run("-list-rules")

	// Assert
	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Errorf("want 0, got %d", exitStatus)
	}
	for _, want := range []string{"syntax-error: ", "self-transition-divergence: ", "self-transition-post-breaks-guard: "} {
		if !strings.Contains(spy.Stdout.String(), want) {
			t.Errorf("want %q, got %q", want, spy.Stdout.String())
		}
	}
}
