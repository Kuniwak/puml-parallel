package tools

import (
	"errors"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/Kuniwak/puml-parallel/csdf/animation/bmc"
)

// DeclareCheckerOptions declares -checks, -evaluator, -evaluator-module,
// -seed and -env on flags, filling in checker. -seed and -env are csdfbmc's
// --seed and --env: a REPL given them walks the environment csdfbmc walks
// with them. Neither has a default; checker.Seed stays nil when -seed is
// not given.
func DeclareCheckerOptions(flags *flag.FlagSet, checker *bmc.Checker) {
	flags.StringVar(&checker.Checks, "checks", "", "csdfbmc checks module (TYPES, TYPEOF, GHOSTS, INVARIANTS); with a glue, the value prompt suggests the values the clauses admit")
	flags.StringVar(&checker.Evaluator, "evaluator", "", "with -checks, the command line of the glue csdfbmc reads the clauses through")
	flags.StringVar(&checker.EvaluatorModule, "evaluator-module", "", "with -checks, the Python module (and its arguments) of the glue csdfbmc reads the clauses through, such as csdfbmc.python_glue")
	flags.Func("seed", "with -checks, choose from this seed the functions of the environment the checks module declares and nothing gives, as csdfbmc --seed does", func(word string) error {
		n, err := parseSeed(word)
		if err != nil {
			return err
		}
		checker.Seed = &n
		return nil
	})
	flags.Func("env", "with -checks, give the function of the environment NAME the value true or false, as csdfbmc --env NAME=true|false does (repeatable)", func(word string) error {
		b, err := parseBinding(word)
		if err != nil {
			return err
		}
		checker.Env = append(checker.Env, b)
		return nil
	})
}

// parseSeed reads an integer as csdfbmc reads --seed.
func parseSeed(word string) (int, error) {
	n, err := strconv.Atoi(word)
	if err != nil {
		return 0, fmt.Errorf("-seed expects an integer, not %q", word)
	}
	return n, nil
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
// has one form, and -seed and -env decide the functions a checks module
// declares.
func ValidateCheckerOptions(c bmc.Checker) error {
	glue := c.Evaluator != "" || c.EvaluatorModule != ""
	switch {
	case c.Evaluator != "" && c.EvaluatorModule != "":
		return errors.New("give -evaluator or -evaluator-module, not both")
	case c.Checks != "" && !glue:
		return errors.New("-checks needs a glue: csdfbmc reads every clause through one; pass -evaluator or -evaluator-module (csdfbmc.python_glue reads Python)")
	case glue && c.Checks == "":
		return errors.New("-evaluator and -evaluator-module need -checks, the module of the domains")
	case (c.Seed != nil || len(c.Env) > 0) && c.Checks == "":
		return errors.New("-seed and -env need -checks, whose ENVIRONMENT declares the functions they decide")
	}
	return nil
}
