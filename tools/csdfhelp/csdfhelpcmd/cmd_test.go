package csdfhelpcmd

import (
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools/toolsdoc"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

// stdoutOf asserts that cmdFunc succeeded and returns what it wrote to stdout.
func stdoutOf(t *testing.T, exitStatus int, spy *cli.ProcInoutSpy) string {
	t.Helper()

	if exitStatus != 0 {
		t.Log(spy.Stderr.String())
		t.Fatalf("want 0, got %d", exitStatus)
	}
	return spy.Stdout.String()
}

func entryOf(t *testing.T, name string) toolsdoc.Entry {
	t.Helper()

	entry, ok := toolsdoc.Lookup(Registry(), name)
	if !ok {
		t.Fatalf("want an entry named %s, got none", name)
	}
	return entry
}

func TestNewMainFuncExactOutput(t *testing.T) {
	type testCase struct {
		Args     []string
		Expected string
	}

	testCases := map[string]testCase{
		"-v (representative value)": {
			Args:     []string{"-v"},
			Expected: version.Version + "\n",
		},
		"-h writes the usage to stderr, so stdout stays empty (representative value)": {
			Args:     []string{"-h"},
			Expected: "",
		},
		"-short with one tool is one line (lower boundary value)": {
			Args:     []string{"-short", "csdfparse"},
			Expected: "csdfparse: " + entryOf(t, "csdfparse").Summary + "\n",
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			cmdFunc := NewCommandFunc()
			spy := cli.SpyProcInout()

			// Act
			exitStatus := cmdFunc(testCase.Args, spy.New())

			// Assert
			got := stdoutOf(t, exitStatus, spy)
			if testCase.Expected != got {
				t.Error(cmp.Diff(testCase.Expected, got))
			}
		})
	}
}

func TestNewMainFuncSelection(t *testing.T) {
	type testCase struct {
		Args            []string
		WantContains    []string
		WantNotContains []string
	}

	testCases := map[string]testCase{
		"no argument documents every non-hidden tool (lower boundary value)": {
			Args:            nil,
			WantContains:    []string{"# csdfparse\n", "Usage: csdfparse", "# csdfevents\n", "# csdfreplcmd\n"},
			WantNotContains: []string{"# csdfhelp\n"},
		},
		"a tool name documents only that tool (lower boundary value)": {
			Args:            []string{"csdfparse"},
			WantContains:    []string{"# csdfparse\n", "Usage: csdfparse"},
			WantNotContains: []string{"# csdfevents\n"},
		},
		"two tool names (representative value)": {
			Args:            []string{"csdfparse", "csdfevents"},
			WantContains:    []string{"# csdfparse\n", "# csdfevents\n"},
			WantNotContains: []string{"# csdfnorm\n"},
		},
		"the hidden csdfhelp is documented when named (representative value)": {
			Args:         []string{"csdfhelp"},
			WantContains: []string{"# csdfhelp\n", "Usage: csdfhelp"},
		},
		"a command group also documents each subcommand (representative value)": {
			Args:         []string{"csdfreplcmd"},
			WantContains: []string{"# csdfreplcmd\n", "## csdfreplcmd session\n", "## csdfreplcmd read\n"},
		},
		"-short lists every non-hidden tool without Markdown (representative value)": {
			Args:            []string{"-short"},
			WantContains:    []string{"csdfparse: ", "csdfreplcmd: "},
			WantNotContains: []string{"# ", "```", "csdfhelp: "},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			cmdFunc := NewCommandFunc()
			spy := cli.SpyProcInout()

			// Act
			exitStatus := cmdFunc(testCase.Args, spy.New())

			// Assert
			got := stdoutOf(t, exitStatus, spy)
			for _, want := range testCase.WantContains {
				if !strings.Contains(got, want) {
					t.Errorf("want %q, got none", want)
				}
			}
			for _, unwanted := range testCase.WantNotContains {
				if strings.Contains(got, unwanted) {
					t.Errorf("want no %q, got one", unwanted)
				}
			}
		})
	}
}

// TestNewMainFuncDocumentsEveryRegisteredTool keeps the no-argument document in
// step with the registry, whatever it comes to hold.
func TestNewMainFuncDocumentsEveryRegisteredTool(t *testing.T) {
	// Arrange
	cmdFunc := NewCommandFunc()
	spy := cli.SpyProcInout()

	// Act
	exitStatus := cmdFunc(nil, spy.New())

	// Assert
	got := stdoutOf(t, exitStatus, spy)
	for _, entry := range Registry() {
		heading := "# " + entry.Name + "\n"
		if entry.Hidden {
			if strings.Contains(got, heading) {
				t.Errorf("want no %q, got one", heading)
			}
			continue
		}
		if !strings.Contains(got, heading) {
			t.Errorf("want %q, got none", heading)
		}
	}
}

// TestNewMainFuncShortListsEveryRegisteredTool does the same for -short.
func TestNewMainFuncShortListsEveryRegisteredTool(t *testing.T) {
	// Arrange
	cmdFunc := NewCommandFunc()
	spy := cli.SpyProcInout()

	// Act
	exitStatus := cmdFunc([]string{"-short"}, spy.New())

	// Assert
	got := stdoutOf(t, exitStatus, spy)
	for _, entry := range Registry() {
		line := entry.Name + ": " + entry.Summary + "\n"
		if entry.Hidden {
			if strings.Contains(got, line) {
				t.Errorf("want no %q, got one", line)
			}
			continue
		}
		if !strings.Contains(got, line) {
			t.Errorf("want %q, got none", line)
		}
	}
}
