package csdfpromotecmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/core"
	"github.com/Kuniwak/puml-parallel/csdf/coreio"
	"github.com/Kuniwak/puml-parallel/csdf/promote"
	"github.com/Kuniwak/puml-parallel/tools"
	"github.com/Kuniwak/puml-parallel/version"
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
		// -json prints the directives as csdf/promote parses them.
		if opts.JSON {
			return mainJSON(opts, inout)
		}
		return mainFormula(opts, inout)
	}
}

// mainFormula expands the promotion with csdf/core and prints it as PUML.
func mainFormula(opts *Options, inout *cli.ProcInout) error {
	prom, err := coreio.ReadPromotionBytes(opts.Bytes, coreio.FileIncluder(opts.Base))
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

	d, warnings, err := core.PromoteOnly(prom, opts.Only)
	if err != nil {
		return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", err)
	}
	if err := report(inout.Stderr, warnings, opts.Werror); err != nil {
		return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", err)
	}
	if opts.LintOnly {
		return nil
	}

	d.Name = tools.GeneratedBy("csdfpromote", opts.Args)
	canonical, err := coreio.Canonical(d)
	if err != nil {
		return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", err)
	}
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

// mainJSON prints the directives as csdf/promote parses them (-json).
func mainJSON(opts *Options, inout *cli.ProcInout) error {
	global, err := promote.ParseGlobalBytes(opts.Bytes)
	if err != nil {
		return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", err)
	}
	if err := json.NewEncoder(inout.Stdout).Encode(global); err != nil {
		return fmt.Errorf("csdfpromotecmd.NewMainFunc: writing JSON: %w", err)
	}
	return nil
}
