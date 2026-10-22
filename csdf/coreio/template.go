package coreio

import (
	"fmt"
	"strings"
	"text/template"

	"github.com/Kuniwak/puml-parallel/csdf/clause"
	"github.com/Kuniwak/puml-parallel/csdf/core"
)

// Templates gloss the builtins of a formula promotion for a reader
// (docs/PROMOTION.md, Wording): a template is named after a builtin word
// and given the builtin's terms as .Args and, for @at, its state as .State.
// A gloss is a line comment, so it never replaces the builtin it glosses
// (docs/CLAUSE_DESIGN.md).
type Templates struct {
	set *template.Template
}

// glossed is what a template is given.
type glossed struct {
	Args  []string
	State string
}

// LoadTemplates reads a Go text/template file of templates named after
// builtin words. A template of any other name, such as one csdfpromote
// reads, is refused, and so is one that cannot render or writes a line
// break, each being rendered once here.
func LoadTemplates(text string) (Templates, error) {
	set, err := template.New("").Option("missingkey=error").Parse(text)
	if err != nil {
		return Templates{}, fmt.Errorf("coreio.LoadTemplates: %w", err)
	}
	for _, t := range set.Templates() {
		name := t.Name()
		if name == "" {
			continue
		}
		spec, ok := clause.Op(name).Spec()
		if !ok {
			return Templates{}, fmt.Errorf("coreio.LoadTemplates: %s is no builtin word; csdfpromote's templates are not the formula promotion's", name)
		}
		sample := glossed{Args: make([]string, spec.Terms), State: "S"}
		for i := range sample.Args {
			sample.Args[i] = fmt.Sprintf("t%d", i)
		}
		var w strings.Builder
		if err := t.Execute(&w, sample); err != nil {
			return Templates{}, fmt.Errorf("coreio.LoadTemplates: %s cannot render: %w", name, err)
		}
		if strings.ContainsAny(w.String(), "\r\n") {
			return Templates{}, fmt.Errorf("coreio.LoadTemplates: %s writes a line break, which would end its line comment", name)
		}
	}
	return Templates{set: set}, nil
}

// Gloss is the gloss of e's guard and post: each builtin conjunct that
// has a template, rendered, joined by ∧; "" when none has one.
func (ts Templates) Gloss(e core.Edge) (string, error) {
	var out []string
	for _, f := range []clause.And{e.Guard, e.Post} {
		for _, c := range clause.Normalize(f).Conjuncts {
			b, ok := c.(clause.Builtin)
			if !ok {
				continue
			}
			t := ts.set.Lookup(string(b.Op))
			if t == nil {
				continue
			}
			g := glossed{State: b.State}
			for _, a := range b.Args {
				s, err := clause.PrintTerm(a)
				if err != nil {
					return "", err
				}
				g.Args = append(g.Args, s)
			}
			var w strings.Builder
			if err := t.Execute(&w, g); err != nil {
				return "", err
			}
			out = append(out, w.String())
		}
	}
	return strings.Join(out, " ∧ "), nil
}

// WriteGlossed is Write with the gloss ts give of each edge written as a
// line comment above it; Read skips it, so Read(WriteGlossed(d, ts)) is d
// as Read(Write(d)) is.
func WriteGlossed(d *core.Diagram, ts Templates) (string, error) {
	return write(d, ts.Gloss)
}
