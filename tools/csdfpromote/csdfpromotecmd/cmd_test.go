package csdfpromotecmd

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v6/cli"
	"github.com/Kuniwak/puml-parallel/v6/csdf"
	"github.com/Kuniwak/puml-parallel/v6/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/v6/tools"
	"github.com/Kuniwak/puml-parallel/v6/version"
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
			spy.Stdin = strings.NewReader(resolvedTreeOf(t, path))
			golden := strings.TrimSuffix(path, ".puml") + ".expanded.puml"

			// Execute
			exitStatus := cmdFunc(nil, spy.New())

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

// resolvedTreeOf is the JSON csdfinclude prints for the diagram at path.
func resolvedTreeOf(t *testing.T, path string) string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolvedTreeOfSource(t, path, string(src))
}

// resolvedTreeOfSource is the JSON csdfinclude prints for source, parsed
// as read from path.
func resolvedTreeOfSource(t *testing.T, path, source string) string {
	t.Helper()
	tree, err := csdf.Parse(path, source)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := coreio.ResolveFileIncludes(tree, csdf.BaseDirOfTree(tree))
	if err != nil {
		t.Fatal(err)
	}
	bs, err := csdf.MarshalSyntaxTree(resolved)
	if err != nil {
		t.Fatal(err)
	}
	return string(bs)
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

// The command's other runs differ only in what they print, so they are one
// table: the arguments, the exit status, and what standard output has to say.
func TestNewMainFunc(t *testing.T) {
	cases := map[string]struct {
		args            []string
		stdin           string
		stdinTreeOf     string // the path whose syntax tree is the standard input
		stdinResolvedOf string // the path whose resolved syntax tree is the standard input
		stdinSourceAt   string // the path stdin is parsed as read from, its tree resolved, when set
		wantExit        int
		wantStdoutEmpty bool
		wantStdout      []string
		wantNoStdout    []string
		wantStderr      string
	}{
		"-no-comments is refused as an unknown flag": {
			args:            []string{"-no-comments"},
			stdinResolvedOf: membersPath,
			wantExit:        1,
			wantStdoutEmpty: true,
			wantStderr:      "flag provided but not defined: -no-comments",
		},
		"-lint-only prints nothing": {
			args:            []string{"-lint-only"},
			stdinResolvedOf: membersPath,
			wantStdoutEmpty: true,
		},
		"-json is refused as an unknown flag": {
			args:            []string{"-json"},
			stdinResolvedOf: membersPath,
			wantExit:        1,
			wantStdoutEmpty: true,
			wantStderr:      "flag provided but not defined: -json",
		},
		"-format json prints the expansion as csdfparse prints a diagram": {
			args:            []string{"-format", "json"},
			stdinResolvedOf: membersPath,
			wantStdout:      []string{`"version":3`, `"kind":"builtin","op":"notin"`},
			wantNoStdout:    []string{"@startuml", `"origin"`},
		},
		"-format json prints a diagram with no directive as it is, its end edge included": {
			args:            []string{"-format", "json"},
			stdinResolvedOf: "../../../examples/valid/skip.puml",
			wantStdout:      []string{`"version":3`, `"ends":[{"src":"s0","guard":[]}]`},
		},
		"a diagram with no directive is printed as it is": {
			stdinResolvedOf: "../../../examples/valid/skip.puml",
			wantStdout:      []string{"s0 --> [*]"},
		},
		"an unknown -format is refused": {
			args:            []string{"-format", "tsv"},
			stdinResolvedOf: membersPath,
			wantExit:        1,
			wantStdoutEmpty: true,
			wantStderr:      "unknown output",
		},
		"-template glosses the builtins in a line comment": {
			args:            []string{"-template", "../../../examples/promote/templates/ja.tmpl"},
			stdinResolvedOf: membersPath,
			wantStdout:      []string{"' members[会員ID] は〈memActive〉", "@notin(会員ID, members')"},
		},
		"-template is refused with -format json": {
			args:            []string{"-template", "../../../examples/promote/templates/ja.tmpl", "-format", "json"},
			stdinResolvedOf: membersPath,
			wantExit:        1,
			wantStdoutEmpty: true,
			wantStderr:      "-format json takes none",
		},
		"-only keeps the named families, and says what its result means": {
			args:            []string{"-only", "receipts"},
			stdinResolvedOf: "../../../examples/promote/RECEIPTS.puml",
			wantStdout:      []string{"running : receipts"},
			wantNoStdout:    []string{"REPORT", "running : cycles"},
			wantStderr:      "note: a restriction keeps only safety",
		},
		"-only names a family by its map, not its type": {
			args:            []string{"-only", "Receipt"},
			stdinResolvedOf: "../../../examples/promote/RECEIPTS.puml",
			wantExit:        1,
			wantStdoutEmpty: true,
			wantStderr:      "Receipt is no promoted map",
		},
		"-only with -format json drops what reads a left-out map, and says so": {
			args:            []string{"-only", "receipts", "-format", "json"},
			stdinResolvedOf: "../../../examples/promote/RECEIPTS.puml",
			wantStdout:      []string{`"version":3`},
			wantNoStdout:    []string{"cycles"},
			wantStderr:      "dropped, as it reads a left-out map",
		},
		"-only refuses an unknown family": {
			args:            []string{"-only", "shipments"},
			stdinResolvedOf: "../../../examples/promote/RECEIPTS.puml",
			wantExit:        1,
			wantStdoutEmpty: true,
			wantStderr:      "shipments is no promoted map",
		},
		"-only is refused for a diagram with no directive": {
			args:            []string{"-only", "receipts"},
			stdinResolvedOf: "../../../examples/valid/skip.puml",
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
			stdinSourceAt: "../../../examples/promote/A.puml",
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
		"a syntax tree with no directive is read": {
			args:        []string{"-format", "json"},
			stdinTreeOf: "../../../examples/valid/skip.puml",
			wantStdout:  []string{`"ends":[{"src":"s0","guard":[]}]`},
		},
		// The file the !include names is there, but csdfinclude resolves it.
		"an unresolved !include is refused, naming csdfinclude": {
			args:            []string{"-format", "json"},
			stdinTreeOf:     membersPath,
			wantExit:        1,
			wantStdoutEmpty: true,
			wantStderr:      "csdfinclude",
		},
		"a syntax tree with its !include resolved is expanded": {
			args:            []string{"-format", "json"},
			stdinResolvedOf: membersPath,
			wantStdout:      []string{`"version":3`, `"kind":"builtin","op":"notin"`},
		},
		"PlantUML is refused, naming csdfparse": {
			stdin:           "@startuml\nstate \"S\" as s0\n[*] --> s0\n@enduml\n",
			wantExit:        1,
			wantStdoutEmpty: true,
			wantStderr:      "run csdfparse on the diagram first",
		},
		"a warning is printed at its level": {
			stdinResolvedOf: "../../../examples/promote/MODES.puml",
			wantStderr:      "info: reviews: frozen in degraded",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
			spy := cli.SpyProcInout()
			stdin := c.stdin
			if c.stdinTreeOf != "" {
				stdin = syntaxTreeOf(t, c.stdinTreeOf)
			}
			if c.stdinResolvedOf != "" {
				stdin = resolvedTreeOf(t, c.stdinResolvedOf)
			}
			if c.stdinSourceAt != "" {
				stdin = resolvedTreeOfSource(t, c.stdinSourceAt, c.stdin)
			}
			spy.Stdin = strings.NewReader(stdin)

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
