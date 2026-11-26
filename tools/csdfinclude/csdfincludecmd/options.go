package csdfincludecmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Kuniwak/puml-parallel/v6/cli"
	"github.com/Kuniwak/puml-parallel/v6/tools"
)

type Options struct {
	Common *tools.CommonOptions
	// Base is the directory the relative paths of !include lines are
	// resolved against, "" for the default, csdf.BaseDirOfTree.
	Base  string
	Bytes []byte
}

// CommonOptions returns the parsed common options.
func (o *Options) CommonOptions() *tools.CommonOptions { return o.Common }

func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		flags := flag.NewFlagSet("csdfinclude", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: csdfinclude [options] [tree.json]

Resolves the !include lines of the concrete syntax tree csdfparse prints
(JSON), and prints the tree again. Each !include keeps the path as written
and gains "tree", the syntax tree of the file it names (docs/SYNTAX.md),
parsed as csdfparse parses it, its own !include lines resolved in turn,
each relative to the file it is written in; the "path" of an included tree
is the file read. A missing file, a file that does not parse and a cycle
of !include are refused with the line of the !include.
A file argument, a "-" argument, and standard input are all equivalent.

Relative paths of !include lines are resolved against the directory of the
tree's "path", the file csdfparse read, or against the current directory when
the tree has no path, as csdfparse prints for standard input. Use -base to
override.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdfparse path/to/GLOBAL.puml | csdfinclude
  $ csdfparse path/to/GLOBAL.puml | csdfinclude | csdfpromote -format json
  $ csdfparse < path/to/GLOBAL.puml | csdfinclude -base path/to
`)
		}

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)
		base := flags.String("base", "", `directory the paths of !include lines are resolved against (default: the directory of the tree's path, or "." for a tree with no path)`)

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return &Options{Common: tools.CommonOptionsHelp}, nil
			}
			return nil, fmt.Errorf("csdfincludecmd.NewParseOptionsFunc: parse failed: %w", err)
		}

		commonOpts, err := tools.ValidateCommonOptions(&commonRawOpts)
		if err != nil {
			return nil, fmt.Errorf("csdfincludecmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		if commonOpts.Version {
			return &Options{Common: tools.CommonOptionsVersion}, nil
		}

		_, bs, err := tools.ValidateArgsAsFileInput(flags.Args(), inout)
		if err != nil {
			return nil, fmt.Errorf("csdfincludecmd.NewParseOptionsFunc: validate arguments failed: %w", err)
		}
		return &Options{Common: commonOpts, Base: *base, Bytes: bs}, nil
	}
}
