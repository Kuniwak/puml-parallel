package coreio

import (
	"fmt"
	"path/filepath"

	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/pngsrc"
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
	return func(p string) (string, error) {
		src, ok := sources[p]
		if !ok {
			return "", fmt.Errorf("coreio.SourcesIncluder: %s is not among the included files given", p)
		}
		return src, nil
	}
}

// RecordingIncluder is include, recording in sources what it includes by
// the path written.
func RecordingIncluder(include Include, sources map[string]string) Include {
	return func(p string) (string, error) {
		src, err := include(p)
		if err != nil {
			return "", err
		}
		sources[p] = src
		return src, nil
	}
}

// FileIncluder includes a file by the path an !include writes, relative to
// dir unless it is absolute: a .puml file, or a .png with its source
// embedded.
func FileIncluder(dir string) Include {
	return func(p string) (string, error) {
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		return pngsrc.ReadFile(p)
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

// ReadPromotionBytes reads the global diagram in raw, a .puml text or a
// .png, as ReadPromotion does.
func ReadPromotionBytes(raw []byte, include Include) (core.Promotion, error) {
	src, err := pngsrc.Extract(raw)
	if err != nil {
		return core.Promotion{}, fmt.Errorf("coreio.ReadPromotionBytes: reading PlantUML source: %w", err)
	}
	prom, err := ReadPromotion(src, include)
	if err != nil {
		return core.Promotion{}, fmt.Errorf("coreio.ReadPromotionBytes: %w", err)
	}
	return prom, nil
}

// ReadExpandedBytes reads the diagram in raw, a .puml text or a .png, as
// ReadBytes does, unless it holds promotion directives
// (csdf.HoldsPromotionDirectives): then it is the expansion csdfpromote
// prints for it (Expand), each !include read by include, with the warnings
// of the promotion. A diagram without promotion directives reads as
// ReadBytes reads it, with no warnings.
func ReadExpandedBytes(raw []byte, include Include) (*core.Diagram, []core.Warning, error) {
	src, err := pngsrc.Extract(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("coreio.ReadExpandedBytes: reading PlantUML source: %w", err)
	}
	if !csdf.HoldsPromotionDirectives(src) {
		d, err := ReadBytes(raw)
		if err != nil {
			return nil, nil, fmt.Errorf("coreio.ReadExpandedBytes: %w", err)
		}
		return d, nil, nil
	}
	prom, err := ReadPromotionBytes(raw, include)
	if err != nil {
		return nil, nil, fmt.Errorf("coreio.ReadExpandedBytes: %w", err)
	}
	expanded, warnings, err := Expand(prom, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("coreio.ReadExpandedBytes: %w", err)
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
