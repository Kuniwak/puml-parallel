"""What csdfbmc and csdfbmc next read alike: the diagram, the checks module,
the environment, and the glue the clauses are read through."""
from __future__ import annotations

import argparse
import shlex
from contextlib import contextmanager
from typing import Iterator

from csdfbmc.meaning import GlueMeaning, standard
from csdfbmc.model import Diagram
from csdfbmc.protocol import EvaluatorError, checked, start_command, start_module
from csdfbmc.checks import PurityError, Checks, load_checks
from csdfbmc import environment as environments
from csdfbmc.environment import Environment

# What reading the inputs may raise; each command reports it and exits 2.
LOADING_ERRORS = (OSError, ValueError, KeyError, PurityError, EvaluatorError)


def add_inputs(ap: argparse.ArgumentParser) -> None:
    ap.add_argument("diagram", help="the diagram as JSON (csdfparse's output)")
    ap.add_argument("checks", help="the Python module holding TYPES and, when wanted, TYPEOF, GHOSTS, INVARIANTS and SYMMETRIC")
    ap.add_argument("--seed", type=int, metavar="N", help="choose the functions a clause not formalised denotes from this seed, where nothing gives them (csdfbmc.environment)")
    ap.add_argument("--env", action="append", default=[], metavar="NAME=true|false",
                    help="give the function NAME a clause not formalised denotes the value true or false, over the checks module's ENVIRONMENT (repeatable)")
    glue = ap.add_mutually_exclusive_group()
    glue.add_argument("--evaluator", metavar="CMD", help="read every clause through the glue this command line starts (the evaluator protocol)")
    glue.add_argument("--evaluator-module", metavar="MODULE", help="read every clause through the glue this Python module and its arguments load in-process")


def environment(a: argparse.Namespace, checks: Checks) -> Environment:
    """The environment the arguments and the checks module give: --env over
    ENVIRONMENT, and --seed for the rest."""
    given = {}
    for word in a.env:
        name, sep, value = word.rpartition("=")
        if not sep or not name or value not in ("true", "false"):
            raise ValueError(f"--env expects NAME=true or NAME=false, not {word!r}")
        given[name] = value == "true"
    return environments.over(checks.environment, given, a.seed)


@contextmanager
def opened(a: argparse.Namespace) -> Iterator[tuple[Diagram, Checks, object, object, Environment]]:
    """The diagram, the checks, the meaning of the clauses, what tells the
    writes of a post clause (csdfbmc.cone) and the environment, as the parsed
    arguments name them; the glue started for them is closed on leaving."""
    if a.evaluator is None and a.evaluator_module is None:
        raise ValueError("a walk needs a glue: csdfbmc reads every clause through one; pass --evaluator CMD or "
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
        yield diagram, checks, meaning, meaning.writes, environment(a, checks)
    finally:
        client.close()
