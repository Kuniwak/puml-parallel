package csdfreplwebcmd

import (
	"bytes"
	"errors"
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
	"github.com/Kuniwak/puml-parallel/v3/tools/replweb/engineload"
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

// ChooseEngine gives the Source of the engine the page of opts carries.
type ChooseEngine func(opts *Options) engineload.Source

func NewMainFunc() cli.MainFunc[*Options] {
	return newMainFunc(RunPlantUML, func(opts *Options) engineload.Source {
		builder := engineload.Builder{Go: engineload.ExecGo{}, Module: engineload.ThisModule()}
		builder.WorkDir, _ = os.Getwd()
		if dir, err := os.UserCacheDir(); err == nil {
			builder.CacheDir = filepath.Join(dir, "csdfreplweb")
		}
		return engineload.Choose(opts.Engine, engineload.Embedded(), builder.Source())
	})
}

// noGoError says what to do when the engine cannot be built for want of Go.
type noGoError struct{ err error }

func (e noGoError) Error() string {
	return "the engine of the page is built with the go command, and there is none on the PATH: " +
		"install Go, use a release binary of csdfreplweb, which embeds the engine, " +
		"or give -engine path/to/engine.wasm.gz (attached to every release)"
}
func (e noGoError) Unwrap() error { return e.err }
func (noGoError) UserFacing()     {}

func newMainFunc(render RenderSVG, chooseEngine ChooseEngine) cli.MainFunc[*Options] {
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
		engine, err := chooseEngine(opts)()
		if errors.Is(err, engineload.ErrNoGo) {
			return noGoError{err}
		}
		if err != nil {
			return err
		}
		python, err := pythonOf(opts)
		if err != nil {
			return err
		}
		return replweb.WriteStaticPage(inout.Stdout, replweb.Page{
			Title: title, Path: name, Diagram: []byte(source), Includes: includes, SVG: svg, Highlights: highlights,
			Python: python,
		}, engine)
	}
}

// pythonOf is what the page's Python reads to suggest values, or nil when
// opts give no checker: the checks module, and every word after the module
// of -evaluator-module that names a file, such as the definitions
// csdfbmc.python_glue reads.
func pythonOf(opts *Options) (*replweb.Python, error) {
	if !opts.Checker.Asks() {
		return nil, nil
	}
	checks, err := os.ReadFile(opts.Checker.Checks)
	if err != nil {
		return nil, fmt.Errorf("csdfreplwebcmd: cannot read the checks module: %w", err)
	}
	files := map[string][]byte{opts.Checker.Checks: checks}
	words, err := SplitCommandLine(opts.Checker.EvaluatorModule)
	if err != nil {
		return nil, fmt.Errorf("csdfreplwebcmd: -evaluator-module: %w", err)
	}
	for _, word := range words[min(1, len(words)):] {
		if info, err := os.Stat(word); err != nil || !info.Mode().IsRegular() {
			continue
		}
		bs, err := os.ReadFile(word)
		if err != nil {
			return nil, fmt.Errorf("csdfreplwebcmd: cannot read %q of -evaluator-module: %w", word, err)
		}
		files[word] = bs
	}
	return &replweb.Python{URL: opts.Pyodide, Checker: opts.Checker, Files: files}, nil
}
