"""csdfbmc: bounded model checking of a Composable State Diagram over finite domains.

The Go tools own the structure: csdfparse prints the diagram as JSON and
csdfpromote -format json prints an expansion with the clauses of every edge.
This package owns the values: the formal twin of each clause (a pure Python
function keyed by the clause's prose), the finite domains the values are drawn
from, and the walk over the product of control states and valuations that
checks for deadlocks and tau cycles within the bound.
"""
from csdfbmc.model import Diagram
from csdfbmc.twins import Twins, load_twins, PurityError
from csdfbmc.explore import Explorer, Result

__all__ = ["Diagram", "Twins", "load_twins", "PurityError", "Explorer", "Result"]
