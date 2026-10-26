package csdfpromotecmd

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
)

// Output is what the expansion is written as.
type Output int

const (
	// OutputPUML writes the expansion as PlantUML, which every other tool
	// reads.
	OutputPUML Output = iota
	// OutputJSON writes the expansion as csdfparse prints a diagram.
	OutputJSON
)

// The names of the Outputs, as -format takes them.
const (
	OutputNamePUML = "puml"
	OutputNameJSON = "json"
)

// ParseOutput returns the Output named name.
func ParseOutput(name string) (Output, error) {
	switch name {
	case OutputNamePUML:
		return OutputPUML, nil
	case OutputNameJSON:
		return OutputJSON, nil
	}
	return 0, fmt.Errorf("unknown output %q (want %s or %s)", name, OutputNamePUML, OutputNameJSON)
}

type Options struct {
	Common *tools.CommonOptions
	// Base is what an !include path is resolved against: the directory the
	// input was read from, or the working directory when it came from stdin.
	Base     string
	Template string
	Werror   bool
	LintOnly bool
	// Only names the families the expansion keeps, by map; empty keeps them
	// all.
	Only   []string
	Output Output
	Args   []string
	Bytes  []byte
}

// CommonOptions returns the parsed common options.
func (o *Options) CommonOptions() *tools.CommonOptions { return o.Common }

func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		flags := flag.NewFlagSet("csdfpromote", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: csdfpromote [options] [file.puml|file.png]

Expands the promotion directives of a global Composable State Diagram: every
local diagram named by a <<promote>> block becomes self-loops on the state the
block was written in, carrying the map the family of instances lives in. The
result is plain PlantUML, which every other tool reads.
A file argument, a "-" argument, and standard input are all equivalent.

Every predicate of the expansion is its clauses joined by the connective " ∧ ":
the generated ones are builtins (@in, @notin, @at, @frame, @update, @empty,
@eq, @ne, @exists), and the author's are copied with each local variable v of
the instance x of the map m written into their holes as ${m[x].v}. Warnings
are printed on stderr at their level (warning, info); a note says what a
result about an -only restriction does not say about the whole.

With -only, the expansion keeps the named families, each named by its map,
and leaves out the others' maps and the edges only they move. An event a kept
family takes together with a left-out one stays, offered by the environment.
A clause that reads a left-out map is dropped and reported. A restriction
keeps only safety: a deadlock or another refusal it shows is no conclusion
about the full promotion, and a counterexample may be spurious.

With -template, the builtins of each edge are glossed in a line comment above
it, by templates named after the builtin words (see
examples/promote/templates/ja.tmpl).

-format json prints the expansion as JSON, as csdfparse prints a diagram
(core.DiagramDoc, version 2), with the local states of each family and the
variables each holds, which an instance in the map is: the shape csdfbmc
reads. -template is refused with it.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdfpromote path/to/GLOBAL.puml
  $ csdfpromote -lint-only -Werror path/to/GLOBAL.puml
  $ csdfpromote -template examples/promote/templates/ja.tmpl path/to/GLOBAL.puml
  $ csdfpromote path/to/GLOBAL.puml | csdflivelockfree -
  $ csdfpromote -format json path/to/GLOBAL.puml
  $ csdfpromote -only orders,Shipment -format json path/to/GLOBAL.puml
`)
		}

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)
		base := flags.String("base", "", "directory to resolve !include paths against (default: the directory of the input file)")
		templatePath := flags.String("template", "", "file of templates glossing the builtins of each edge in a line comment (default: none)")
		werror := flags.Bool("Werror", false, "treat warnings as errors")
		lintOnly := flags.Bool("lint-only", false, "check the directives without printing the expansion")
		only := flags.String("only", "", "comma-separated families to keep, each named by its map (default: all)")
		var format string
		flags.StringVar(&format, "format", OutputNamePUML, "what the expansion is written as: puml (for the other tools) or json (for a program such as csdfbmc, as csdfparse prints a diagram)")

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return &Options{Common: tools.CommonOptionsHelp}, nil
			}
			return nil, fmt.Errorf("csdfpromotecmd.NewParseOptionsFunc: parse failed: %w", err)
		}

		commonOpts, err := tools.ValidateCommonOptions(&commonRawOpts)
		if err != nil {
			return nil, fmt.Errorf("csdfpromotecmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		if commonOpts.Version {
			return &Options{Common: tools.CommonOptionsVersion}, nil
		}

		output, err := ParseOutput(format)
		if err != nil {
			return nil, fmt.Errorf("csdfpromotecmd.NewParseOptionsFunc: %w", err)
		}

		path, bs, err := tools.ValidateArgsAsFileInput(flags.Args(), inout)
		if err != nil {
			return nil, fmt.Errorf("csdfpromotecmd.NewParseOptionsFunc: validate arguments failed: %w", err)
		}

		resolved := *base
		if resolved == "" {
			resolved = "."
			if path != "" {
				resolved = filepath.Dir(path)
			}
		}

		return &Options{
			Common:   commonOpts,
			Base:     resolved,
			Template: *templatePath,
			Werror:   *werror,
			LintOnly: *lintOnly,
			Only:     splitList(*only),
			Output:   output,
			Args:     args,
			Bytes:    bs,
		}, nil
	}
}

// splitList reads a comma-separated list, dropping the empty items.
func splitList(s string) []string {
	var out []string
	for _, item := range strings.Split(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}
