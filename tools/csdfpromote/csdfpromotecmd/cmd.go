package csdfpromotecmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/Kuniwak/puml-parallel/v4/cli"
	"github.com/Kuniwak/puml-parallel/v4/csdf"
	"github.com/Kuniwak/puml-parallel/v4/csdf/core"
	"github.com/Kuniwak/puml-parallel/v4/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/v4/pngsrc"
	"github.com/Kuniwak/puml-parallel/v4/tools"
	"github.com/Kuniwak/puml-parallel/v4/version"
)

func NewMainFunc() cli.MainFunc[*Options] {
	return func(opts *Options, inout *cli.ProcInout) error {
		if opts.Common.Help {
			return nil
		}
		if opts.Common.Version {
			fmt.Fprintln(inout.Stdout, version.Version)
			return nil
		}
		return mainFormula(opts, inout)
	}
}

// mainFormula expands the promotion with csdf/core and prints it as PUML.
func mainFormula(opts *Options, inout *cli.ProcInout) error {
	tree, err := readTree(opts.Bytes)
	if err != nil {
		return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", err)
	}
	var templates *coreio.Templates
	if opts.Template != "" && opts.Output == OutputJSON {
		return errors.New("csdfpromotecmd.NewMainFunc: -template glosses the PUML output; -format json takes none")
	}
	if opts.Template != "" {
		text, err := os.ReadFile(opts.Template)
		if err != nil {
			return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", err)
		}
		loaded, err := coreio.LoadTemplates(string(text))
		if err != nil {
			return fmt.Errorf("csdfpromotecmd.NewMainFunc: %s: %w", opts.Template, err)
		}
		templates = &loaded
	}

	canonical, warnings, err := coreio.ExpandTree(tree, coreio.FileIncluder(opts.Base), opts.Only)
	if err != nil {
		return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", err)
	}
	if err := report(inout.Stderr, warnings, opts.Werror); err != nil {
		return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", err)
	}
	if opts.LintOnly {
		return nil
	}

	canonical.Name = tools.GeneratedBy("csdfpromote", opts.Args)
	if opts.Output == OutputJSON {
		raw, err := core.MarshalDiagram(canonical)
		if err != nil {
			return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", err)
		}
		fmt.Fprintln(inout.Stdout, string(raw))
		return nil
	}
	var text string
	if templates != nil {
		text, err = coreio.WriteGlossed(canonical, *templates)
	} else {
		text, err = coreio.Write(canonical)
	}
	if err != nil {
		return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", err)
	}
	fmt.Fprint(inout.Stdout, text)
	return nil
}

// readTree reads the syntax tree of the diagram in raw: the JSON csdfparse
// prints, or a .puml text or a .png, parsed.
func readTree(raw []byte) (*csdf.SyntaxTree, error) {
	if isJSON(raw) {
		return csdf.UnmarshalSyntaxTree(raw)
	}
	src, err := pngsrc.Extract(raw)
	if err != nil {
		return nil, fmt.Errorf("reading PlantUML source: %w", err)
	}
	return csdf.Parse("", src)
}

// isJSON reports whether raw is JSON rather than PlantUML: it opens with
// "{", which no PlantUML source or PNG does.
func isJSON(raw []byte) bool {
	return bytes.HasPrefix(bytes.TrimSpace(raw), []byte("{"))
}

// report writes each warning on w; with werror, one of level Warn is an
// error.
func report(w io.Writer, warnings []core.Warning, werror bool) error {
	warned := false
	for _, x := range warnings {
		fmt.Fprintln(w, x)
		warned = warned || x.Level == core.Warn
	}
	if werror && warned {
		return errors.New("the promotion has warnings and -Werror is set")
	}
	return nil
}
