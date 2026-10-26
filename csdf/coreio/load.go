package coreio

import (
	"fmt"
	"path/filepath"

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
