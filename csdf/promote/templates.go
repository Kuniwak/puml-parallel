package promote

import (
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/Kuniwak/puml-parallel/csdf"
)

// Clause is what a clause template is rendered with. Src and Dst are the names
// of the local states the edge runs between, and Var is the state variable a
// frame clause is about; each template uses the fields its own clause needs.
// Before and After name the map before and after a move.
type Clause struct {
	Map    string
	ID     string
	Src    string
	Dst    string
	Var    string
	Before string
	After  string
}

// The clauses the expansion generates, each replaceable on its own. What joins
// the clauses of one predicate is not among them: the connective is grammar
// (csdf.ClauseConnective), and a template that changed it would hide the clause
// boundaries from every tool downstream.
const (
	clauseInDom    = "inDom"
	clauseNotInDom = "notInDom"
	clauseAtState  = "atState"
	clauseUpdate   = "update"
	clauseCreate   = "create"
	clauseDelete   = "delete"
	clauseFrame    = "frame"
	clauseEmpty    = "empty"
)

// connectiveTemplate is the name an older template file gave the connective,
// which LoadTemplates refuses.
const connectiveTemplate = "and"

// defaultTemplates is the symbolic wording, which is the default because it does
// not belong to any one natural language. A -template file replaces the clauses
// it names and leaves the rest of these in place.
const defaultTemplates = `
{{define "inDom"}}{{.ID}} ∈ dom {{.Map}}{{end}}
{{define "notInDom"}}{{.ID}} ∉ dom {{.Map}}{{end}}
{{define "atState"}}{{.Map}}({{.ID}}) ∈ 〈{{.Src}}〉{{end}}
{{define "update"}}{{.After}} = {{.Before}} ⊕ { {{- .ID}} ↦ 〈{{.Dst}}〉}{{end}}
{{define "create"}}{{.After}} = {{.Before}} ∪ { {{- .ID}} ↦ 〈{{.Dst}}〉}{{end}}
{{define "delete"}}{{.After}} = { {{- .ID -}} } ⩤ {{.Before}}{{end}}
{{define "frame"}}{{.Var}}' = {{.Var}}{{end}}
{{define "empty"}}{{.Map}} = ∅{{end}}
`

// Templates is the wording of the generated clauses.
type Templates struct{ t *template.Template }

// DefaultTemplates returns the symbolic wording.
func DefaultTemplates() *Templates {
	return &Templates{t: template.Must(template.New("promote").Parse(defaultTemplates))}
}

// LoadTemplates reads a template file over the defaults. A clause the file does
// not define keeps its symbolic wording.
//
// Every clause is rendered once here, so a template that cannot render is a
// refusal rather than an opaque predicate in the output.
func LoadTemplates(path string) (*Templates, error) {
	bs, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("promote.LoadTemplates: cannot read file: %w: %q", err, path)
	}

	t, err := DefaultTemplates().t.Clone()
	if err != nil {
		return nil, fmt.Errorf("promote.LoadTemplates: %w", err)
	}
	if _, err := t.Parse(string(bs)); err != nil {
		return nil, fmt.Errorf("promote.LoadTemplates: %q: %w", path, err)
	}
	if t.Lookup(connectiveTemplate) != nil {
		return nil, fmt.Errorf("promote.LoadTemplates: %q: the %q template is not accepted: the clauses are joined by %q, which is grammar rather than wording", path, connectiveTemplate, csdf.ClauseConnective)
	}

	ts := &Templates{t: t}
	probe := Clause{Map: "m", ID: "id", Src: "S", Dst: "T", Var: "v", Before: "m", After: "m'"}
	for _, name := range []string{clauseInDom, clauseNotInDom, clauseAtState, clauseUpdate, clauseCreate, clauseDelete, clauseFrame, clauseEmpty} {
		if _, err := ts.render(name, probe); err != nil {
			return nil, fmt.Errorf("promote.LoadTemplates: %q: %w", path, err)
		}
	}
	return ts, nil
}

// clause renders one clause. A template that fails here has already been
// rendered once by LoadTemplates, so the error is reported rather than hidden
// inside a predicate.
func (ts *Templates) clause(name string, c Clause) string {
	out, err := ts.render(name, c)
	if err != nil {
		panic(fmt.Errorf("promote: rendering the %q clause: %w", name, err))
	}
	return out
}

func (ts *Templates) render(name string, c Clause) (string, error) {
	var sb strings.Builder
	if err := ts.t.ExecuteTemplate(&sb, name, c); err != nil {
		return "", err
	}
	return sb.String(), nil
}
