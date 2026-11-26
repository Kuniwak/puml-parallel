package coreio

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Kuniwak/puml-parallel/v7/csdf"
	"github.com/Kuniwak/puml-parallel/v7/csdf/pumlline"
	"github.com/Kuniwak/puml-parallel/v7/usererr"
)

// ResolveIncludes is tree with each !include resolved: the pragma, its
// path kept as written, holds the syntax tree of the file it names
// (csdf.PragmaSyntax.Tree), read by include and parsed, its own !include
// lines resolved in turn. include is given an !include of the outermost
// tree as written, and one in an included file joined to that file's
// directory, as written there. The Path of an included tree is the path of
// the file include read. A cycle of !include is refused. The tree given is
// left as it is.
func ResolveIncludes(tree *csdf.SyntaxTree, include Include) (*csdf.SyntaxTree, error) {
	r := &includeResolver{include: include}
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
	include Include
	within  []string
}

// body is es with each !include resolved; dir is the directory of the
// included file es is in, as include was given it, "" for the outermost
// tree.
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
	if !filepath.IsAbs(key) && dir != "" {
		key = filepath.Join(dir, key)
	}
	if slices.Contains(r.within, key) {
		return nil, unreadInclude(p, fmt.Errorf("a cycle of !include: %s", strings.Join(append(slices.Clone(r.within), key), ", ")))
	}
	src, path, err := r.include(key)
	if err != nil {
		return nil, unreadInclude(p, err)
	}
	tree, err := csdf.Parse(path, src)
	if err != nil {
		return nil, refusedInclude(p, path, err)
	}
	r.within = append(r.within, key)
	body, err := r.body(tree.Body, filepath.Dir(key))
	r.within = r.within[:len(r.within)-1]
	if err != nil {
		return nil, refusedInclude(p, path, err)
	}
	tree.Body = body
	out := *p
	out.Tree = tree
	return &out, nil
}

// ResolveFileIncludes is ResolveIncludes reading files (FileIncluder)
// against base, such as csdf.BaseDirOfTree(tree); an empty base is
// refused.
func ResolveFileIncludes(tree *csdf.SyntaxTree, base string) (*csdf.SyntaxTree, error) {
	if base == "" {
		return nil, errors.New("coreio.ResolveFileIncludes: no base directory")
	}
	return ResolveIncludes(tree, FileIncluder(base))
}

// IncludeError is the refusal of the !include of Arg written on Line: its
// file could not be read, or Err is in In, the file it names. Its message
// is the one shown to a user, the line and the path written kept.
type IncludeError struct {
	Line int
	Arg  string
	In   string // the file Err is in, "" when the file was not read
	Err  error
}

// unreadInclude refuses p, whose file could not be read for err.
func unreadInclude(p *csdf.PragmaSyntax, err error) *IncludeError {
	return &IncludeError{Line: p.Line, Arg: p.Arg, Err: err}
}

// refusedInclude refuses p for err in in, the file it names.
func refusedInclude(p *csdf.PragmaSyntax, in string, err error) *IncludeError {
	return &IncludeError{Line: p.Line, Arg: p.Arg, In: in, Err: err}
}

func (e *IncludeError) Error() string {
	if e.In == "" {
		return fmt.Sprintf("line %d: !include %s: %s", e.Line, e.Arg, usererr.Message(e.Err))
	}
	return fmt.Sprintf("line %d: !include %s: %s: %s", e.Line, e.Arg, e.In, usererr.Message(e.Err))
}

func (e *IncludeError) Unwrap() error { return e.Err }

// UserFacing marks the message as the one to show.
func (e *IncludeError) UserFacing() {}
