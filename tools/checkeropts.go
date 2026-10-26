package tools

import (
	"flag"
	"fmt"
	"strconv"

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
	flags.Func("seed", "with -checks, choose from this seed the functions of the environment the checks module declares and nothing gives, as csdfbmc --seed does", func(s string) error {
		n, err := strconv.Atoi(s)
		if err != nil {
			return fmt.Errorf("-seed expects an integer, not %q", s)
		}
		checker.Seed = &n
		return nil
	})
	flags.Func("env", "with -checks, give the function of the environment NAME the value true or false, as csdfbmc --env NAME=true|false does (repeatable)", func(s string) error {
		b, err := bmc.ParseBinding(s)
		if err != nil {
			return err
		}
		checker.Env = append(checker.Env, b)
		return nil
	})
}
