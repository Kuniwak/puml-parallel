package coreio

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Kuniwak/puml-parallel/v9/csdf"
	"github.com/Kuniwak/puml-parallel/v9/csdf/core"
	"github.com/Kuniwak/puml-parallel/v9/pngsrc"
)

// LoadAuthored reads the diagram an author wrote at path, a .puml file or a
// .png PlantUML rendered with its source embedded.
func LoadAuthored(path string) (*core.Diagram, error) {
	src, err := pngsrc.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("coreio.LoadAuthored: %w", err)
	}
	d, err := ReadAuthored(src)
	if err != nil {
		return nil, fmt.Errorf("coreio.LoadAuthored: %s: %w", path, err)
	}
	return d, nil
}

// Load reads the diagram a tool wrote at path, such as one csdfparallel or
// csdfhide printed, whose clauses may hold ∃; a diagram an author wrote is
// read too.
func Load(path string) (*core.Diagram, error) {
	src, err := pngsrc.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("coreio.Load: %w", err)
	}
	d, err := Read(src)
	if err != nil {
		return nil, fmt.Errorf("coreio.Load: %s: %w", path, err)
	}
	return d, nil
}

// LoadAll loads the diagram at each of paths, as Load does, in the order
// paths gives them.
func LoadAll(paths []string) ([]*core.Diagram, error) {
	diagrams := make([]*core.Diagram, len(paths))
	for i, path := range paths {
		d, err := Load(path)
		if err != nil {
			return nil, fmt.Errorf("coreio.LoadAll: cannot parse diagrams: %w", err)
		}
		diagrams[i] = d
	}
	return diagrams, nil
}

// LoadPromotion reads the global diagram at path, each !include read
// relative to the directory path is in.
func LoadPromotion(path string) (core.Promotion, error) {
	src, err := pngsrc.ReadFile(path)
	if err != nil {
		return core.Promotion{}, fmt.Errorf("coreio.LoadPromotion: %w", err)
	}
	prom, err := ReadPromotion(src, FileIncluder(filepath.Dir(path)))
	if err != nil {
		return core.Promotion{}, fmt.Errorf("coreio.LoadPromotion: %s: %w", path, err)
	}
	return prom, nil
}

// SourcesIncluder includes a file by the path an !include writes from
// sources, which maps each such path to its source; a path it does not hold
// is refused.
func SourcesIncluder(sources map[string]string) Include {
	return func(p string) (string, string, error) {
		src, ok := sources[p]
		if !ok {
			return "", "", fmt.Errorf("coreio.SourcesIncluder: %s is not among the included files given", p)
		}
		return src, p, nil
	}
}

// RecordingIncluder is include, recording in sources what it includes by
// the path written.
func RecordingIncluder(include Include, sources map[string]string) Include {
	return func(p string) (string, string, error) {
		src, read, err := include(p)
		if err != nil {
			return "", "", err
		}
		sources[p] = src
		return src, read, nil
	}
}

// FileIncluder includes a file by the path an !include writes, relative to
// dir unless it is absolute: a .puml file, or a .png with its source
// embedded. The path read is dir joined to the path written.
func FileIncluder(dir string) Include {
	return func(p string) (string, string, error) {
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		src, err := pngsrc.ReadFile(p)
		if err != nil {
			return "", "", err
		}
		return src, p, nil
	}
}

// ReadBytes reads the diagram in raw, a .puml text or a .png PlantUML
// rendered with its source embedded, as Read does.
func ReadBytes(raw []byte) (*core.Diagram, error) {
	src, err := pngsrc.Extract(raw)
	if err != nil {
		return nil, fmt.Errorf("coreio.ReadBytes: reading PlantUML source: %w", err)
	}
	d, err := Read(src)
	if err != nil {
		return nil, fmt.Errorf("coreio.ReadBytes: %w", err)
	}
	return d, nil
}

