package csdfcompcmd

import (
	"errors"
	"flag"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/tools"
)

type Options struct {
	Common *tools.CommonOptions
	// BaseDir is the directory the paths of REFER expressions are resolved against.
	BaseDir string
	Bytes   []byte
}

// CommonOptions returns the parsed common options.
func (o *Options) CommonOptions() *tools.CommonOptions { return o.Common }

func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		flags := flag.NewFlagSet("csdfcomp", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: csdfcomp [options] [tree.json]

Composes the Composable State Diagrams referred to by a composition tree,
following CSP interface parallel and hiding semantics, and prints the result
as PlantUML. The tree is a JSON process expression built from REFER, HIDE and
INTERFACE_PARALLEL nodes (see docs/COMPOSITION_TREE.md).
A file argument, a "-" argument, and standard input are all equivalent.

Relative paths of REFER expressions are resolved against the directory of the
tree file, or against the current directory when the tree is read from
standard input. Use -base to override.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdfcomp path/to/tree.json
  $ csdfcomp -base path/to < path/to/tree.json
`)
		}

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)
		baseFlag := flags.String("base", "", `directory the paths of REFER expressions are resolved against (default: the directory of the tree file, or "." for standard input)`)

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return &Options{Common: tools.CommonOptionsHelp}, nil
			}
			return nil, fmt.Errorf("csdfcompcmd.NewParseOptionsFunc: parse failed: %w", err)
		}

		commonOpts, err := tools.ValidateCommonOptions(&commonRawOpts)
		if err != nil {
			return nil, fmt.Errorf("csdfcompcmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		if commonOpts.Version {
			return &Options{Common: tools.CommonOptionsVersion}, nil
		}

		treePath, bs, err := tools.ValidateArgsAsFileInput(flags.Args(), inout)
		if err != nil {
			return nil, fmt.Errorf("csdfcompcmd.NewParseOptionsFunc: validate arguments failed: %w", err)
		}

		baseDir := *baseFlag
		if baseDir == "" {
			baseDir = csdf.BaseDirOf(treePath)
		}

		return &Options{
			Common:  commonOpts,
			BaseDir: baseDir,
			Bytes:   bs,
		}, nil
	}
}
