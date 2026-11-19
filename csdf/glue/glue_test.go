package glue_test

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v3/csdf/glue"
	"github.com/google/go-cmp/cmp"
)

// pythonGlue is the reference glue run as a command from the repository root.
func pythonGlue(t *testing.T) *exec.Cmd {
	t.Helper()
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is not on the PATH")
	}
	cmd := exec.Command(python, "-E", "-s", "-B", "-m", "csdfbmc.python_glue")
	cmd.Dir = "../.."
	return cmd
}

func wire(t *testing.T, source string) clause.AtomWire {
	t.Helper()
	f, err := clause.Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	w, _, err := clause.Wire(f.Conjuncts[0].(clause.Atom))
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func TestLintAsksTheGlueWhetherAnAtomIsOfItsLanguage(t *testing.T) {
	type testCase struct {
		Atom    string
		Role    string
		WantWhy string
	}
	testCases := map[string]testCase{
		"a guard of the language": {Atom: "$n > 0", Role: "guard", WantWhy: ""},
		"a post of the language":  {Atom: "$n' == $n + 1", Role: "post", WantWhy: ""},
		"an undefined name":       {Atom: "$n > LIMIT", Role: "guard", WantWhy: "LIMIT is neither a builtin nor a definition"},
		"a post of another shape": {Atom: "$n' > 0", Role: "post", WantWhy: "a post atom of this glue is"},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			c, err := glue.Start(pythonGlue(t))
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()

			// Execute
			why, err := c.Lint(wire(t, tc.Atom), tc.Role)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if tc.WantWhy == "" && why != "" || !strings.Contains(why, tc.WantWhy) {
				t.Errorf("want why holding %q, got %q", tc.WantWhy, why)
			}
		})
	}
}

func TestSupportsSaysWhatTheHelloListed(t *testing.T) {
	// Setup
	c, err := glue.Start(pythonGlue(t))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	// Execute, Assert
	if !c.Supports("lint") || c.Supports("beyond") {
		t.Errorf("want lint supported and beyond not")
	}
}

func TestSplitCommandReadsWordsAsAShellDoes(t *testing.T) {
	type testCase struct {
		Line string
		Want []string
	}
	testCases := map[string]testCase{
		"words":             {Line: "python3 -m csdfbmc.python_glue", Want: []string{"python3", "-m", "csdfbmc.python_glue"}},
		"single quotes":     {Line: "glue 'a b.py'", Want: []string{"glue", "a b.py"}},
		"double quotes":     {Line: `glue "a \"b\".py"`, Want: []string{"glue", `a "b".py`}},
		"a backslash":       {Line: `glue a\ b.py`, Want: []string{"glue", "a b.py"}},
		"a carriage return": {Line: "glue\r\na.py\r", Want: []string{"glue", "a.py"}},
		"an empty word":     {Line: `glue ''`, Want: []string{"glue", ""}},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Execute
			got, err := glue.SplitCommand(tc.Line)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.Want, got); diff != "" {
				t.Errorf("words mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSplitCommandRefusesWhatAShellCannotRead(t *testing.T) {
	testCases := map[string]string{
		"an unclosed single quote": `glue 'a`,
		"an unclosed double quote": `glue "a`,
		"a trailing backslash":     `glue a\`,
	}
	for name, line := range testCases {
		t.Run(name, func(t *testing.T) {
			// Execute
			_, err := glue.SplitCommand(line)

			// Assert
			if err == nil {
				t.Errorf("want an error for %q", line)
			}
		})
	}
}

// fakeGlue is a glue that says hello with lint and then runs script.
func fakeGlue(script string) *exec.Cmd {
	return exec.Command("sh", "-c", `read line; echo '{"language": "fake", "protocol": 2, "supports": ["lint"]}'; `+script)
}

func TestLintRefusesAnAnswerItCannotTrust(t *testing.T) {
	testCases := map[string]string{
		"ok false without why": `read line; echo '{"ok": false}'; cat > /dev/null`,
		"ok false, why empty":  `read line; echo '{"ok": false, "why": ""}'; cat > /dev/null`,
		"neither ok nor error": `read line; echo '{}'; cat > /dev/null`,
		"an error":             `read line; echo '{"error": "boom"}'; cat > /dev/null`,
		"no answer":            `exit 0`,
	}
	for name, script := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			c, err := glue.Start(fakeGlue(script))
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()

			// Execute
			_, err = c.Lint(clause.AtomWire{Text: []string{"x"}, Holes: []int{}, Primed: []bool{}}, "guard")

			// Assert
			if err == nil {
				t.Error("want an error")
			}
		})
	}
}

func TestLintSaysTheGlueEndedWithoutAnsweringWhenItStopsReadingFirst(t *testing.T) {
	// Setup: the glue closes its input before it answers the hello, so the
	// lint request after the hello cannot be written.
	c, err := glue.Start(exec.Command("sh", "-c", `read line; exec <&-; echo '{"language": "fake", "protocol": 2, "supports": ["lint"]}'`))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	// Execute
	_, err = c.Lint(clause.AtomWire{Text: []string{"x"}, Holes: []int{}, Primed: []bool{}}, "guard")

	// Assert
	if err == nil || !strings.Contains(err.Error(), "the glue ended without answering") {
		t.Errorf("want the glue said to have ended without answering, got %v", err)
	}
}

func TestCloseReadsWhatTheGlueStillWritesAndSaysHowItEnded(t *testing.T) {
	type testCase struct {
		Script  string
		WantErr bool
	}
	testCases := map[string]testCase{
		"a glue writing more than a pipe holds after its input ends": {Script: `cat > /dev/null; head -c 1000000 /dev/zero`, WantErr: false},
		"a glue that fails": {Script: `cat > /dev/null; exit 3`, WantErr: true},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Setup
			c, err := glue.Start(fakeGlue(tc.Script))
			if err != nil {
				t.Fatal(err)
			}

			// Execute
			done := make(chan error, 1)
			go func() { done <- c.Close() }()

			// Assert
			select {
			case err := <-done:
				if (err != nil) != tc.WantErr {
					t.Errorf("want an error %v, got %v", tc.WantErr, err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("Close did not return")
			}
		})
	}
}
