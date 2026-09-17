package csdftracechkcmd

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

func run(t *testing.T, args ...string) (int, *cli.ProcInoutSpy) {
	t.Helper()
	spy := cli.SpyProcInout()
	exitStatus := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())(args, spy.New())
	return exitStatus, spy
}

func TestNewMainFuncAcceptedTraceExitsZeroWithTheObligation(t *testing.T) {
	// Act
	exitStatus, spy := run(t, filepath.Join("testdata", "a.puml"), filepath.Join("testdata", "ok.tsv"))

	// Assert
	if exitStatus != 0 {
		t.Fatalf("want exit 0, got %d (stderr: %s)", exitStatus, spy.Stderr.String())
	}
	out := spy.Stdout.String()
	for _, want := range []string{
		"# " + filepath.Join("testdata", "ok.tsv") + ": ACCEPTED",
		"coins' is {coin}",
		"∃ x0 x1 x2. post_1(x0, x1)",
		"## Prompt",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q\n%s", want, out)
		}
	}
}

func TestNewMainFuncRejectedTraceExitsOneAndSaysWhere(t *testing.T) {
	// Act
	exitStatus, spy := run(t, filepath.Join("testdata", "a.puml"), filepath.Join("testdata", "ng.tsv"))

	// Assert
	if exitStatus != 1 {
		t.Fatalf("want exit 1, got %d", exitStatus)
	}
	out := spy.Stdout.String()
	for _, want := range []string{
		": REJECTED",
		"Event 1 of 1, `reset` (row 2 of",
		"visible events enabled there: `insert(coin)`",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stdout missing %q\n%s", want, out)
		}
	}
}

func TestNewMainFuncReportsEveryTraceAndFailsIfAnyIsRejected(t *testing.T) {
	// Act
	exitStatus, spy := run(t, filepath.Join("testdata", "a.puml"), filepath.Join("testdata", "ok.tsv"), filepath.Join("testdata", "ng.tsv"))

	// Assert
	if exitStatus != 1 {
		t.Fatalf("want exit 1, got %d", exitStatus)
	}
	out := spy.Stdout.String()
	if !strings.Contains(out, ": ACCEPTED") || !strings.Contains(out, ": REJECTED") {
		t.Errorf("want both reports\n%s", out)
	}
}

func TestNewMainFuncPrefixMatchAcceptsAStrippedTrace(t *testing.T) {
	// Act: stripped.tsv says "insert", not "insert(coin)".
	exitStatus, spy := run(t, "-match", "prefix", filepath.Join("testdata", "a.puml"), filepath.Join("testdata", "stripped.tsv"))

	// Assert
	if exitStatus != 0 {
		t.Fatalf("want exit 0, got %d (stdout: %s)", exitStatus, spy.Stdout.String())
	}
}

func TestNewMainFuncExactMatchRejectsAStrippedTrace(t *testing.T) {
	// Act
	exitStatus, _ := run(t, filepath.Join("testdata", "a.puml"), filepath.Join("testdata", "stripped.tsv"))

	// Assert
	if exitStatus != 1 {
		t.Fatalf("want exit 1, got %d", exitStatus)
	}
}

func TestNewMainFuncMalformedTraceIsAnError(t *testing.T) {
	// Act: a diagram is not a trace TSV.
	exitStatus, spy := run(t, filepath.Join("testdata", "a.puml"), filepath.Join("testdata", "a.puml"))

	// Assert
	if exitStatus != 1 {
		t.Fatalf("want exit 1, got %d", exitStatus)
	}
	if !strings.Contains(spy.Stderr.String(), "a.puml") {
		t.Errorf("want the file named in stderr, got %q", spy.Stderr.String())
	}
}

func TestNewMainFuncVersion(t *testing.T) {
	// Act
	exitStatus, spy := run(t, "-v")

	// Assert
	if exitStatus != 0 {
		t.Errorf("want 0, got %d", exitStatus)
	}
	if diff := cmp.Diff(version.Version+"\n", spy.Stdout.String()); diff != "" {
		t.Error(diff)
	}
}
