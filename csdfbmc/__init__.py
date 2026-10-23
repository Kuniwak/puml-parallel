"""csdfbmc: bounded model checking of a Composable State Diagram over finite domains.

The Go tools own the structure: csdfparse prints the diagram as JSON, its
clauses as formulas (csdfpromote -format json, an expansion alike). This
package owns the values: it evaluates the builtins and ∃ itself, reads every
atom through a glue of the evaluator protocol (csdfbmc.python_glue reads
Python) unless the checks module declares it a function of the environment,
takes the types, ghosts and invariants from the checks module, and walks the
product of control states and valuations, checking for deadlocks and tau
cycles within the bound.
"""
from csdfbmc.model import Diagram
from csdfbmc.checks import Checks, load_checks, PurityError
from csdfbmc.explore import Explorer, Result

__all__ = ["Diagram", "Checks", "load_checks", "PurityError", "Explorer", "Result"]
