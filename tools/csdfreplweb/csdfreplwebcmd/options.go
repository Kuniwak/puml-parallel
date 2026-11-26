package csdfreplwebcmd

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/Kuniwak/puml-parallel/v4/cli"
	"github.com/Kuniwak/puml-parallel/v4/csdf/animation/bmc"
	"github.com/Kuniwak/puml-parallel/v4/tools"
)

// DefaultPumlExec is the PlantUML command -puml-exec runs unless told
// otherwise: plantuml on the PATH, reading the source on its standard input
// and writing SVG.
const DefaultPumlExec = "plantuml -pipe -tsvg"

// DefaultPyodide is where a page suggesting values loads Pyodide from
// unless -pyodide says otherwise: a pinned release on its CDN.
const DefaultPyodide = "https://cdn.jsdelivr.net/pyodide/v0.29.5/full/"

type Options struct {
	Common *tools.CommonOptions
	File   string
	// PumlExec is the command, split into words, that reads PlantUML source
	// on its standard input and writes the SVG of it on its standard output.
	PumlExec []string
	// Engine is the WebAssembly file of the engine the page carries, or ""
	// for the one the binary embeds or builds.
	Engine string
	// Checker is what the page's Python reads to suggest values, as
	// csdfrepl -checks does; it asks nothing when -checks is not given.
	Checker bmc.Checker
	// Pyodide is the URL of the directory holding pyodide.js, which a page
	// suggesting values loads.
	Pyodide string
	// Shapes is the file of what csdfbmc shapes says the diagram's formal
	// clauses reveal of its types, which the page shows by the inputs; ""
	// shows none.
	Shapes string
	// Prose are other versions of the diagram whose clauses are written
	// for people, which the page shows in place of the diagram's own, each
	// in the language its file name ends in (_ja.puml); none shows the
	// diagram's own.
	Prose []string
}

// CommonOptions returns the parsed common options.
func (o *Options) CommonOptions() *tools.CommonOptions { return o.Common }

