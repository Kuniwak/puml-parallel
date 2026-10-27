package tools

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/animation/bmc"
)

// DeclareCheckerOptions declares -checks, -evaluator, -evaluator-module
// and -env on flags, filling in checker. -env is csdfbmc's --env: a REPL
// given it walks the environment csdfbmc walks with it. It has no default.
func DeclareCheckerOptions(flags *flag.FlagSet, checker *bmc.Checker) {
	flags.StringVar(&checker.Checks, "checks", "", "csdfbmc checks module (TYPES, GHOSTS, INVARIANTS); with a glue, the value prompt suggests the values the clauses admit")
	flags.StringVar(&checker.Evaluator, "evaluator", "", "with -checks, the command line of the glue csdfbmc reads the clauses through")
	flags.StringVar(&checker.EvaluatorModule, "evaluator-module", "", "with -checks, the Python module (and its arguments) of the glue csdfbmc reads the clauses through, such as csdfbmc.python_glue")
	flags.Func("env", "with -checks, give the function of the environment NAME the value true or false, as csdfbmc --env NAME=true|false does (repeatable)", func(word string) error {
		b, err := parseBinding(word)
		if err != nil {
			return err
		}
		checker.Env = append(checker.Env, b)
		return nil
	})
}

// parseBinding reads NAME=true or NAME=false as csdfbmc reads --env: the
// name is everything before the last "=", and must not be empty.
func parseBinding(word string) (bmc.Binding, error) {
	i := strings.LastIndex(word, "=")
	if i <= 0 || (word[i+1:] != "true" && word[i+1:] != "false") {
		return bmc.Binding{}, fmt.Errorf("-env expects NAME=true or NAME=false, not %q", word)
	}
	return bmc.Binding{Name: word[:i], Value: word[i+1:] == "true"}, nil
}

// ValidateCheckerOptions refuses the checker options csdfbmc could not run
// with: a checks module needs a glue, a glue needs a checks module, the glue
// has one form, and -env decides the functions a checks module declares.
func ValidateCheckerOptions(c bmc.Checker) error {
	glue := c.Evaluator != "" || c.EvaluatorModule != ""
	switch {
	case c.Evaluator != "" && c.EvaluatorModule != "":
		return errors.New("give -evaluator or -evaluator-module, not both")
	case c.Checks != "" && !glue:
		return errors.New("-checks needs a glue: csdfbmc reads every clause through one; pass -evaluator or -evaluator-module (csdfbmc.python_glue reads Python)")
	case glue && c.Checks == "":
		return errors.New("-evaluator and -evaluator-module need -checks, the module of the domains")
	case len(c.Env) > 0 && c.Checks == "":
		return errors.New("-env needs -checks, whose ENVIRONMENT declares the functions they decide")
	}
	return nil
}
