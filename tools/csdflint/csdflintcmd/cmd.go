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

		var findings []lint.Finding
		for _, input := range opts.Inputs {
			findings = append(findings, lint.Run(rules, lint.NewInput(input.Name, input.Bytes))...)
		}

		if err := lint.WriteTSV(inout.Stdout, findings); err != nil {
			return fmt.Errorf("csdflintcmd.NewMainFunc: %w", err)
		}

		// The findings are written either way: a run that fails still has to say
		// what it found, and the exit status is the verdict on top of them.
		if lint.HasError(findings) {
			return fmt.Errorf("csdflintcmd.NewMainFunc: %w", errors.New("the input has errors"))
		}
		return nil
	}
}
