package csdf

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// Every diagram of the examples, plain or global, has a syntax tree.
func TestParseReadsEveryExample(t *testing.T) {
	err := filepath.WalkDir("../examples", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "invalid" {
			return filepath.SkipDir
		}
		if d.IsDir() || filepath.Ext(path) != ".puml" {
			return nil
		}
		t.Run(path, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := Parse("", string(src)); err != nil {
				t.Error(err)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestParseReadsTheElementsWhereTheyAreWritten(t *testing.T) {
	tests := map[string]struct {
		source string
		want   []Element
	}{
		"comments, pragmas, ignored regions and observations": {
			source: "@startuml\n' CSDF-IGNORE-BEGIN\n  title T\n' CSDF-IGNORE-END\n' CSDF-OBSERVE v = s0\n!include x.puml\nstate \"S\" as s0 /' why '/\n@enduml\n",
			want: []Element{
				&IgnoreSyntax{Pos: Pos{2, 1}, Lines: []string{"  title T"}},
				&ObserveSyntax{Pos: Pos{5, 1}, Name: "v", View: true, States: []string{"s0"}},
				&PragmaSyntax{Pos: Pos{6, 1}, Name: "include", Arg: "x.puml"},
				&StateSyntax{Pos: Pos{7, 1}, ID: "s0", Label: "S"},
				&CommentSyntax{Pos: Pos{7, 17}, Text: " why ", Block: true},
			},
		},
		"a line comment inside a note, among its lines": {
			source: "@startuml\nnote as n\n  ' first\n  assume x\n  ' last\nend note\n@enduml\n",
			want: []Element{
				&NoteSyntax{Pos: Pos{2, 1}, ID: "n", Lines: []Element{
					&CommentSyntax{Pos: Pos{3, 3}, Text: " first"},
					&AssumeSyntax{Pos: Pos{4, 3}, Text: "x"},
					&CommentSyntax{Pos: Pos{5, 3}, Text: " last"},
				}},
			},
		},
		"a column counted in code points": {
			source: "@startuml\n状態 --> 状態 : E\n@enduml\n",
			want: []Element{
				&EdgeSyntax{Pos: Pos{2, 1}, Src: "状態", Dst: "状態", Event: EventSyntax{Pos: Pos{2, 13}, Text: "E", Channel: "E"}},
			},
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Act
			tree, err := Parse("", tt.source)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tt.want, tree.Body); diff != "" {
				t.Error(diff)
			}
		})
	}
}

// The JSON of the syntax tree of every example reads back into a tree whose
// JSON is the same.
func TestUnmarshalSyntaxTreeReadsBackEveryExample(t *testing.T) {
	err := filepath.WalkDir("../examples", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "invalid" {
			return filepath.SkipDir
		}
		if d.IsDir() || filepath.Ext(path) != ".puml" {
			return nil
		}
		t.Run(path, func(t *testing.T) {
			// Arrange
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			tree, err := Parse(path, string(src))
			if err != nil {
				t.Fatal(err)
			}
			want, err := MarshalSyntaxTree(tree)
			if err != nil {
				t.Fatal(err)
			}

			// Act
			back, err := UnmarshalSyntaxTree(want)

			// Assert
			if err != nil {
				t.Fatal(err)
			}
			got, err := MarshalSyntaxTree(back)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(string(want), string(got)); diff != "" {
				t.Error(diff)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUnmarshalSyntaxTreeNamesTheElementItRefuses(t *testing.T) {
	cases := map[string]struct{ body, want string }{
		"a malformed clause in an edge": {
			body: `{"kind":"edge","line":7,"col":1,"src":"a","dst":"b","event":{"line":7,"col":7,"text":"E","channel":"E","fields":[]},"guard":{"line":7,"col":11,"text":"@ne(x)"},"post":null}`,
			want: `the edge at line 7: the clause "@ne(x)": `,
		},
		"a tree in a pragma other than !include": {
			body: `{"kind":"pragma","line":2,"col":1,"name":"define","arg":"X","tree":{"kind":"csdf-syntax-tree","version":1,"name":"","line":1,"col":1,"body":[]}}`,
			want: "the pragma at line 2: !define holds a tree; only an !include does",
		},
		"an element of unknown kind": {
			body: `{"kind":"bogus","line":3,"col":1}`,
			want: "the bogus at line 3: an element of unknown kind",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			// Arrange
			raw := `{"kind":"csdf-syntax-tree","version":1,"name":"","line":1,"col":1,"body":[` + c.body + `]}`

			// Act
			_, err := UnmarshalSyntaxTree([]byte(raw))

			// Assert
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Errorf("got %v, want an error containing %q", err, c.want)
			}
		})
	}
}

// A resolved !include carries the tree of the file it names, written in
// full, and reads back with it.
func TestUnmarshalSyntaxTreeReadsBackAnIncludedTree(t *testing.T) {
	// Arrange
	included := &SyntaxTree{Pos: Pos{1, 1}, Path: "local/A.puml", Name: "A", Body: []Element{
		&StateSyntax{Pos: Pos{2, 1}, ID: "s0", Label: "S"},
	}}
	tree := &SyntaxTree{Pos: Pos{1, 1}, Name: "G", Body: []Element{
		&PragmaSyntax{Pos: Pos{2, 3}, Name: "include", Arg: "local/A.puml", Tree: included},
	}}
	want, err := MarshalSyntaxTree(tree)
	if err != nil {
		t.Fatal(err)
	}

	// Act
	back, err := UnmarshalSyntaxTree(want)

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(tree, back); diff != "" {
		t.Error(diff)
	}
	if !strings.Contains(string(want), `"arg":"local/A.puml","tree":{"kind":"csdf-syntax-tree","version":1,"path":"local/A.puml"`) {
		t.Errorf("want the included tree after the path written, got %s", want)
	}
}

func TestPlainRefusesAGlobalDiagramWithTheHint(t *testing.T) {
	// Arrange
	tree, err := Parse("", "@startuml\n!include x.puml\n@enduml\n")
	if err != nil {
		t.Fatal(err)
	}

	// Act
	_, err = Plain(tree)

	// Assert
	if err == nil || !strings.Contains(err.Error(), "holds promotion directives") {
		t.Errorf("Plain() error = %v, want the promotion hint", err)
	}
}

// The hint is for a refusal of what a global diagram has; a mistake in a
// diagram that also holds directives is the refusal alone.
func TestPlainDoesNotHintAtAMistakeOfAGlobalDiagram(t *testing.T) {
	// Arrange
	tree, err := Parse("", "@startuml\ns : x\n!include x.puml\n@enduml\n")
	if err != nil {
		t.Fatal(err)
	}

	// Act
	_, err = Plain(tree)

	// Assert
	var hinted *PromotionHintError
	if err == nil || errors.As(err, &hinted) {
		t.Errorf("Plain() error = %v, want a refusal of the variable without the hint", err)
	}
}

func TestHoldsDirectivesReadsTheSyntaxTree(t *testing.T) {
	tests := map[string]struct {
		source string
		want   bool
	}{
		"an !include":            {source: "@startuml\n!include x.puml\n@enduml\n", want: true},
		"a sync note":            {source: "@startuml\nnote as n\n  sync E : m(i)\nend note\n@enduml\n", want: true},
		"a <<promote>> block":    {source: "@startuml\nstate \"S\" as s {\n  state \"m : (i : I) ⇸ T\" as b <<promote>>\n}\n@enduml\n", want: true},
		"an !include ignored":    {source: "@startuml\n' CSDF-IGNORE-BEGIN\n!include x.puml\n' CSDF-IGNORE-END\n@enduml\n", want: false},
		"a note for the picture": {source: "@startuml\nnote left of s\n  for the picture, sync\nend note\n@enduml\n", want: false},
		"a diagram without any":  {source: "@startuml\n[*] --> s\n@enduml\n", want: false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			// Act
			tree, err := Parse("", tt.source)
			if err != nil {
				t.Fatal(err)
			}
			got := HoldsDirectives(tree.Body)

			// Assert
			if got != tt.want {
				t.Errorf("HoldsDirectives() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBaseDirOfTree(t *testing.T) {
	testCases := map[string]struct {
		Path     string
		Expected string
	}{
		"a tree parsed from a file":                {Path: "path/to/G.puml", Expected: "path/to"},
		"a file in the current directory":          {Path: "G.puml", Expected: "."},
		"no path, e.g. parsed from standard input": {Path: "", Expected: "."},
	}
	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			if got := BaseDirOfTree(&SyntaxTree{Path: testCase.Path}); got != testCase.Expected {
				t.Errorf("want %q, got %q", testCase.Expected, got)
			}
		})
	}
}
