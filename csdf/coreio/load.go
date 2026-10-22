package coreio

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/pngsrc"
)

// LoadAuthored reads the diagram an author wrote at path, a .puml file or a
// .png PlantUML rendered with its source embedded.
func LoadAuthored(path string) (*core.Diagram, error) {
	src, err := source(path)
	if err != nil {
		return nil, fmt.Errorf("coreio.LoadAuthored: %w", err)
	}
	d, err := ReadAuthored(src)
	if err != nil {
		return nil, fmt.Errorf("coreio.LoadAuthored: %s: %w", path, err)
	}
	return d, nil
}

// LoadPromotion reads the global diagram at path, each !include read
// relative to the directory path is in.
func LoadPromotion(path string) (core.Promotion, error) {
	src, err := source(path)
	if err != nil {
		return core.Promotion{}, fmt.Errorf("coreio.LoadPromotion: %w", err)
	}
	dir := filepath.Dir(path)
	include := func(p string) (string, error) {
		if !filepath.IsAbs(p) {
			p = filepath.Join(dir, p)
		}
		return source(p)
	}
	prom, err := ReadPromotion(src, include)
	if err != nil {
		return core.Promotion{}, fmt.Errorf("coreio.LoadPromotion: %s: %w", path, err)
	}
	return prom, nil
}

// source is the PlantUML source of the file at path.
func source(path string) (string, error) {
	bs, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	src, err := pngsrc.Extract(bs)
	if err != nil {
		return "", fmt.Errorf("%s: reading PlantUML source: %w", path, err)
	}
	return src, nil
}
