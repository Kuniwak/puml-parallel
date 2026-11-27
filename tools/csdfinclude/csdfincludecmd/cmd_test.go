package csdfincludecmd

import (
	"os"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v8/cli"
	"github.com/Kuniwak/puml-parallel/v8/csdf"
	"github.com/Kuniwak/puml-parallel/v8/tools"
	"github.com/Kuniwak/puml-parallel/v8/version"
	"github.com/google/go-cmp/cmp"
)

const membersPath = "../../../examples/promote/MEMBERS.puml"

// syntaxTreeOf is the JSON csdfparse prints for source read from path.
func syntaxTreeOf(t *testing.T, path, source string) string {
	t.Helper()
	tree, err := csdf.Parse(path, source)
	if err != nil {
		t.Fatal(err)
	}
	bs, err := csdf.MarshalSyntaxTree(tree)
	if err != nil {
		t.Fatal(err)
	}
	return string(bs)
}

// fileTreeOf is the JSON csdfparse prints for the file at path.
func fileTreeOf(t *testing.T, path string) string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return syntaxTreeOf(t, path, string(src))
}

func TestNewMainFunc(t *testing.T) {
	const global = "@startuml G\nstate \"R\" as r {\n  !include local/MEMBER.puml\n}\n@enduml\n"
	cases := map[string]struct {
		args       []string
		stdin      string
		wantExit   int
		wantStdout string
		wantStderr string
	}{
		"the tree of the file included is put in the !include, its path the file read": {
			stdin:      fileTreeOf(t, membersPath),
			wantStdout: `"arg":"local/MEMBER.puml","tree":{"kind":"csdf-syntax-tree","version":1,"path":"../../../examples/promote/local/MEMBER.puml"`,
		},
		"-base resolves the !include of a tree with no path, its path joined to base": {
			args:       []string{"-base", "../../../examples/promote"},
			stdin:      syntaxTreeOf(t, "", global),
			wantStdout: `"arg":"local/MEMBER.puml","tree":{"kind":"csdf-syntax-tree","version":1,"path":"../../../examples/promote/local/MEMBER.puml"`,
		},
		"-base overrides the directory of the tree's path": {
			args:       []string{"-base", "../../../examples/promote"},
			stdin:      syntaxTreeOf(t, "elsewhere/G.puml", global),
			wantStdout: `"path":"../../../examples/promote/local/MEMBER.puml"`,
		},
		"a tree with no !include is printed as it is": {
			stdin:      syntaxTreeOf(t, "", "@startuml\nstate \"S\" as s0\n[*] --> s0\n@enduml\n"),
			wantStdout: `"body":[{"kind":"state"`,
		},
		"a tree with no path has its !include resolved against the current directory": {
			stdin:      syntaxTreeOf(t, "", "@startuml G\n!include testdata/LEAF.puml\n@enduml\n"),
			wantStdout: `"arg":"testdata/LEAF.puml","tree":{"kind":"csdf-syntax-tree","version":1,"path":"testdata/LEAF.puml"`,
		},
		"a missing file is refused": {
			stdin:      fileTreeOf(t, "testdata/MISSING.puml"),
			wantExit:   1,
			wantStderr: "line 2: !include absent.puml: no such file or directory",
		},
		"a cycle of !include is refused": {
			stdin:      fileTreeOf(t, "testdata/A.puml"),
			wantExit:   1,
			wantStderr: "a cycle of !include: sub/B.puml, A.puml, sub/B.puml",
		},
		"PlantUML is refused, naming csdfparse": {
			stdin:      global,
			wantExit:   1,
			wantStderr: "run csdfparse on the diagram first",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Setup
			cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
			spy := cli.SpyProcInout()
			spy.Stdin = cli.StubStdin(strings.NewReader(c.stdin))

			// Execute
			exitStatus := cmdFunc(c.args, spy.New())

			// Assert
			if exitStatus != c.wantExit {
				t.Fatalf("want %d, got %d: %s", c.wantExit, exitStatus, spy.Stderr.String())
			}
			if !strings.Contains(spy.Stdout.String(), c.wantStdout) {
				t.Errorf("want %q in the output, got %s", c.wantStdout, spy.Stdout.String())
			}
			if !strings.Contains(spy.Stderr.String(), c.wantStderr) {
				t.Errorf("want %q on stderr, got %q", c.wantStderr, spy.Stderr.String())
			}
		})
	}
}

func TestNewMainFuncVersion(t *testing.T) {
	// Setup
	cmdFunc := tools.NewCommandFunc(NewParseOptionsFunc(), NewMainFunc())
	spy := cli.SpyProcInout()

	// Execute
	exitStatus := cmdFunc([]string{"-v"}, spy.New())

	// Assert
	if exitStatus != 0 {
		t.Fatalf("want 0, got %d", exitStatus)
	}
	if diff := cmp.Diff(version.Version+"\n", spy.Stdout.String()); diff != "" {
		t.Error(diff)
	}
}
