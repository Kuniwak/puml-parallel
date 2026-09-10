package csdflintcmd

import (
	"errors"
	"fmt"

	"github.com/Kuniwak/puml-parallel/cli"
	"github.com/Kuniwak/puml-parallel/csdf/lint"
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

		rules := lint.DefaultRules()

		if opts.ListRules {
			for _, rule := range rules {
				fmt.Fprintf(inout.Stdout, "%s: %s\n", rule.ID(), rule.Description())
			}
			return nil
		}

		inputs := make([]*lint.Input, 0, len(opts.Inputs))
		for _, input := range opts.Inputs {
			inputs = append(inputs, lint.NewInput(input.Name, input.Bytes))
		}
		findings := lint.RunAll(rules, inputs)

		if err := lint.WriteTSV(inout.Stdout, findings); err != nil {
			return fmt.Errorf("csdflintcmd.NewMainFunc: %w", err)
		}

		// The findings are written either way: a run that fails still has to say
		// what it found, and the exit status is the verdict on top of them.
		if lint.HasError(findings) {
			return errors.New("csdflintcmd.NewMainFunc: the input has errors")
		}
		return nil
	}
}
