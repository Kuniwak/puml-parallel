package coreio

import (
	"fmt"
	"path/filepath"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/core"
)

// Loader reads the diagram a REFER expression of a composition tree names.
type Loader func(path string) (*core.Diagram, error)

// FileLoader loads the diagrams of a tree from the file system, as Load
// does, resolving a relative path against baseDir.
func FileLoader(baseDir string) Loader {
	return func(path string) (*core.Diagram, error) {
		if !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}
		return Load(path)
	}
}

// ComposeTree evaluates a composition tree (csdf.ParseExpr) with the
// constructors of csdf/core: a REFER is the diagram load reads, a HIDE is
// core.Hide, and an INTERFACE_PARALLEL is core.ComposeAll.
func ComposeTree(expr csdf.Expr, load Loader) (*core.Diagram, error) {
	d, err := composeTree(expr, load)
	if err != nil {
		return nil, fmt.Errorf("coreio.ComposeTree: %w", err)
	}
	return d, nil
}

func composeTree(expr csdf.Expr, load Loader) (*core.Diagram, error) {
	switch e := expr.(type) {
	case *csdf.ReferExpr:
		d, err := load(e.Path)
		if err != nil {
			return nil, fmt.Errorf("in a %s expression of %q: %w", csdf.OpRefer, e.Path, err)
		}
		return d, nil
	case *csdf.HideExpr:
		proc, err := composeTree(e.Proc, load)
		if err != nil {
			return nil, err
		}
		d, err := core.Hide(proc, e.Channels)
		if err != nil {
			return nil, fmt.Errorf("in a %s expression: %w", csdf.OpHide, err)
		}
		return d, nil
	case *csdf.InterfaceParallelExpr:
		procs := make([]*core.Diagram, 0, len(e.Procs))
		for _, p := range e.Procs {
			proc, err := composeTree(p, load)
			if err != nil {
				return nil, err
			}
			procs = append(procs, proc)
		}
		d, err := core.ComposeAll(procs, e.Sync)
		if err != nil {
			return nil, fmt.Errorf("in an %s expression: %w", csdf.OpInterfaceParallel, err)
		}
		return d, nil
	}
	return nil, fmt.Errorf("unknown process expression: %T", expr)
}
