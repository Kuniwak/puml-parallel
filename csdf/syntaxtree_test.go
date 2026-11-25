package csdf

import (
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
				&CommentSyntax{Pos: Pos{7, 17}, Text: " why ", Block: true},
				&StateSyntax{Pos: Pos{7, 1}, ID: "s0", Label: "S"},
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
