package csdfpromotecmd

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v4/cli"
	"github.com/Kuniwak/puml-parallel/v4/csdf"
	"github.com/Kuniwak/puml-parallel/v4/tools"
	"github.com/Kuniwak/puml-parallel/v4/version"
	"github.com/google/go-cmp/cmp"
)

const membersPath = "../../../examples/promote/MEMBERS.puml"

var update = flag.Bool("update", false, "rewrite the recorded expansions")

// TestNewMainFuncExpandsTheRecordedExamples expands every worked example and
// compares it with the expansion recorded next to it, which makes a change
// in what csdfpromote prints visible in review. Run with -update to rewrite
// them.
func TestNewMainFuncExpandsTheRecordedExamples(t *testing.T) {
	const dir = "../../../examples/promote"
	paths, err := filepath.Glob(filepath.Join(dir, "*.puml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if strings.HasSuffix(path, ".expanded.puml") {
			continue
		}
		t.Run(filepath.Base(path), func(t *testing.T) {
			// Setup
			cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
			spy := cli.SpyProcInout()
			golden := strings.TrimSuffix(path, ".puml") + ".expanded.puml"

			// Execute
			exitStatus := cmdFunc([]string{path}, spy.New())

			// Assert
			if exitStatus != 0 {
				t.Log(spy.Stderr.String())
				t.Fatalf("want 0, got %d", exitStatus)
			}
			// The recording leaves the title out, which the command fills
			// with the command line that produced it.
			got := tools.Untitled(spy.Stdout.String())
			if *update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			bs, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("%v; run the test with -update to record it", err)
			}
			if diff := cmp.Diff(string(bs), got); diff != "" {
				t.Errorf("mismatch (-recorded +got):\n%s", diff)
			}
		})
	}
}

// syntaxTreeOf is the JSON csdfparse prints for the diagram at path.
func syntaxTreeOf(t *testing.T, path string) string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := csdf.Parse(path, string(src))
	if err != nil {
		t.Fatal(err)
	}
	bs, err := csdf.MarshalSyntaxTree(tree)
	if err != nil {
		t.Fatal(err)
	}
	return string(bs)
}

func TestNewMainFuncReadsTheSyntaxTreeCsdfparsePrints(t *testing.T) {
	// Setup
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()
	spy.Stdin = strings.NewReader(syntaxTreeOf(t, "../../../examples/valid/skip.puml"))

	// Execute
	exitStatus := cmdFunc([]string{"-format", "json"}, spy.New())

	// Assert
	if exitStatus != 0 {
		t.Fatalf("want 0, got %d: %s", exitStatus, spy.Stderr.String())
	}
	if want := `"ends":[{"src":"s0","guard":[]}]`; !strings.Contains(spy.Stdout.String(), want) {
		t.Errorf("want %q in the output, got:\n%s", want, spy.Stdout.String())
	}
}

