package csdfpromotecmd

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"strings"

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
	// Only names the families the expansion keeps, by map or by type; empty
	// keeps them all.
	Only   []string
	Output promote.Output
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
the generated ones (the instance exists, is in a local state, moves, is
created or removed, a variable is unchanged) and then the author's, copied
verbatim. With -format json, the expansion is written as JSON instead, for a
program that must not parse the wording back:

  {"diagram": <the diagram as csdfparse prints it>,
   "edges": [{"edge": <the edge>, "origin": "promote: ...",
              "guard": [CLAUSE, ...], "post": [CLAUSE, ...]}, ...],
   "promotes": [{"map": M, "id_param": P, "type": T, "path": F, "in": S}, ...],
   "locals": {F: <the local diagram as csdfparse prints it>, ...}}

where a CLAUSE is {"kind": K, "text": T, ...} with K one of in_dom, not_in_dom,
at_state, update, create, delete, frame (the generated clauses, with "map",
"id", "state" or "var" saying what they are about), local (a clause of the
local edge, with the "path" and "line" it was written on) or constrain (a
clause of a constrain directive, with its "line") or dropped (a guard clause
that read a map -only left out, with the "maps" it read; it is listed, not in
the predicate). An edge the author wrote by hand has no origin and no clauses.

With -only, the expansion keeps the named families (a map, or the type of
one) and leaves out the others' maps and the edges only they move. An event a
kept family takes together with a left-out one stays, offered by the
environment. A guard clause that read a left-out map is taken as true and
listed: as a warning, and as a dropped clause. Every invariant violation and
event-avoiding cycle over the kept families survives the restriction; a
deadlock a left-out family caused does not.

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
		templatePath := flags.String("template", "", "file replacing the wording of the generated clauses (default: symbolic)")
		noComments := flags.Bool("no-comments", false, "omit the line comment that says where each generated edge came from")
		werror := flags.Bool("Werror", false, "treat warnings as errors")
		lintOnly := flags.Bool("lint-only", false, "check the directives without printing the expansion")
		asJSON := flags.Bool("json", false, "print the parsed directives as JSON instead of expanding them")
		only := flags.String("only", "", "comma-separated families to keep, each a map or a type (default: all)")
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
			Only:       splitList(*only),
			Output:     output,
			Args:       args,
			Bytes:      bs,
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
