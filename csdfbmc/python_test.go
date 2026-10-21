package csdfbmc_test

import (
	"os/exec"
	"strings"
	"testing"
)

// python finds the interpreter. The commands run with -E -s so that neither
// the environment nor the user site can change what the tests import, while
// the working directory stays on the path, which is how -m finds the package;
// -B keeps bytecode out of the tree, where a release archive would pick it up.
func python(t *testing.T) string {
	t.Helper()
	path, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not on the PATH")
	}
	return path
}

func TestPythonUnitTestsPass(t *testing.T) {
	cmd := exec.Command(python(t), "-E", "-s", "-B", "-m", "unittest", "discover", "-s", "tests", "-t", "..")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("python unit tests failed: %v\n%s", err, out)
	}
}

// The toy ledger checks six properties at once; as written it reports a
// deadlock after the retries and nothing else.
func TestTheWalletExampleReportsADeadlockAfterTheRetries(t *testing.T) {
	cmd := exec.Command(python(t), "-E", "-s", "-B", "-m", "csdfbmc", "examples/bmc/wallet_formal.json", "examples/bmc/wallet_formal_checks.py",
		"--evaluator-module", "csdfbmc.python_glue examples/bmc/wallet_formal_checks.py", "--avoid", "確定", "--within", "sending")
	cmd.Dir = ".."
	out, err := cmd.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		t.Fatalf("want exit status 1, got %v\n%.2000s", err, out)
	}
	text := string(out)
	for _, want := range []string{"deadlock ×", "at sending after 再送", "'再送', '再送'"} {
		if !strings.Contains(text, want) {
			t.Errorf("want %q in the report, got:\n%.2000s", want, text)
		}
	}
	for _, unwanted := range []string{"invariant", "cycle", "post leaves", "post admits"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("want no %q in the report, got:\n%.2000s", unwanted, text)
		}
	}
}

// The first worked example reaches a state with no way out, read through the
// reference glue as a command or in-process, and the report names it with
// its trace.
func TestTheWithdrawExampleReportsADeadlockThroughTheGlue(t *testing.T) {
	py := python(t)
	for _, glue := range [][]string{
		{"--evaluator", py + " -E -s -B -m csdfbmc.python_glue examples/bmc/withdraw_formal_defs.py"},
		{"--evaluator-module", "csdfbmc.python_glue examples/bmc/withdraw_formal_defs.py"},
	} {
		t.Run(glue[0], func(t *testing.T) {
			cmd := exec.Command(py, append([]string{"-E", "-s", "-B", "-m", "csdfbmc", "examples/bmc/withdraw_formal.json", "examples/bmc/withdraw_formal_defs.py"}, glue...)...)
			cmd.Dir = ".."
			out, err := cmd.CombinedOutput()
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
				t.Fatalf("want exit status 1 (a deadlock found), got %v\n%s", err, out)
			}
			if want := "states: 22\ndeadlock ×3 at sending after 出庫; first after ['出庫(数量=3)']\n"; string(out) != want {
				t.Errorf("want the deadlock and its trace, got:\n%s", out)
			}
		})
	}
}

// The expansion of a promoted diagram is walked through the reference glue,
// local clauses and all; the fee-bumping example comes out clean.
func TestThePromotedExampleWalksClean(t *testing.T) {
	cmd := exec.Command(python(t), "-E", "-s", "-B", "-m", "csdfbmc", "examples/bmc/attempts/attempts.json", "examples/bmc/attempts/checks.py",
		"--evaluator-module", "csdfbmc.python_glue", "--avoid", "CONFIRMED")
	cmd.Dir = ".."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("want exit status 0 (nothing found), got %v\n%s", err, out)
	}
	if string(out) != "states: 496\n" {
		t.Errorf("want only the state count, got:\n%s", out)
	}
}

// The attempts are declared SYMMETRIC, and the glue says no clause tells
// their IDs apart, so --symmetry walks one state per renaming of their IDs
// and still comes out clean.
func TestThePromotedExampleWalksCleanUnderSymmetry(t *testing.T) {
	cmd := exec.Command(python(t), "-E", "-s", "-B", "-m", "csdfbmc", "examples/bmc/attempts/attempts.json", "examples/bmc/attempts/checks.py",
		"--evaluator-module", "csdfbmc.python_glue", "--avoid", "CONFIRMED", "--symmetry", "--por")
	cmd.Dir = ".."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("want exit status 0 (nothing found), got %v\n%s", err, out)
	}
	if string(out) != "states: 104\n" {
		t.Errorf("want the 104 representatives of the 496 states, got:\n%s", out)
	}
}

// A sync moves two instances of the family of locations at once, and the
// stock the move takes out of one is what it puts into the other, so the
// conservation invariant holds. Partial-order reduction does not take such a
// move for one of a single instance, so it walks every state.
func TestTheExampleThatMovesTwoInstancesWalksClean(t *testing.T) {
	cases := map[string]struct{ extra []string }{
		"in full":                      {extra: nil},
		"with partial-order reduction": {extra: []string{"--por"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Setup
			args := append([]string{"-E", "-s", "-B", "-m", "csdfbmc", "examples/bmc/move/move.json", "examples/bmc/move/checks.py",
				"--evaluator-module", "csdfbmc.python_glue"}, c.extra...)
			cmd := exec.Command(python(t), args...)
			cmd.Dir = ".."

			// Execute
			out, err := cmd.CombinedOutput()

			// Assert
			if err != nil {
				t.Fatalf("want exit status 0 (nothing found), got %v\n%s", err, out)
			}
			if string(out) != "states: 68\n" {
				t.Errorf("want only the state count, got:\n%s", out)
			}
		})
	}
}
