"""csdfbmc: bounded model checking of a Composable State Diagram over finite domains.

The Go tools own the structure: csdfparse prints the diagram as JSON and
csdfpromote -format json prints an expansion with the clauses of every edge.
This package owns the values: it reads every clause through a glue of the
evaluator protocol (csdfbmc.python_glue reads Python), takes the finite
domains, ghosts and invariants from a checks module, and walks the product of
control states and valuations, checking for deadlocks and tau cycles within
the bound.
"""
from csdfbmc.model import Diagram
from csdfbmc.checks import Checks, load_checks, PurityError
from csdfbmc.explore import Explorer, Result

__all__ = ["Diagram", "Checks", "load_checks", "PurityError", "Explorer", "Result"]
