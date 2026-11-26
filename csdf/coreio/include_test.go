package coreio

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/v6/csdf"
	"github.com/Kuniwak/puml-parallel/v6/usererr"
)

// includeAt is the !include reached from es by indexes, each the place of
// an element in a body, the body of a state, or the included tree's body.
func includeAt(t *testing.T, es []csdf.Element, at ...int) *csdf.PragmaSyntax {
	t.Helper()
	var p *csdf.PragmaSyntax
	for _, i := range at {
		switch e := es[i].(type) {
		case *csdf.StateSyntax:
			es = e.Body
		case *csdf.PragmaSyntax:
			p = e
			if e.Tree != nil {
				es = e.Tree.Body
			}
		default:
			t.Fatalf("no !include at %v", at)
		}
	}
	return p
}

// under is include, saying it read each file from under dir.
func under(dir string, include Include) Include {
	return func(p string) (string, string, error) {
		src, read, err := include(p)
		return src, filepath.Join(dir, read), err
	}
}

func TestResolveIncludes(t *testing.T) {
	sources := map[string]string{
		"local/A.puml":     "@startuml A\n!include sub/B.puml\n@enduml\n",
		"local/sub/B.puml": "@startuml B\n@enduml\n",
	}
	cases := map[string]struct {
		source   string
		base     string
		at       []int
		wantName string
		wantPath string
	}{
		"an !include in a state, the path written kept": {
			source:   "@startuml G\nstate \"R\" as r {\n  !include local/A.puml\n}\n@enduml\n",
			at:       []int{0, 0},
			wantName: "A",
			wantPath: "local/A.puml",
		},
		"the path of the file include read": {
			source:   "@startuml G\n!include local/A.puml\n@enduml\n",
			base:     "dir",
			at:       []int{0},
			wantName: "A",
			wantPath: "dir/local/A.puml",
		},
		"a nested !include, relative to the file it is in": {
			source:   "@startuml G\n!include local/A.puml\n@enduml\n",
			base:     "dir",
			at:       []int{0, 0},
			wantName: "B",
			wantPath: "dir/local/sub/B.puml",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Setup
			tree, err := csdf.Parse("", c.source)
			if err != nil {
				t.Fatal(err)
			}

			// Execute
			got, err := ResolveIncludes(tree, under(c.base, SourcesIncluder(sources)))

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			p := includeAt(t, got.Body, c.at...)
			if p.Tree == nil {
				t.Fatal("want the included tree")
			}
			if p.Tree.Name != c.wantName || p.Tree.Path != c.wantPath {
				t.Errorf("got %q at %q, want %q at %q", p.Tree.Name, p.Tree.Path, c.wantName, c.wantPath)
			}
		})
	}
}

func TestResolveFileIncludes(t *testing.T) {
	const global = "@startuml G\n!include local/MEMBER.puml\n@enduml\n"
	const dir = "../../examples/promote"
	cases := map[string]struct {
		treePath string
		base     string
		wantPath string
		wantErr  string
	}{
		"against base, whatever the tree's path": {
			treePath: "elsewhere/G.puml",
			base:     dir,
			wantPath: dir + "/local/MEMBER.puml",
		},
		"against base, for a tree with no path": {
			base:     dir,
			wantPath: dir + "/local/MEMBER.puml",
		},
		"refused with no base": {
			treePath: dir + "/G.puml",
			wantErr:  "coreio.ResolveFileIncludes: no base directory",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Setup
			tree, err := csdf.Parse(c.treePath, global)
			if err != nil {
				t.Fatal(err)
			}

			// Execute
			got, err := ResolveFileIncludes(tree, c.base)

			// Assert
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("got %v, want an error containing %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if p := includeAt(t, got.Body, 0); p.Tree == nil || p.Tree.Path != c.wantPath {
				t.Errorf("got %+v, want the tree of %s", p.Tree, c.wantPath)
			}
		})
	}
}

func TestResolveIncludesLeavesTheTreeGivenAsItIs(t *testing.T) {
	// Setup
	tree, err := csdf.Parse("", "@startuml G\n!include A.puml\n@enduml\n")
	if err != nil {
		t.Fatal(err)
	}

	// Execute
	if _, err := ResolveIncludes(tree, SourcesIncluder(map[string]string{"A.puml": "@startuml A\n@enduml\n"})); err != nil {
		t.Fatal(err)
	}

	// Assert
	if tree.Body[0].(*csdf.PragmaSyntax).Tree != nil {
		t.Error("want the tree given left as it was")
	}
}

// A refusal is an IncludeError, whose message, shown to a user as it is,
// keeps the line and the path written of each !include down to its cause.
func TestResolveIncludesRefuses(t *testing.T) {
	cases := map[string]struct {
		sources map[string]string
		// wantLine and wantArg are the line and the path written of the
		// !include the IncludeError refuses.
		wantLine  int
		wantArg   string
		wantShown string
	}{
		"a missing file, naming the line": {
			sources:   map[string]string{},
			wantLine:  2,
			wantArg:   "A.puml",
			wantShown: "line 2: !include A.puml: coreio.SourcesIncluder: A.puml is not among the included files given",
		},
		"a missing file of a nested !include, naming each": {
			sources:   map[string]string{"A.puml": "@startuml A\n\n!include B.puml\n@enduml\n"},
			wantLine:  2,
			wantArg:   "A.puml",
			wantShown: "line 2: !include A.puml: A.puml: line 3: !include B.puml: coreio.SourcesIncluder: B.puml is not among the included files given",
		},
		"a file including itself": {
			sources:   map[string]string{"A.puml": "@startuml A\n!include A.puml\n@enduml\n"},
			wantLine:  2,
			wantArg:   "A.puml",
			wantShown: "a cycle of !include: A.puml, A.puml",
		},
		"a cycle through another file": {
			sources: map[string]string{
				"A.puml":     "@startuml A\n!include sub/B.puml\n@enduml\n",
				"sub/B.puml": "@startuml B\n!include ../A.puml\n@enduml\n",
			},
			wantLine:  2,
			wantArg:   "A.puml",
			wantShown: "a cycle of !include: A.puml, sub/B.puml, A.puml",
		},
		"a file that does not parse, naming it": {
			sources:   map[string]string{"A.puml": "@startuml A\n[*] -->\n@enduml\n"},
			wantLine:  2,
			wantArg:   "A.puml",
			wantShown: "line 2: !include A.puml: A.puml: ",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Setup
			tree, err := csdf.Parse("", "@startuml G\n!include A.puml\n@enduml\n")
			if err != nil {
				t.Fatal(err)
			}

			// Execute
			_, err = ResolveIncludes(tree, SourcesIncluder(c.sources))

			// Assert
			var ie *IncludeError
			if !errors.As(err, &ie) {
				t.Fatalf("got %v, want an IncludeError", err)
			}
			if ie.Line != c.wantLine || ie.Arg != c.wantArg {
				t.Errorf("got the !include of %q on line %d, want %q on line %d", ie.Arg, ie.Line, c.wantArg, c.wantLine)
			}
			if got := usererr.Message(err); !strings.Contains(got, c.wantShown) {
				t.Errorf("shown as %q, want it to contain %q", got, c.wantShown)
			}
		})
	}
}