// ReadExpandedBytes reads the diagram in raw, a .puml text or a .png, as
// ReadBytes does, unless it holds promotion directives
// (csdf.HoldsDirectives): then it is the expansion csdfpromote
// prints for it (Expand), each !include resolved by include
// (ResolveIncludes), with the warnings of the promotion. A diagram without
// promotion directives reads as ReadBytes reads it, with no warnings.
func ReadExpandedBytes(raw []byte, include Include) (*core.Diagram, []core.Warning, error) {
	src, err := pngsrc.Extract(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("coreio.ReadExpandedBytes: reading PlantUML source: %w", err)
	}
	parsed, err := csdf.Parse("", src)
	if err != nil {
		return nil, nil, fmt.Errorf("coreio.ReadExpandedBytes: %w", err)
	}
	tree, err := ResolveIncludes(parsed, include)
	if err != nil {
		return nil, nil, fmt.Errorf("coreio.ReadExpandedBytes: %w", err)
	}
	d, warnings, err := ExpandTree(tree, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("coreio.ReadExpandedBytes: %w", err)
	}
	return d, warnings, nil
}

// ExpandTree is the diagram tree says, as the tools evaluate it: Lower's
// when it holds no promotion directive (csdf.HoldsDirectives), with no
// warnings, keep then naming no map; otherwise the expansion of the global diagram it is
// (ReadPromotionTree, its !include lines resolved, then Expand with keep),
// with the warnings of the promotion.
func ExpandTree(tree *csdf.SyntaxTree, keep []string) (*core.Diagram, []core.Warning, error) {
	if !csdf.HoldsDirectives(tree.Body) {
		if len(keep) > 0 {
			return nil, nil, fmt.Errorf("coreio.ExpandTree: %s is no promoted map: the diagram holds no promotion directive", strings.Join(keep, ", "))
		}
		d, err := Lower(tree)
		if err != nil {
			return nil, nil, fmt.Errorf("coreio.ExpandTree: %w", err)
		}
		return d, nil, nil
	}
	prom, err := ReadPromotionTree(tree)
	if err != nil {
		return nil, nil, fmt.Errorf("coreio.ExpandTree: %w", err)
	}
	expanded, warnings, err := Expand(prom, keep)
	if err != nil {
		return nil, nil, fmt.Errorf("coreio.ExpandTree: %w", err)
	}
	return expanded, warnings, nil
}

// Expand is the expansion csdfpromote prints for prom: core.PromoteOnly(prom,
// keep) in its canonical order, with the warnings of the promotion; with keep
// empty every family is kept.
func Expand(prom core.Promotion, keep []string) (*core.Diagram, []core.Warning, error) {
	d, warnings, err := core.PromoteOnly(prom, keep)
	if err != nil {
		return nil, nil, fmt.Errorf("coreio.Expand: %w", err)
	}
	if err := core.TypesDeclared(d); err != nil {
		return nil, nil, fmt.Errorf("coreio.Expand: %w", err)
	}
	canonical, err := Canonical(d)
	if err != nil {
		return nil, nil, fmt.Errorf("coreio.Expand: %w", err)
	}
	return canonical, warnings, nil
}

// ReadTreeJSON reads the syntax tree csdfparse prints (JSON), after a
// UTF-8 byte order mark if any; other input, such as PlantUML, is refused,
// naming csdfparse.
func ReadTreeJSON(raw []byte) (*csdf.SyntaxTree, error) {
	raw = bytes.TrimPrefix(raw, utf8BOM)
	// After whitespace, if any, JSON opens with "{", which no PlantUML
	// source or PNG does.
	if !bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{")) {
		return nil, errors.New("coreio.ReadTreeJSON: want the syntax tree csdfparse prints (JSON), not PlantUML; run csdfparse on the diagram first")
	}
	tree, err := csdf.UnmarshalSyntaxTree(raw)
	if err != nil {
		return nil, fmt.Errorf("coreio.ReadTreeJSON: %w", err)
	}
	return tree, nil
}

// utf8BOM is the byte order mark an editor may write before UTF-8 text.
var utf8BOM = []byte("\xef\xbb\xbf")

// UnresolvedInclude is the refusal of the !include of path left unresolved
// in a syntax tree, which csdfinclude resolves.
func UnresolvedInclude(path string) error {
	return fmt.Errorf("!include %s is unresolved in the syntax tree; resolve it by csdfinclude (csdfparse x.puml | csdfinclude | csdfpromote)", path)
}
