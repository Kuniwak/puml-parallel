package csdfreplwebcmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/Kuniwak/puml-parallel/v3/csdf/animation"
	"github.com/Kuniwak/puml-parallel/v3/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/v3/pngsrc"
	"github.com/Kuniwak/puml-parallel/v3/tools/replweb"
	"github.com/Kuniwak/puml-parallel/v3/version"
)

// RenderSVG runs argv in dir with source on its standard input and returns
// the SVG it writes.
type RenderSVG func(argv []string, dir string, source []byte) ([]byte, error)

// RunPlantUML is the RenderSVG that runs the command, with no shell.
func RunPlantUML(argv []string, dir string, source []byte) ([]byte, error) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(source)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("csdfreplwebcmd.RunPlantUML: %s failed: %w: %s", strings.Join(argv, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

func NewMainFunc() cli.MainFunc[*Options] {
	return newMainFunc(RunPlantUML)
}

func newMainFunc(render RenderSVG) cli.MainFunc[*Options] {
	return func(opts *Options, inout *cli.ProcInout) error {
		if opts.Common.Help {
			return nil
		}
		if opts.Common.Version {
			fmt.Fprintln(inout.Stdout, version.Version)
			return nil
		}

		raw, err := os.ReadFile(opts.File)
		if err != nil {
			return fmt.Errorf("csdfreplwebcmd: cannot read the diagram: %w", err)
		}
		source, err := pngsrc.Extract(raw)
		if err != nil {
			return fmt.Errorf("csdfreplwebcmd: %q: %w", opts.File, err)
		}
		dir := filepath.Dir(opts.File)
		includes := map[string]string{}
		d, _, err := animation.ReadDiagram([]byte(source), coreio.RecordingIncluder(coreio.FileIncluder(dir), includes))
		if err != nil {
			return fmt.Errorf("csdfreplwebcmd: %q: %w", opts.File, err)
		}
		svg, err := render(opts.PumlExec, dir, []byte(source))
		if err != nil {
			return err
		}
		highlights, err := replweb.MapSVG(svg, d)
		if err != nil {
			return fmt.Errorf("csdfreplwebcmd: %w", err)
		}
		name := filepath.Base(opts.File)
		title := d.Name
		if title == "" {
			title = strings.TrimSuffix(name, filepath.Ext(name))
		}
		return replweb.WriteStaticPage(inout.Stdout, replweb.Page{
			Title: title, Path: name, Diagram: []byte(source), Includes: includes, SVG: svg, Highlights: highlights,
		})
	}
}
