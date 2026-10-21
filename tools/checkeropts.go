package tools

import (
	"flag"

	"github.com/Kuniwak/puml-parallel/csdf/animation/bmc"
)

// DeclareCheckerOptions declares -checks, -evaluator and -evaluator-module on
// flags, filling in checker.
func DeclareCheckerOptions(flags *flag.FlagSet, checker *bmc.Checker) {
	flags.StringVar(&checker.Checks, "checks", "", "csdfbmc checks module (TYPES, TYPEOF, GHOSTS, INVARIANTS); with a glue, the value prompt suggests the values the clauses admit")
	flags.StringVar(&checker.Evaluator, "evaluator", "", "with -checks, the command line of the glue csdfbmc reads the clauses through")
	flags.StringVar(&checker.EvaluatorModule, "evaluator-module", "", "with -checks, the Python module (and its arguments) of the glue csdfbmc reads the clauses through, such as csdfbmc.python_glue")
}
