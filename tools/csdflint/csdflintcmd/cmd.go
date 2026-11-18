package csdflintcmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/Kuniwak/puml-parallel/v3/cli"
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

		rules := lint.DefaultRules()

		if opts.ListRules {
			for _, rule := range append(rules, lint.AtomNotInLanguageRule{}) {
				fmt.Fprintf(inout.Stdout, "%s: %s\n", rule.ID(), rule.Description())
			}
			return nil
		}

		if opts.Evaluator != "" {
			c, err := startGlue(opts.Evaluator)
			if err != nil {
				return &tools.ExitStatusError{Status: 2, Message: err.Error()}
			}
			defer c.Close()
			rules = append(rules, lint.AtomNotInLanguageRule{Glue: c})
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

// startGlue starts the glue the command line names, its errors on the
// tool's standard error.
func startGlue(line string) (*glue.Client, error) {
	words, err := glue.SplitCommand(line)
	if err != nil {
		return nil, err
	}
	if len(words) == 0 {
		return nil, errors.New("-evaluator expects the command line of a glue")
	}
	cmd := exec.Command(words[0], words[1:]...)
	cmd.Stderr = os.Stderr
	return glue.Start(cmd)
}
