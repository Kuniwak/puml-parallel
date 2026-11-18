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

	// Setup
	c, err := glue.Start(pythonGlue(t))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
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

func TestStartRefusesAGlueThatDoesNotAnswerLint(t *testing.T) {
	// Setup: a glue whose hello lists no lint.
	cmd := exec.Command("sh", "-c", `read line; echo '{"language": "fake", "protocol": 2}'; cat > /dev/null`)

	// Execute
	_, err := glue.Start(cmd)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "does not answer lint") {
		t.Errorf("want a refusal of a glue without lint, got %v", err)
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

func TestSplitCommandRefusesAnUnclosedQuote(t *testing.T) {
	// Execute
	_, err := glue.SplitCommand(`glue 'a`)

	// Assert
	if err == nil {
		t.Error("want an error for an unclosed quote")
	}
}
