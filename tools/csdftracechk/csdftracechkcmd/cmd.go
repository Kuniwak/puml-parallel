package csdftracechkcmd

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf"
	"github.com/Kuniwak/puml-parallel/csdf/tracechk"
	"github.com/Kuniwak/puml-parallel/usererr"
	"github.com/Kuniwak/puml-parallel/version"
)

// traceReadError says which trace file could not be read as a trace. It is
// UserFacing because the file name is what the reader needs, and unwrapping to
// the deepest error would drop it.
type traceReadError struct {
	name string
	err  error
}

func (e *traceReadError) Error() string { return fmt.Sprintf("%s: %s", e.name, usererr.Message(e.err)) }
func (e *traceReadError) Unwrap() error { return e.err }
func (e *traceReadError) UserFacing()   {}

func NewMainFunc() cli.MainFunc[*Options] {
	return func(opts *Options, inout *cli.ProcInout) error {
		if opts.Common.Help {
			return nil
		}
		if opts.Common.Version {
			fmt.Fprintln(inout.Stdout, version.Version)
			return nil
		}

		diagram, err := csdf.ParseBytes(opts.Diagram)
		if err != nil {
			return fmt.Errorf("csdftracechkcmd.NewMainFunc: %w", err)
		}

		traces := make([]tracechk.Trace, 0, len(opts.Traces))
		for _, input := range opts.Traces {
			events, err := tracechk.ReadTSV(bytes.NewReader(input.Bytes))
			if err != nil {
				return fmt.Errorf("csdftracechkcmd.NewMainFunc: %w", &traceReadError{name: input.Name, err: err})
			}
			traces = append(traces, tracechk.Trace{Name: input.Name, Events: events})
		}

		// Every trace is reported, and the exit status is the verdict on top of
		// the reports: a run that fails still has to say where each trace failed.
		results := tracechk.CheckAll(opts.Match.Rule(), diagram, traces)
		for _, result := range results {
			if err := tracechk.WriteMarkdown(inout.Stdout, diagram, result); err != nil {
				return fmt.Errorf("csdftracechkcmd.NewMainFunc: %w", err)
			}
		}
		if tracechk.AnyRejected(results) {
			return errors.New("csdftracechkcmd.NewMainFunc: some trace is not a trace of the diagram even with every guard true")
		}
		return nil
	}
}
