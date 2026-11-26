package coreio

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/v5/csdf"
	"github.com/Kuniwak/puml-parallel/v5/csdf/pumlline"
)

// ResolveIncludes is tree with each !include resolved: the pragma, its
// path kept as written, holds the syntax tree of the file it names
// (csdf.PragmaSyntax.Tree), read by include and parsed, its own !include
// lines resolved in turn. include is given each file by its path relative
// to base, the directory include reads against: an !include of the
// outermost tree as written, one in an included file joined to that
// file's directory. The Path of an included tree is the file read, base
// joined to that path, or the path itself when it is absolute. A cycle of
// !include is refused. The tree given is left as it is.
func ResolveIncludes(tree *csdf.SyntaxTree, base string, include Include) (*csdf.SyntaxTree, error) {
	r := &includeResolver{base: base, include: include}
	body, err := r.body(tree.Body, "")
	if err != nil {
		return nil, fmt.Errorf("coreio.ResolveIncludes: %w", err)
	}
	out := *tree
	out.Body = body
	return &out, nil
}

// includeResolver resolves the !include lines of a tree, keeping the
// files it is within by the path include is given.
type includeResolver struct {
	base    string
	include Include
	within  []string
}

// body is es with each !include resolved; dir is the directory, relative
// to base, of the included file es is in, "" for the outermost tree.
func (r *includeResolver) body(es []csdf.Element, dir string) ([]csdf.Element, error) {
	out := make([]csdf.Element, 0, len(es))
	for _, e := range es {
		switch e := e.(type) {
		case *csdf.StateSyntax:
			inner, err := r.body(e.Body, dir)
			if err != nil {
				return nil, err
			}
			s := *e
			if e.Body != nil {
				s.Body = inner
			}
			out = append(out, &s)
		case *csdf.PragmaSyntax:
			if e.Name != "include" {
				out = append(out, e)
				continue
			}
			p, err := r.pragma(e, dir)
			if err != nil {
				return nil, err
			}
			out = append(out, p)
		default:
			out = append(out, e)
		}
	}
	return out, nil
}

func (r *includeResolver) pragma(p *csdf.PragmaSyntax, dir string) (*csdf.PragmaSyntax, error) {
	key := pumlline.Unquote(p.Arg)
	path := key
	if !filepath.IsAbs(key) {
		if dir != "" {
			key = filepath.Join(dir, key)
		}
		path = filepath.Join(r.base, key)
	}
	if slices.Contains(r.within, key) {
		return nil, fmt.Errorf("line %d: !include %s: a cycle of !include: %s", p.Line, p.Arg, strings.Join(append(slices.Clone(r.within), key), ", "))
	}
	src, err := r.include(key)
	if err != nil {
		return nil, fmt.Errorf("line %d: !include %s: %w", p.Line, p.Arg, err)
	}
	tree, err := csdf.Parse(path, src)
	if err != nil {
		return nil, fmt.Errorf("line %d: !include %s: %s: %w", p.Line, p.Arg, path, err)
	}
	r.within = append(r.within, key)
	body, err := r.body(tree.Body, filepath.Dir(key))
	r.within = r.within[:len(r.within)-1]
	if err != nil {
		return nil, fmt.Errorf("line %d: !include %s: %s: %w", p.Line, p.Arg, path, err)
	}
	tree.Body = body
	out := *p
	out.Tree = tree
	return &out, nil
}
