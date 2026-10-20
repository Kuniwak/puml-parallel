"""What csdfbmc and csdfbmc next read alike: the diagram, the twins module,
the twins of each family, and the glue the clauses may be read through."""
from __future__ import annotations

import argparse
import shlex
from contextlib import contextmanager
from typing import Iterator

from csdfbmc.meaning import GlueMeaning, TwinsMeaning
from csdfbmc.model import Diagram
from csdfbmc.protocol import EvaluatorError, checked, start_command, start_module
from csdfbmc.twins import PurityError, Twins, load_twins

# What reading the inputs may raise; each command reports it and exits 2.
LOADING_ERRORS = (OSError, ValueError, KeyError, PurityError, EvaluatorError)


def add_inputs(ap: argparse.ArgumentParser) -> None:
    ap.add_argument("diagram", help="the diagram as JSON (csdfparse's output)")
    ap.add_argument("twins", help="the Python module holding DOMAINS and, unless a glue reads the clauses, CLAUSES")
    glue = ap.add_mutually_exclusive_group()
    glue.add_argument("--evaluator", metavar="CMD", help="read every clause through the glue this command line starts (the evaluator protocol)")
    glue.add_argument("--evaluator-module", metavar="MODULE", help="read every clause through the glue this Python module and its arguments load in-process")
    ap.add_argument("--local", metavar="PATH=TWINS.py", action="append", default=[],
                    help="the twins of the local diagram at PATH, as the expansion names it (repeatable)")


def _load_twins(a: argparse.Namespace) -> Twins:
    twins = load_twins(a.twins)
    for spec in a.local:
        path, sep, module = spec.partition("=")
        if not sep or not path or not module:
            raise ValueError(f"--local expects PATH=TWINS.py, got {spec!r}")
        twins.locals[path] = load_twins(module)
    return twins


@contextmanager
def opened(a: argparse.Namespace) -> Iterator[tuple[Diagram, Twins, object]]:
    """The diagram, the twins and the meaning of the clauses the parsed
    arguments name; a glue started for them is closed on leaving."""
    client = None
    if a.evaluator is not None:
        client = start_command(shlex.split(a.evaluator))
    elif a.evaluator_module is not None:
        name, *args = shlex.split(a.evaluator_module) or [""]
        client = start_module(name, args)
    try:
        diagram = Diagram.load(a.diagram)
        twins = _load_twins(a)
        yield diagram, twins, GlueMeaning(checked(client)) if client is not None else TwinsMeaning(diagram, twins)
    finally:
        if client is not None:
            client.close()
