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
	"github.com/Kuniwak/puml-parallel/pngsrc"
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
		// The JSON outputs are read by csdfbmc, which reads the text clauses
		// of csdf/promote until it moves onto csdf/core (#76, step 6).
		if opts.JSON || opts.Output == promote.OutputJSON {
			return mainJSON(opts, inout)
		}
		return mainFormula(opts, inout)
	}
}

// mainFormula expands the promotion with csdf/core and prints it as PUML.
func mainFormula(opts *Options, inout *cli.ProcInout) error {
	source, err := pngsrc.Extract(opts.Bytes)
	if err != nil {
		return fmt.Errorf("csdfpromotecmd.NewMainFunc: reading PlantUML source: %w", err)
	}
	prom, err := coreio.ReadPromotion(source, coreio.FileIncluder(opts.Base))
	if err != nil {
		return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", err)
	}
	var templates *coreio.Templates
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

// mainJSON is csdfpromote as it was before csdf/core: -json and -format
// json, read by csdfbmc.
func mainJSON(opts *Options, inout *cli.ProcInout) error {
	global, err := promote.ParseGlobalBytes(opts.Bytes)
	if err != nil {
		return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", err)
	}

	if opts.JSON {
		if err := json.NewEncoder(inout.Stdout).Encode(global); err != nil {
			return fmt.Errorf("csdfpromotecmd.NewMainFunc: writing JSON: %w", err)
		}
		return nil
	}

	if opts.Template != "" {
		return errors.New("csdfpromotecmd.NewMainFunc: -template glosses the PUML output; -format json takes none")
	}

	expansion, diags := promote.Expand(global, promote.FileLoader(opts.Base), promote.DefaultTemplates())
	for _, d := range diags {
		fmt.Fprintln(inout.Stderr, d)
	}
	// An error leaves the diagram unprinted, so an unsound expansion never
	// reaches the tools downstream.
	if err := promote.Refusal(diags, opts.Werror); err != nil {
		return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", err)
	}
	if len(opts.Only) > 0 {
		restricted, rdiags := promote.Restrict(expansion, opts.Only)
		for _, d := range rdiags {
			fmt.Fprintln(inout.Stderr, d)
		}
		if err := promote.Refusal(rdiags, opts.Werror); err != nil {
			return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", err)
		}
		expansion = restricted
	}
	if opts.LintOnly {
		return nil
	}

	expansion.Diagram.Name = tools.GeneratedBy("csdfpromote", opts.Args)
	if err := promote.Write(inout.Stdout, expansion, opts.Output, !opts.NoComments); err != nil {
		return fmt.Errorf("csdfpromotecmd.NewMainFunc: %w", err)
	}
	return nil
}
