package csdfbmc_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"github.com/Kuniwak/puml-parallel/v9/csdfbmc"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
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

// pythonCommand runs the interpreter with args, as PythonArgs gives them,
// watched: a run still going a minute before the test binary's deadline
// writes every thread's traceback to its standard error and exits, so a hang
// names where it waits instead of being killed silently.
func pythonCommand(t *testing.T, args []string) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(python(t), watched(secondsLeft(t), args)...)
	// A process python3 leaves behind holding its output, such as a pool
	// worker in a session of its own, would keep the output open, and
	// hide the watchdog's tracebacks with it, until the test binary's
	// deadline.
	cmd.WaitDelay = 10 * time.Second
	return cmd
}

// secondsLeft is how long a run may take: until a minute before the test
// binary's deadline, or a day when it has none (go test -timeout 0).
func secondsLeft(t *testing.T) float64 {
	deadline, ok := t.Deadline()
	if !ok {
		return (24 * time.Hour).Seconds()
	}
	return time.Until(deadline.Add(-time.Minute)).Seconds()
}

// watchdog runs the code after it once it has set faulthandler to write the
// tracebacks and exit after the seconds given.
const watchdog = `import faulthandler, sys; faulthandler.dump_traceback_later(float(sys.argv.pop(1)), exit=True); exec(sys.argv.pop(1))`

// watched runs args, the flags, -c and its code as PythonArgs gives them,
// under the watchdog.
func watched(seconds float64, args []string) []string {
	c := slices.Index(args, "-c")
	return slices.Concat(args[:c+1], []string{watchdog, strconv.FormatFloat(seconds, 'f', -1, 64)}, args[c+1:])
}

// shellWords joins args as one shell-quoted line, as --evaluator reads it.
func shellWords(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
	}
	return strings.Join(quoted, " ")
}

// runGlue runs the reference glue as a command, writes each request to it as
// one JSON line, and gives back its answers, one decoded JSON object each.
func runGlue(t *testing.T, requests ...map[string]any) []map[string]any {
	t.Helper()
	var in strings.Builder
	for _, r := range requests {
		b, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		in.Write(append(b, '\n'))
	}
	cmd := pythonCommand(t, csdfbmc.ModuleArgs(".", "csdfbmc.python_glue"))
	cmd.Dir = ".."
	cmd.Stdin = strings.NewReader(in.String())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("the glue failed: %v\n%s", err, stderr.Bytes())
	}
	var answers []map[string]any
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		var a map[string]any
		if err := json.Unmarshal(sc.Bytes(), &a); err != nil {
			t.Fatal(err)
		}
		answers = append(answers, a)
	}
	return answers
}

func TestPythonUnitTestsPass(t *testing.T) {
	cmd := pythonCommand(t, csdfbmc.ModuleArgs("..", "unittest", "discover", "-s", "tests", "-t", ".."))
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("python unit tests failed: %v\n%s", err, out)
	}
}

// The printer with retries checks six properties at once; as written it reports a
// deadlock after the retries and nothing else.
func TestThePrinterRetryExampleReportsADeadlockAfterTheRetries(t *testing.T) {
	cmd := pythonCommand(t, csdfbmc.PythonArgs(".", "examples/bmc/printer_retry_formal.json", "examples/bmc/printer_retry_formal_checks.py",
		"--evaluator-module", "csdfbmc.python_glue examples/bmc/printer_retry_formal_checks.py", "--avoid", "完了", "--within", "printing"))
	cmd.Dir = ".."
	out, err := cmd.CombinedOutput()
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
		t.Fatalf("want exit status 1, got %v\n%.2000s", err, out)
	}
	text := string(out)
	for _, want := range []string{"deadlock ×", "at printing after 再試行", "'再試行', '再試行'"} {
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
func TestThePrinterExampleReportsADeadlockThroughTheGlue(t *testing.T) {
	py := python(t)
	for _, glue := range [][]string{
		{"--evaluator", shellWords(append([]string{py}, watched(secondsLeft(t), csdfbmc.ModuleArgs(".", "csdfbmc.python_glue", "examples/bmc/printer_formal_defs.py"))...))},
		{"--evaluator-module", "csdfbmc.python_glue examples/bmc/printer_formal_defs.py"},
	} {
		t.Run(glue[0], func(t *testing.T) {
			cmd := pythonCommand(t, csdfbmc.PythonArgs(".", append([]string{"examples/bmc/printer_formal.json", "examples/bmc/printer_formal_defs.py"}, glue...)...))
			cmd.Dir = ".."
			out, err := cmd.CombinedOutput()
			if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
				t.Fatalf("want exit status 1 (a deadlock found), got %v\n%s", err, out)
			}
			if want := "states: 22\ndeadlock ×3 at printing after 印刷; first after ['印刷(枚数=3)']\n  stuck: printing --> idle : 排紙 (line 14): guard false on the state: 用紙 - 印刷枚数 >= WASTE\n"; string(out) != want {
				t.Errorf("want the deadlock and its trace, got:\n%s", out)
			}
		})
	}
}

// The expansion of a promoted diagram is walked through the reference glue,
// local clauses and all. With no end edge, the walk stops only when every ID
// of the bound is done; the IDs are a sample, so that is the bound
// reached, not a deadlock. No cycle avoids DONE.
func TestThePromotedExampleStopsOnlyWhenEveryIDIsDone(t *testing.T) {
	cmd := pythonCommand(t, csdfbmc.PythonArgs(".", "examples/bmc/attempts/attempts.json", "examples/bmc/attempts/checks.py",
		"--evaluator-module", "csdfbmc.python_glue", "--avoid", "DONE"))
	cmd.Dir = ".."
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("want exit status 0 (the bound reached is no finding), got %v\n%s", err, out)
	}
	text := string(out)
	for _, want := range []string{"states: 496\n", "bound reached ×19 at running after DONE", "(SUBMIT(試行ID=t4, 新規ジョブ={'優先度': 1}) is enabled by a value beyond a sample)"} {
		if !strings.Contains(text, want) {
			t.Errorf("want %q in the report, got:\n%s", want, text)
		}
	}
	if strings.Contains(text, "deadlock") {
		t.Errorf("want no deadlock, got:\n%s", text)
	}
	if strings.Contains(text, "cycle") {
		t.Errorf("want no cycle, got:\n%s", text)
	}
}

// A run past its time writes every thread's traceback, naming where it
// waited, and exits with a failure.
func TestAWatchedRunPastItsTimeWritesWhereItWaits(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sleeper.py"), []byte("import time\ndef waiting_here():\n    time.sleep(60)\nwaiting_here()\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(python(t), watched(1, csdfbmc.ModuleArgs(dir, "sleeper"))...)

	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatalf("want the run to fail, got it ended\n%s", out)
	}
	if !strings.Contains(string(out), "waiting_here") {
		t.Errorf("want the traceback to name waiting_here, got:\n%s", out)
	}
}
