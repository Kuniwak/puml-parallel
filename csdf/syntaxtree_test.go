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
			if _, err := Parse(string(src)); err != nil {
				t.Error(err)
			}
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestParseKeepsCommentsPragmasAndIgnoredRegions(t *testing.T) {
	// Act
	tree, err := Parse("@startuml\n' CSDF-IGNORE-BEGIN\n  title T\n' CSDF-IGNORE-END\n' CSDF-OBSERVE v = s0\n!include x.puml\nstate \"S\" as s0 /' why '/\n@enduml\n")

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	want := []Element{
		&IgnoreSyntax{Pos: Pos{2, 1}, Lines: []string{"  title T"}},
		&ObserveSyntax{Pos: Pos{5, 1}, Name: "v", View: true, States: []string{"s0"}},
		&PragmaSyntax{Pos: Pos{6, 1}, Name: "include", Arg: "x.puml"},
		&CommentSyntax{Pos: Pos{7, 17}, Text: " why ", Block: true},
		&StateSyntax{Pos: Pos{7, 1}, ID: "s0", Label: "S"},
	}
	if diff := cmp.Diff(want, tree.Body); diff != "" {
		t.Error(diff)
	}
}

func TestParseCountsColumnsInCodePoints(t *testing.T) {
	// Act
	tree, err := Parse("@startuml\n状態 --> 状態 : E\n@enduml\n")

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	e := tree.Body[0].(*EdgeSyntax)
	if e.Event.Col != 13 {
		t.Errorf("the event's column = %d, want 13", e.Event.Col)
	}
}

func TestPlainRefusesAGlobalDiagramWithTheHint(t *testing.T) {
	// Arrange
	tree, err := Parse("@startuml\n!include x.puml\n@enduml\n")
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
