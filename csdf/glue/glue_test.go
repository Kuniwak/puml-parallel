package glue_test

import (
	"os/exec"
	"strings"
	"testing"

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
		"words":         {Line: "python3 -m csdfbmc.python_glue", Want: []string{"python3", "-m", "csdfbmc.python_glue"}},
		"single quotes": {Line: "glue 'a b.py'", Want: []string{"glue", "a b.py"}},
		"double quotes": {Line: `glue "a \"b\".py"`, Want: []string{"glue", `a "b".py`}},
		"a backslash":   {Line: `glue a\ b.py`, Want: []string{"glue", "a b.py"}},
		"an empty word": {Line: `glue ''`, Want: []string{"glue", ""}},
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
