package csdfrepldcmd

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/tools"
)

type Options struct {
	Common *tools.CommonOptions
	Sock   string
	// Twins is the csdfbmc twins module that lets every session's value
	// prompts suggest values; Locals the per-family twins of an expansion.
	Twins  string
	Locals []string
}

// CommonOptions returns the parsed common options.
func (o *Options) CommonOptions() *tools.CommonOptions { return o.Common }

func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		flags := flag.NewFlagSet("csdfrepld", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: csdfrepld [options]

Runs the CSDF REPL daemon. It listens on a Unix domain socket and holds
interactive exploration sessions in memory, which the csdfreplcmd client drives.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
The socket path defaults to $%s, then $XDG_RUNTIME_DIR/%s, then <tmp>/%s.

Examples:
  $ csdfrepld
  $ csdfrepld -sock /tmp/csdfrepld.sock
`, tools.SocketEnv, tools.SocketName, tools.SocketName)
		}

		var sock string
		flags.StringVar(&sock, "sock", "", "path of the Unix domain socket to listen on")
		twins := flags.String("twins", "", "csdfbmc twins module; value prompts then suggest the values it admits and statevar -pick works")
		var locals repeated
		flags.Var(&locals, "local", "with -twins, the twins of a family's local diagram as PATH=TWINS.py (repeatable)")

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return &Options{Common: tools.CommonOptionsHelp}, nil
			}
			return nil, fmt.Errorf("csdfrepldcmd.NewParseOptionsFunc: parse failed: %w", err)
		}

		commonOpts, err := tools.ValidateCommonOptions(&commonRawOpts)
		if err != nil {
			return nil, fmt.Errorf("csdfrepldcmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		if commonOpts.Version {
			return &Options{Common: tools.CommonOptionsVersion}, nil
		}

		return &Options{Common: commonOpts, Sock: sock, Twins: *twins, Locals: []string(locals)}, nil
	}
}

type repeated []string

func (r *repeated) String() string     { return strings.Join(*r, ",") }
func (r *repeated) Set(v string) error { *r = append(*r, v); return nil }
