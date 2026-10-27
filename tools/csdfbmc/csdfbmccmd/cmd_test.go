package csdfbmccmd

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/Kuniwak/puml-parallel/v3/tools"
	"github.com/Kuniwak/puml-parallel/v3/version"
)

func run(t *testing.T, env map[string]string, args ...string) (int, *cli.ProcInoutSpy) {
	t.Helper()
	spy := cli.SpyProcInout()
	spy.Env = env
	status := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())(args, spy.New())
	return status, spy
}

func needPython(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is not on the PATH")
	}
}

func TestVersionAndHelpAreTheLaunchersOwn(t *testing.T) {
	status, spy := run(t, nil, "-v")
	if status != 0 || spy.Stdout.String() != version.Version+"\n" {
		t.Errorf("-v: status %d, stdout %q", status, spy.Stdout.String())
	}
	status, spy = run(t, nil, "-h")
	if status != 0 || !strings.Contains(spy.Stderr.String(), "Usage: csdfbmc") {
		t.Errorf("-h: status %d, stderr %q", status, spy.Stderr.String())
	}
}

func TestEverythingAfterTheLaunchersOptionsGoesToTheChecker(t *testing.T) {
	needPython(t)
	cache := t.TempDir()
	status, spy := run(t, map[string]string{"CSDFBMC_CACHE": cache},
		"../../../examples/bmc/withdraw_formal.json", "../../../examples/bmc/withdraw_formal_defs.py",
		"--evaluator-module", "csdfbmc.python_glue ../../../examples/bmc/withdraw_formal_defs.py", "--json")
	if status != 1 {
		t.Fatalf("status = %d, want 1 (a deadlock found)\nstderr: %s", status, spy.Stderr.String())
	}
	if out := spy.Stdout.String(); !strings.Contains(out, `"deadlocks"`) {
		t.Errorf("--json was not passed through; stdout: %s", out)
	}
	if spy.Stderr.Len() != 0 {
		t.Errorf("want nothing on stderr, got %q", spy.Stderr.String())
	}
}

func TestTheCheckersUsageErrorIsItsExitStatus(t *testing.T) {
	needPython(t)
	status, spy := run(t, map[string]string{"CSDFBMC_CACHE": t.TempDir()}, "no-such-diagram.json", "no-such-checks.py")
	if status != 2 || !strings.Contains(spy.Stderr.String(), "csdfbmc:") {
		t.Errorf("status = %d, stderr %q; want 2 and the checker's own message", status, spy.Stderr.String())
	}
}

func TestAMissingInterpreterIsReported(t *testing.T) {
	status, spy := run(t, map[string]string{"CSDFBMC_CACHE": t.TempDir()}, "-python", "/no/such/python3", "a.json", "b.py")
	if status != 2 || !strings.Contains(spy.Stderr.String(), "/no/such/python3") {
		t.Errorf("status = %d, stderr %q; want 2 naming the interpreter", status, spy.Stderr.String())
	}
}

func TestNextRunsTheValuePromptModule(t *testing.T) {
	needPython(t)
	status, spy := run(t, map[string]string{"CSDFBMC_CACHE": t.TempDir()},
		"next", "../../../examples/bmc/withdraw_formal.json", "../../../examples/bmc/withdraw_formal_defs.py",
		"--evaluator-module", "csdfbmc.python_glue ../../../examples/bmc/withdraw_formal_defs.py", "--initial")
	if status != 0 {
		t.Fatalf("status = %d, stderr %s", status, spy.Stderr.String())
	}
	if out := spy.Stdout.String(); !strings.Contains(out, `"candidates"`) || !strings.Contains(out, `"values": [3]`) {
		t.Errorf("next --initial printed %q", out)
	}
}
