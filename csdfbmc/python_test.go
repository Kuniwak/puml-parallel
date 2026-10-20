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

// The first worked example reaches a state with no way out, and the report
// names it with its trace.
func TestTheWithdrawExampleReportsADeadlock(t *testing.T) {
	cmd := exec.Command(python(t), "-E", "-s", "-B", "-m", "csdfbmc", "examples/bmc/withdraw.json", "examples/bmc/withdraw_twins.py")
	cmd.Dir = ".."
	out, err := cmd.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		t.Fatalf("want exit status 1 (a deadlock found), got %v\n%s", err, out)
	}
	for _, want := range []string{"deadlock at sending [3, 3]", "出庫(数量=3)"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("want %q in the report, got:\n%s", want, out)
		}
	}
}

// The toy ledger checks six properties at once; as written it reports a
// deadlock after the retries and nothing else.
func TestTheWalletExampleReportsADeadlockAfterTheRetries(t *testing.T) {
	cmd := exec.Command(python(t), "-E", "-s", "-B", "-m", "csdfbmc", "examples/bmc/wallet.json", "examples/bmc/wallet_twins.py", "--avoid", "確定", "--within", "sending")
	cmd.Dir = ".."
	out, err := cmd.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		t.Fatalf("want exit status 1, got %v\n%.2000s", err, out)
	}
	text := string(out)
	for _, want := range []string{"deadlock at sending", "'再送', '再送'"} {
		if !strings.Contains(text, want) {
			t.Errorf("want %q in the report, got:\n%.2000s", want, text)
		}
	}
	for _, unwanted := range []string{"invariant", "cycle", "domain too small"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("want no %q in the report, got:\n%.2000s", unwanted, text)
		}
	}
}
