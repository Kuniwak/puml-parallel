package csdfbmc_test

import (
	"bufio"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/google/go-cmp/cmp"
)

// An atom as the Go side writes it is what the reference glue reads: a post
// atom parsed from its surface syntax gets its values after the step from
// the glue run as a command.
func TestTheGlueReadsTheAtomsTheToolsWrite(t *testing.T) {
	// Setup
	f, err := clause.Parse("$x' in ($x, $x + 1)")
	if err != nil {
		t.Fatal(err)
	}
	wire, _ := clause.Wire(f.Conjuncts[0].(clause.Atom))
	lines := []map[string]any{
		{"op": "hello", "protocol": 2},
		{"op": "post", "atom": wire, "values": [][]any{{1}}},
	}
	var in strings.Builder
	for _, l := range lines {
		b, err := json.Marshal(l)
		if err != nil {
			t.Fatal(err)
		}
		in.Write(append(b, '\n'))
	}
	cmd := exec.Command(python(t), "-E", "-s", "-B", "-m", "csdfbmc.python_glue")
	cmd.Dir = ".."
	cmd.Stdin = strings.NewReader(in.String())

	// Execute
	out, err := cmd.Output()

	// Assert
	if err != nil {
		t.Fatalf("the glue failed: %v", err)
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
	want := []map[string]any{
		{"language": "python", "protocol": float64(2), "supports": []any{"symmetric"}},
		{"results": []any{[]any{[]any{float64(1)}, []any{float64(2)}}}},
	}
	if diff := cmp.Diff(want, answers); diff != "" {
		t.Errorf("answers mismatch (-want +got):\n%s", diff)
	}
}
