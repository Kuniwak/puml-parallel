package csdflintcmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/Kuniwak/puml-parallel/v3/cli"
	"github.com/Kuniwak/puml-parallel/v3/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v3/csdf/glue"
	"github.com/Kuniwak/puml-parallel/v3/csdf/lint"
	"github.com/Kuniwak/puml-parallel/v3/tools"
	"github.com/Kuniwak/puml-parallel/v3/version"
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

		if opts.ListRules {
			for _, rule := range lint.Catalog() {
				fmt.Fprintf(inout.Stdout, "%s: %s\n", rule.ID, rule.Description)
			}
			return nil
		}

		rules := lint.DefaultRules()
		if opts.Evaluator != "" {
			c, err := startGlue(opts.Evaluator, inout.Stderr)
			if err != nil {
				return &tools.ExitStatusError{Status: 2, Message: err.Error()}
			}
			defer c.Close()
			rules = append(rules, lint.AtomNotInLanguageRule{Lint: func(atom clause.AtomWire, role lint.Role) (string, error) {
				return c.Lint(atom, string(role))
			}})
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

// startGlue starts the glue the command line names, which has to answer
// lint, its errors on stderr.
func startGlue(line string, stderr io.Writer) (*glue.Client, error) {
	c, err := glue.StartCommandLine(line, stderr)
	if err != nil {
		return nil, fmt.Errorf("-evaluator: %w", err)
	}
	if !c.Supports("lint") {
		c.Close()
		return nil, fmt.Errorf("-evaluator: the %s glue does not answer lint, which csdflint asks of every atom", c.Language())
	}
	return c, nil
}
