package csdfparallelcmd

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/core"
	"github.com/Kuniwak/puml-parallel/tools"
)

func parseSyncEvents(s string) []core.Event {
	if s == "" {
		return nil
	}
	var events []core.Event
	for _, event := range strings.Split(s, ";") {
		trimmed := strings.TrimSpace(event)
		if trimmed != "" {
			events = append(events, core.Event(trimmed))
		}
	}
	return events
}

type Options struct {
	Common *tools.CommonOptions
	Sync   []core.Event
	Files  []string
}

func NewParseOptionsFunc() cli.ParseOptionsFunc[*Options] {
	return func(args []string, inout *cli.ProcInout) (*Options, error) {
		flags := flag.NewFlagSet("csdfparallel", flag.ContinueOnError)
		flags.SetOutput(inout.Stderr)
		flags.Usage = func() {
			w := flags.Output()
			fmt.Fprintf(w, `Usage: csdfparallel [options] <file1.puml> [file2.puml] ...

Composes Composable State Diagrams in parallel following CSP interface parallel semantics.

Options:
`)
			flags.PrintDefaults()
			fmt.Fprintf(w, `
Examples:
  $ csdfparallel a.puml
  $ csdfparallel -sync 'insert;choose;drop' a.puml b.puml
`)
		}

		var commonRawOpts tools.CommonRawOptions
		tools.DeclareCommonOptions(flags, &commonRawOpts)
		syncFlag := flags.String("sync", "", "semicolon-separated list of synchronization events")

		if err := flags.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return &Options{Common: tools.CommonOptionsHelp}, nil
			}
			return nil, fmt.Errorf("csdfparallelcmd.NewParseOptionsFunc: parse failed: %w", err)
		}

		commonOpts, err := tools.ValidateCommonOptions(&commonRawOpts)
		if err != nil {
			return nil, fmt.Errorf("csdfparallelcmd.NewParseOptionsFunc: validate common options failed: %w", err)
		}
		if commonOpts.Version {
			return &Options{Common: tools.CommonOptionsVersion}, nil
		}

		files := flags.Args()
		if len(files) < 1 {
			return nil, fmt.Errorf("csdfparallelcmd.NewParseOptionsFunc: too few arguments")
		}

		return &Options{
			Common: commonOpts,
			Sync:   parseSyncEvents(*syncFlag),
			Files:  files,
		}, nil
	}
}