// The command's other runs differ only in what they print, so they are one
// table: the arguments, the exit status, and what standard output has to say.
func TestNewMainFunc(t *testing.T) {
	cases := map[string]struct {
		args            []string
		stdin           string
		wantExit        int
		wantStdoutEmpty bool
		wantStdout      []string
		wantNoStdout    []string
		wantStderr      string
	}{
		"-no-comments is refused as an unknown flag": {
			args:            []string{"-no-comments", membersPath},
			wantExit:        1,
			wantStdoutEmpty: true,
			wantStderr:      "flag provided but not defined: -no-comments",
		},
		"-lint-only prints nothing": {
			args:            []string{"-lint-only", membersPath},
			wantStdoutEmpty: true,
		},
		"-json is refused as an unknown flag": {
			args:            []string{"-json", membersPath},
			wantExit:        1,
			wantStdoutEmpty: true,
			wantStderr:      "flag provided but not defined: -json",
		},
		"-format json prints the expansion as csdfparse prints a diagram": {
			args:         []string{"-format", "json", membersPath},
			wantStdout:   []string{`"version":3`, `"kind":"builtin","op":"notin"`},
			wantNoStdout: []string{"@startuml", `"origin"`},
		},
		"-format json prints a diagram with no directive as it is, its end edge included": {
			args:       []string{"-format", "json", "../../../examples/valid/skip.puml"},
			wantStdout: []string{`"version":3`, `"ends":[{"src":"s0","guard":[]}]`},
		},
		"a diagram with no directive is printed as it is": {
			args:       []string{"../../../examples/valid/skip.puml"},
			wantStdout: []string{"s0 --> [*]"},
		},
		"an unknown -format is refused": {
			args:            []string{"-format", "tsv", membersPath},
			wantExit:        1,
			wantStdoutEmpty: true,
			wantStderr:      "unknown output",
		},
		"-template glosses the builtins in a line comment": {
			args:       []string{"-template", "../../../examples/promote/templates/ja.tmpl", membersPath},
			wantStdout: []string{"' members[会員ID] は〈memActive〉", "@notin(会員ID, members')"},
		},
		"-template is refused with -format json": {
			args:            []string{"-template", "../../../examples/promote/templates/ja.tmpl", "-format", "json", membersPath},
			wantExit:        1,
			wantStdoutEmpty: true,
			wantStderr:      "-format json takes none",
		},
		"-only keeps the named families, and says what its result means": {
			args:         []string{"-only", "receipts", "../../../examples/promote/RECEIPTS.puml"},
			wantStdout:   []string{"running : receipts"},
			wantNoStdout: []string{"REPORT", "running : cycles"},
			wantStderr:   "note: a restriction keeps only safety",
		},
		"-only names a family by its map, not its type": {
			args:            []string{"-only", "Receipt", "../../../examples/promote/RECEIPTS.puml"},
			wantExit:        1,
			wantStdoutEmpty: true,
			wantStderr:      "Receipt is no promoted map",
		},
		"-only with -format json drops what reads a left-out map, and says so": {
			args:         []string{"-only", "receipts", "-format", "json", "../../../examples/promote/RECEIPTS.puml"},
			wantStdout:   []string{`"version":3`},
			wantNoStdout: []string{"cycles"},
			wantStderr:   "dropped, as it reads a left-out map",
		},
		"-only refuses an unknown family": {
			args:            []string{"-only", "shipments", "../../../examples/promote/RECEIPTS.puml"},
			wantExit:        1,
			wantStdoutEmpty: true,
			wantStderr:      "shipments is no promoted map",
		},
		"-only is refused for a diagram with no directive": {
			args:            []string{"-only", "receipts", "../../../examples/valid/skip.puml"},
			wantExit:        1,
			wantStdoutEmpty: true,
			wantStderr:      "receipts is no promoted map",
		},
		"-v prints the version": {
			args:       []string{"-v"},
			wantStdout: []string{version.Version + "\n"},
		},
		// The sync names a map no block promotes.
		"an unsound promotion is refused, with the reason and no diagram": {
			args: []string{"-base", "../../../examples/promote"},
			stdin: `@startuml A
state "稼働中" as running {
  state "members : (会員ID : 会員ID) ⇸ Member" as runningMembers <<promote>> {
    !include local/MEMBER.puml
  }
}
[*] --> running
note as s
  sync OPEN : reviews(審査ID)
end note
@enduml
`,
			wantExit:        1,
			wantStdoutEmpty: true,
			wantStderr:      "reviews",
		},
		"a warning is printed at its level": {
			args:       []string{"../../../examples/promote/MODES.puml"},
			wantStderr: "info: reviews: frozen in degraded",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
			spy := cli.SpyProcInout()
			spy.Stdin = strings.NewReader(c.stdin)

			exitStatus := cmdFunc(c.args, spy.New())

			if exitStatus != c.wantExit {
				t.Log(spy.Stderr.String())
				t.Fatalf("want %d, got %d", c.wantExit, exitStatus)
			}

			stdout := spy.Stdout.String()
			if c.wantStdoutEmpty && stdout != "" {
				t.Errorf("want no output, got %q", stdout)
			}
			for _, want := range c.wantStdout {
				if !strings.Contains(stdout, want) {
					t.Errorf("want %q in the output, got:\n%s", want, stdout)
				}
			}
			for _, unwanted := range c.wantNoStdout {
				if strings.Contains(stdout, unwanted) {
					t.Errorf("want no %q in the output, got:\n%s", unwanted, stdout)
				}
			}
			if c.wantStderr != "" && !strings.Contains(spy.Stderr.String(), c.wantStderr) {
				t.Errorf("want %q on stderr, got %q", c.wantStderr, spy.Stderr.String())
			}
		})
	}
}
