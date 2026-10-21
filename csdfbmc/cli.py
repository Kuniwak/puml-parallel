"""What csdfbmc and csdfbmc next read alike: the diagram, the checks module,
and the glue the clauses are read through."""
from __future__ import annotations

import argparse
import shlex
from contextlib import contextmanager
from typing import Iterator

from csdfbmc.meaning import GlueMeaning, standard
from csdfbmc.model import Diagram
from csdfbmc.protocol import EvaluatorError, checked, start_command, start_module
from csdfbmc.checks import PurityError, Checks, load_checks

# What reading the inputs may raise; each command reports it and exits 2.
LOADING_ERRORS = (OSError, ValueError, KeyError, PurityError, EvaluatorError)


def add_inputs(ap: argparse.ArgumentParser) -> None:
    ap.add_argument("diagram", help="the diagram as JSON (csdfparse's output)")
    ap.add_argument("checks", help="the Python module holding DOMAINS and, when wanted, GHOSTS, INVARIANTS and SYMMETRIC")
    glue = ap.add_mutually_exclusive_group()
    glue.add_argument("--evaluator", metavar="CMD", help="read every clause through the glue this command line starts (the evaluator protocol)")
    glue.add_argument("--evaluator-module", metavar="MODULE", help="read every clause through the glue this Python module and its arguments load in-process")
    # --local named the twins of a family's local diagram, which are gone; it
    # is still read so that a command line written for them is refused with
    # what to do instead.
    ap.add_argument("--local", metavar="PATH=TWINS.py", action="append", default=[], help=argparse.SUPPRESS)


@contextmanager
def opened(a: argparse.Namespace) -> Iterator[tuple[Diagram, Checks, object, object]]:
    """The diagram, the checks, the meaning of the clauses and what tells the
    writes of a post clause (csdfbmc.cone), as the parsed arguments name
    them; the glue started for them is closed on leaving."""
    if a.local:
        raise ValueError("--local named the twins of a family's local clauses, which are no longer read; write the local "
                         "clauses in the language of a glue and read every clause with --evaluator-module")
    if a.evaluator is None and a.evaluator_module is None:
        raise ValueError("the clauses are read through a glue of the evaluator protocol; pass --evaluator CMD or "
                         "--evaluator-module MODULE (csdfbmc.python_glue reads Python)")
    if a.evaluator is not None:
        client = start_command(shlex.split(a.evaluator))
    else:
        words = shlex.split(a.evaluator_module)
        if not words:
            raise ValueError("--evaluator-module expects MODULE [ARGS...]")
        client = start_module(words[0], words[1:])
    try:
        diagram = Diagram.load(a.diagram)
        checks = load_checks(a.checks)
        meaning = standard(GlueMeaning(checked(client)))
        yield diagram, checks, meaning, meaning.writes
    finally:
        client.close()
