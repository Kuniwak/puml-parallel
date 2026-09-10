package csdflintcmd_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/tools/csdflint/csdflintcmd"
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

func TestNewMainFuncReportsBothSelfTransitionHintsAndSucceeds(t *testing.T) {
	// Arrange: the self-transition has both a guard and a post, so both hints fire.
	path := filepath.Join("testdata", "selfloop.puml")

	// Act
	exitStatus, spy := run(path)

	// Assert: a hint is not a failure.
	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Errorf("want 0, got %d", exitStatus)
	}
	lines := strings.Split(strings.TrimSuffix(spy.Stdout.String(), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("want a header and 2 findings, got %d lines: %q", len(lines), spy.Stdout.String())
	}
	for _, line := range lines[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) != 6 {
			t.Fatalf("want 6 fields, got %d: %q", len(fields), line)
		}
		if fields[0] != path {
			t.Errorf("want %q, got %q", path, fields[0])
		}
		if fields[1] != "5" || fields[2] != "5" {
			t.Errorf("want lines 5-5, got %s-%s", fields[1], fields[2])
		}
		if fields[4] != "HINT" {
			t.Errorf("want HINT, got %s", fields[4])
		}
	}
	if got := strings.Split(lines[1], "\t")[3]; got != "self-transition-divergence" {
		t.Errorf("want self-transition-divergence first, got %s", got)
	}
	if got := strings.Split(lines[2], "\t")[3]; got != "self-transition-post-breaks-guard" {
		t.Errorf("want self-transition-post-breaks-guard second, got %s", got)
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
	spy.Stdin = strings.NewReader("@startuml\nstate \"s0\" as s0\n[*] --> s0\ns0 --> s0 : a\n@enduml\n")

	// Act
	exitStatus := cmdFunc(nil, spy.New())

	// Assert
	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Errorf("want 0, got %d", exitStatus)
	}
	if !strings.Contains(spy.Stdout.String(), "-\t4\t4\tself-transition-divergence\tHINT\t") {
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