func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		flags := flag.NewFlagSet("csdfreplweb", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: csdfreplweb [options] <file.puml|file.png>

Writes a web page that explores a Composable State Diagram as csdfrepl
does, to the standard output. The page is one HTML file that needs nothing
but a browser: open it from disk or put it on any static web host. It draws
the diagram, highlights where the run is and what can happen next, takes
the values of each state in a form, shows the history and lets one jump
back to any step of it, and saves the path explored as a file csdfrepl
-replay walks. Its words are in English and Japanese.

The drawing is the SVG PlantUML renders of the diagram: csdfreplweb runs
the command -puml-exec gives, in the diagram's directory, with the source
on its standard input, and reads the SVG on its standard output. The
command is split into words as a shell splits them, quotes included, but
no shell runs it; give the options that make PlantUML read the standard
input and write SVG yourself (-pipe -tsvg).

A global diagram holding promotion directives (docs/PROMOTION.md) is
explored as its expansion, as csdfrepl explores it, each !include read
relative to the file; its drawing is the global diagram, without
highlights.

With -checks and -evaluator-module, the page suggests at every value
prompt the values the clauses admit, as csdfrepl does with the same
flags, and the tester picks one instead of typing the values. It runs
csdfbmc next in Python compiled to WebAssembly (Pyodide), which it loads
from -pyodide when it opens; the page carries csdfbmc, the checks module
and every word of -evaluator-module naming a file, read relative to the
current directory as csdfbmc reads them. The glue must be a Python
module: a browser starts no command, so -evaluator is refused.

With -shapes, the page shows by each input what the values of its type
look like: the attributes and example values csdfbmc shapes reads from the
clauses of the diagram written in Python, printed to the file given.

With -prose, the page shows in place of each guard and post those of
another version of the diagram whose clauses are written for people, the
diagram's own folded under them ("Formal clause"). The other version is
the same diagram but for its clauses: the same states, variables,
channels and start state, and the same edges and end edges in the same
order; csdfreplweb refuses it, naming the first difference, otherwise.
Give -prose once per language, each file named by its language, as in
diagram_en.puml and diagram_ja.puml: the page shows the version in the
language it is shown in, else the one whose name gives no language, else
the English one.

The page carries the exploration engine compiled to WebAssembly. A
release binary embeds it. Otherwise csdfreplweb builds it with the go
command on the PATH: a released version from the module cache, kept
under the user cache directory (csdfreplweb/<version>), and a build from
a work tree from the work tree the current directory is in. -engine
takes it from a file instead, gzipped or not, such as the
csdfreplweb-engine_<version>.wasm.gz attached to every release.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdfreplweb examples/valid/vending_machine.puml > vending_machine.html
  $ csdfreplweb -puml-exec 'java -jar "/opt/plant uml/plantuml.jar" -pipe -tsvg' diagram.puml > diagram.html
  $ csdfreplweb -engine csdfreplweb-engine_3.14.0.wasm.gz diagram.puml > diagram.html
  $ csdfreplweb -checks examples/bmc/printer_formal_defs.py \
      -evaluator-module "csdfbmc.python_glue examples/bmc/printer_formal_defs.py" \
      examples/bmc/printer_formal.puml > printer.html
  $ csdfparse diagram.puml | csdfpromote -format json > diagram.json && csdfbmc shapes diagram.json > shapes.json
  $ csdfreplweb -shapes shapes.json diagram.puml > diagram.html
  $ csdfreplweb -prose diagram_en.puml -prose diagram_ja.puml diagram_formal.puml > diagram.html
`)
		}

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)
		engine := flags.String("engine", "", "WebAssembly file (gzipped or not) of the engine the page carries, instead of the embedded or built one")
		pumlExec := flags.String("puml-exec", DefaultPumlExec, "command that reads PlantUML source on its standard input and writes SVG on its standard output")
		var checker bmc.Checker
		tools.DeclareCheckerOptions(flags, &checker)
		shapes := flags.String("shapes", "", "JSON file csdfbmc shapes prints of the diagram, whose shapes of the types the page shows by the inputs")
		var prose files
		flags.Var(&prose, "prose", "another version of the diagram, the same but for its clauses, whose guards and posts the page shows in place of the diagram's; repeat it for each language, which the file name ends in, as in vending_ja.puml")
		pyodide := flags.String("pyodide", DefaultPyodide, "with -checks, URL of the directory holding pyodide.js, which the page loads to run csdfbmc")

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return &Options{Common: tools.CommonOptionsHelp}, nil
			}
			return nil, fmt.Errorf("csdfreplwebcmd.NewParseOptionsFunc: parse failed: %w", err)
		}

		commonOpts, err := tools.ValidateCommonOptions(&commonRawOpts)
		if err != nil {
			return nil, fmt.Errorf("csdfreplwebcmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		if commonOpts.Version {
			return &Options{Common: tools.CommonOptionsVersion}, nil
		}

		if err := tools.ValidateCheckerOptions(checker); err != nil {
			return nil, fmt.Errorf("csdfreplwebcmd.NewParseOptionsFunc: %w", err)
		}
		if checker.Evaluator != "" {
			return nil, errors.New("csdfreplwebcmd.NewParseOptionsFunc: -evaluator starts a command, which a browser cannot; give the glue as a Python module with -evaluator-module")
		}

		argv, err := SplitCommandLine(*pumlExec)
		if err != nil {
			return nil, fmt.Errorf("csdfreplwebcmd.NewParseOptionsFunc: -puml-exec: %w", err)
		}
		if flags.NArg() != 1 {
			return nil, errors.New("csdfreplwebcmd.NewParseOptionsFunc: exactly one CSDF file is required")
		}

		return &Options{Common: commonOpts, File: flags.Arg(0), PumlExec: argv, Engine: *engine, Checker: checker, Pyodide: *pyodide, Shapes: *shapes, Prose: prose}, nil
	}
}

// files are the values of a flag given once per file.
type files []string

func (f *files) String() string     { return strings.Join(*f, ", ") }
func (f *files) Set(v string) error { *f = append(*f, v); return nil }
