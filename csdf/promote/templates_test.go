package promote_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kuniwak/puml-parallel/csdf/promote"
)

func TestExpandRendersTheClausesWithTheGivenTemplates(t *testing.T) {
	tmpl, err := promote.LoadTemplates("../../examples/promote/templates/ja.tmpl")
	if err != nil {
		t.Fatalf("promote.LoadTemplates() error = %v", err)
	}

	g, err := promote.ParseGlobal(syncGlobal)
	if err != nil {
		t.Fatalf("promote.ParseGlobal() error = %v", err)
	}

	x, diags := promote.Expand(g, promote.MapLoader(syncLocals), tmpl)
	if x == nil {
		t.Fatalf("promote.Expand() expansion = nil, diagnostics = %v", diags)
	}

	want := "running --> running : SETTLE?約定ID ; 約定ID は buys にある ∧ buys の 約定ID は〈記帳済み〉 ; buys から 約定ID を除く ∧ cycles は変わらない"
	if got := x.String(); !strings.Contains(got, want) {
		t.Errorf("promote.Expand() missing line:\n%s\ngot:\n%s", want, got)
	}
}

// A move template names the map before and after the move by Before and After.
func TestExpandRendersAMoveWithTheMapBeforeAndAfterIt(t *testing.T) {
	// Setup
	path := filepath.Join(t.TempDir(), "move.tmpl")
	if err := os.WriteFile(path, []byte(`{{define "delete"}}{{.After}} は {{.Before}} から {{.ID}} を除いたもの{{end}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	tmpl, err := promote.LoadTemplates(path)
	if err != nil {
		t.Fatalf("promote.LoadTemplates() error = %v", err)
	}
	g, err := promote.ParseGlobal(syncGlobal)
	if err != nil {
		t.Fatalf("promote.ParseGlobal() error = %v", err)
	}

	// Execute
	x, diags := promote.Expand(g, promote.MapLoader(syncLocals), tmpl)

	// Assert
	if x == nil {
		t.Fatalf("promote.Expand() expansion = nil, diagnostics = %v", diags)
	}
	if want := "buys' は buys から 約定ID を除いたもの"; !strings.Contains(x.String(), want) {
		t.Errorf("promote.Expand() missing %q:\n%s", want, x.String())
	}
}

func TestLoadTemplatesRefusesOneThatCannotRender(t *testing.T) {
	if _, err := promote.LoadTemplates("../../examples/promote/testdata/broken.tmpl"); err == nil {
		t.Fatal("promote.LoadTemplates() error = nil, want a refusal")
	}
}

// The connective between clauses is grammar, not wording: a template that tries
// to change it would hide the clause boundaries from every tool downstream.
func TestLoadTemplatesRefusesOneThatRedefinesTheConnective(t *testing.T) {
	_, err := promote.LoadTemplates("../../examples/promote/testdata/and.tmpl")
	if err == nil {
		t.Fatal("promote.LoadTemplates() error = nil, want a refusal of the \"and\" template")
	}
	if !strings.Contains(err.Error(), "and") {
		t.Errorf("promote.LoadTemplates() error = %v, want it to name the \"and\" template", err)
	}
}
