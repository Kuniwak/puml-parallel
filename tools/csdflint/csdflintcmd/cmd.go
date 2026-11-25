package csdflintcmd

import (
	"errors"
	"fmt"
	"os/exec"

	"github.com/Kuniwak/puml-parallel/v4/cli"
	"github.com/Kuniwak/puml-parallel/v4/csdf/clause"
	"github.com/Kuniwak/puml-parallel/v4/csdf/glue"
	"github.com/Kuniwak/puml-parallel/v4/csdf/lint"
	"github.com/Kuniwak/puml-parallel/v4/csdfbmc"
	"github.com/Kuniwak/puml-parallel/v4/tools"
	"github.com/Kuniwak/puml-parallel/v4/version"
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
		var glueErr error
		if opts.Evaluator != "" || opts.EvaluatorModule != "" {
			c, err := startGlue(opts, inout)
			if err != nil {
				return &tools.ExitStatusError{Status: 2, Message: err.Error()}
			}
			defer c.Close()
			rules = append(rules, lint.AtomNotInLanguageRule{
				Lint: func(atom clause.AtomWire, role lint.Role) (string, error) { return c.Lint(atom, string(role)) },
				Fail: func(err error) { glueErr = errors.Join(glueErr, err) },
			})
		}

		inputs := make([]*lint.Input, 0, len(opts.Inputs))
		for _, input := range opts.Inputs {
			inputs = append(inputs, lint.NewInput(input.Name, input.Bytes))
		}
		findings := lint.RunAll(rules, inputs)

		if err := lint.WriteTSV(inout.Stdout, findings); err != nil {
			return fmt.Errorf("csdflintcmd.NewMainFunc: %w", err)
		}

		if glueErr != nil {
			return &tools.ExitStatusError{Status: 2, Message: fmt.Sprintf("-evaluator: the glue failed, so the atoms are not all checked: %v", glueErr)}
		}

		// The findings are written either way: a run that fails still has to say
		// what it found, and the exit status is the verdict on top of them.
		if lint.HasError(findings) {
			return errors.New("csdflintcmd.NewMainFunc: the input has errors")
		}
		return nil
	}
}

// startGlue starts the glue -evaluator or -evaluator-module names, which has
// to answer lint, its errors on the tool's standard error.
func startGlue(opts *Options, inout *cli.ProcInout) (*glue.Client, error) {
	c, err := startNamedGlue(opts, inout)
	if err != nil {
		return nil, err
	}
	if !c.Supports("lint") {
		c.Close()
		return nil, fmt.Errorf("the %s glue does not answer lint, which csdflint asks of every atom", c.Language())
	}
	return c, nil
}

func startNamedGlue(opts *Options, inout *cli.ProcInout) (*glue.Client, error) {
	if opts.Evaluator != "" {
		c, err := glue.StartCommandLine(opts.Evaluator, inout.Stderr)
		if err != nil {
			return nil, fmt.Errorf("-evaluator: %w", err)
		}
		return c, nil
	}
	words, err := glue.SplitCommand(opts.EvaluatorModule)
	if err != nil || len(words) == 0 {
		return nil, fmt.Errorf("-evaluator-module expects MODULE [ARGS...], not %q", opts.EvaluatorModule)
	}
	python, err := csdfbmc.Interpreter(opts.Python, inout.Env)
	if err != nil {
		return nil, fmt.Errorf("-evaluator-module: %w", err)
	}
	dir, err := csdfbmc.Materialize(csdfbmc.CacheRoot(inout.Env))
	if err != nil {
		return nil, fmt.Errorf("-evaluator-module: %w", err)
	}
	cmd := exec.Command(python, csdfbmc.ModuleArgs(dir, csdfbmc.ModuleServe, words...)...)
	cmd.Stderr = inout.Stderr
	c, err := glue.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("-evaluator-module: %w", err)
	}
	return c, nil
}
