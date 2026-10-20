package csdfpromotecmd

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/promote"
	"github.com/Kuniwak/puml-parallel/tools"
)

type Options struct {
	Common *tools.CommonOptions
	// Base is what an !include path is resolved against: the directory the
	// input was read from, or the working directory when it came from stdin.
	Base       string
	Template   string
	NoComments bool
	Werror     bool
	LintOnly   bool
	JSON       bool
	Output     promote.Output
	Args       []string
	Bytes      []byte
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
the generated ones (the instance exists, is in a local state, moves, is
created or removed, a variable is unchanged) and then the author's, copied
verbatim. With -format json, the expansion is written as JSON instead, for a
program that must not parse the wording back:

  {"diagram": <the diagram as csdfparse prints it>,
   "edges": [{"edge": <the edge>, "origin": "promote: ...",
              "guard": [CLAUSE, ...], "post": [CLAUSE, ...]}, ...]}

where a CLAUSE is {"kind": K, "text": T, ...} with K one of in_dom, not_in_dom,
at_state, update, create, delete, frame (the generated clauses, with "map",
"id", "state" or "var" saying what they are about), local (a clause of the
local edge, with the "path" and "line" it was written on) or constrain (a
clause of a constrain directive, with its "line"). An edge the author wrote by
hand has no origin and no clauses.

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
`)
		}

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)
		base := flags.String("base", "", "directory to resolve !include paths against (default: the directory of the input file)")
		templatePath := flags.String("template", "", "file replacing the wording of the generated clauses (default: symbolic)")
		noComments := flags.Bool("no-comments", false, "omit the line comment that says where each generated edge came from")
		werror := flags.Bool("Werror", false, "treat warnings as errors")
		lintOnly := flags.Bool("lint-only", false, "check the directives without printing the expansion")
		asJSON := flags.Bool("json", false, "print the parsed directives as JSON instead of expanding them")
		var format string
		flags.StringVar(&format, "format", promote.OutputNamePUML, "what the expansion is written as: puml (for the other tools) or json (for a program, with the clauses of every edge)")

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

		output, err := promote.ParseOutput(format)
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
			Common:     commonOpts,
			Base:       resolved,
			Template:   *templatePath,
			NoComments: *noComments,
			Werror:     *werror,
			LintOnly:   *lintOnly,
			JSON:       *asJSON,
			Output:     output,
			Args:       args,
			Bytes:      bs,
		}, nil
	}
}
