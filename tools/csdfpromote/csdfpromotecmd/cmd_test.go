package csdfpromotecmd

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/version"
	"github.com/google/go-cmp/cmp"
)

const accountsPath = "../../../examples/promote/ACCOUNTS.puml"

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
		"-no-comments leaves the origins out": {
			args:         []string{"-no-comments", accountsPath},
			wantNoStdout: []string{"' promote:"},
		},
		"-lint-only prints nothing": {
			args:            []string{"-lint-only", accountsPath},
			wantStdoutEmpty: true,
		},
		"-json reports the directives": {
			args:       []string{"-json", accountsPath},
			wantStdout: []string{`"map":"accounts"`},
		},
		"-format json prints the expansion with its clauses": {
			args:         []string{"-format", "json", accountsPath},
			wantStdout:   []string{`"kind":"in_dom"`, `"origin":"promote:`},
			wantNoStdout: []string{"@startuml"},
		},
		"-format json with -no-comments leaves the origins out": {
			args:         []string{"-format", "json", "-no-comments", accountsPath},
			wantNoStdout: []string{`"origin"`},
		},
		"an unknown -format is refused": {
			args:            []string{"-format", "tsv", accountsPath},
			wantExit:        1,
			wantStdoutEmpty: true,
			wantStderr:      "unknown output",
		},
		"-template glosses the builtins in a line comment": {
			args:       []string{"-template", "../../../examples/promote/templates/ja.tmpl", accountsPath},
			wantStdout: []string{"' accounts[口座ID] は〈accOpen〉", "@notin(口座ID, accounts')"},
		},
		"-template is refused with -format json": {
			args:            []string{"-template", "../../../examples/promote/templates/ja.tmpl", "-format", "json", accountsPath},
			wantExit:        1,
			wantStdoutEmpty: true,
			wantStderr:      "-format json takes none",
		},
		"-only keeps the named families, and says what its result means": {
			args:         []string{"-only", "buys", "../../../examples/promote/TRADES.puml"},
			wantStdout:   []string{"running : buys"},
			wantNoStdout: []string{"REPORT", "running : cycles"},
			wantStderr:   "note: a restriction keeps only safety",
		},
		"-only names a family by its map, not its type": {
			args:            []string{"-only", "Buy", "../../../examples/promote/TRADES.puml"},
			wantExit:        1,
			wantStdoutEmpty: true,
			wantStderr:      "Buy is no promoted map",
		},
		"-only lists what it drops on stderr and in the JSON": {
			args:       []string{"-only", "buys", "-format", "json", "../../../examples/promote/TRADES.puml"},
			wantStdout: []string{`"kind":"dropped"`, `"maps":["cycles"]`},
			wantStderr: "-only leaves out",
		},
		"-only refuses an unknown family": {
			args:            []string{"-only", "sells", "../../../examples/promote/TRADES.puml"},
			wantExit:        1,
			wantStdoutEmpty: true,
			wantStderr:      "sells is no promoted map",
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
  state "accounts : 口座ID ⇸ Account" as runningAccounts <<promote>> {
    !include local/ACCOUNT.puml
  }
}
[*] --> running
note as s
  sync OPEN : audits(監査ID)
end note
@enduml
`,
			wantExit:        1,
			wantStdoutEmpty: true,
			wantStderr:      "audits",
		},
		"a warning is printed at its level": {
			args:       []string{"../../../examples/promote/MODES.puml"},
			wantStderr: "info: audits: frozen in degraded",
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
