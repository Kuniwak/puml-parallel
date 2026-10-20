"""What the clauses of a diagram mean to the walk: what a hand-written or
constrain clause reads, and what it evaluates to. The generated clauses of
an expansion have their meaning built into the walk.

TwinsMeaning takes it from a twins module, keyed by the clause's prose (or a
constrain's formal expression); GlueMeaning from a glue of the evaluator
protocol, by the clause's text.
"""
from __future__ import annotations

import inspect
from typing import Protocol

from csdfbmc.formal import FORMAL_GLOBALS
from csdfbmc.model import Clause, Diagram
from csdfbmc.protocol import ClientLike
from csdfbmc.reduce import formal_names, names_beyond
from csdfbmc.twins import Twins, apply, check_pure

# What a clause with no meaning to evaluate stands for: no twin, a glue
# that says the clause is not formalised yet, or the reserved clause opaque.
OPAQUE = object()

# The reserved clause of a clause not formalised yet (docs/SYNTAX.md).
RESERVED_OPAQUE = "opaque"


def is_reserved_opaque(clause: Clause) -> bool:
    return clause.text.strip() == RESERVED_OPAQUE


class Meaning(Protocol):
    """What the walk needs to know of the hand-written and constrain clauses."""
    # readable: the clauses can be read, as source, to check SYMMETRIC.
    readable: bool
    # by_prose: the twins' CLAUSES give the meaning.
    by_prose: bool

    def names(self, clause: Clause, role: str) -> set[str] | None: ...
    def names_beyond(self, clause: Clause, own_id: str | None) -> set[str] | None: ...
    def prefetch(self, clause: Clause, role: str, envs: list[dict]) -> None: ...
    def evaluate(self, clause: Clause, role: str, env: dict): ...
    def opaque_message(self, clause: Clause) -> str: ...


class TwinsMeaning:
    readable = True
    by_prose = True

    def __init__(self, diagram: Diagram, twins: Twins):
        self.d = diagram
        self.twins = twins
        self._formals: dict[str, object] = {}

    def _twin(self, clause: Clause):
        """The callable standing for a clause, or None when there is none."""
        if clause.kind == "local" and clause.map:
            return self.twins.local(self.d.local_of(clause).path).clauses.get(clause.text)
        fn = self.twins.clauses.get(clause.text)
        if fn is None and clause.kind == "constrain" and clause.formal:
            fn = self._formal(clause)
        return fn

    def _formal(self, clause: Clause):
        code = self._formals.get(clause.text)
        if code is None:
            check_pure(clause.formal, f"<formal line {clause.line}>")
            code = compile(clause.formal, f"<formal line {clause.line}>", "eval")
            self._formals[clause.text] = code

        def run(**env):
            return eval(code, FORMAL_GLOBALS, env)
        run.formal = True
        return run

    def names(self, clause: Clause, role: str) -> set[str] | None:
        """The names a clause reads: what its twin's signature asks for, or
        the names of its formal expression. None for an opaque clause."""
        fn = self._twin(clause)
        if fn is None:
            return None
        return formal_names(clause.formal) if getattr(fn, "formal", False) else set(inspect.signature(fn).parameters)

    def names_beyond(self, clause: Clause, own_id: str | None) -> set[str] | None:
        """The names a clause reads but a map read only as m[own_id], when
        that can be told (a formal expression); None otherwise."""
        fn = self._twin(clause)
        if fn is not None and getattr(fn, "formal", False):
            return names_beyond(clause.formal, own_id)
        return None

    def prefetch(self, clause: Clause, role: str, envs: list[dict]) -> None:
        pass

    def evaluate(self, clause: Clause, role: str, env: dict):
        """What clause evaluates to under env, or OPAQUE."""
        fn = self._twin(clause)
        if fn is None:
            return OPAQUE
        if getattr(fn, "formal", False):
            return fn(**env)
        return apply(fn, env, what=f"the twin of {clause.text!r}")

    def opaque_message(self, clause: Clause) -> str:
        where = f"in the twins of {self.d.local_of(clause).path}" if clause.kind == "local" and clause.map else "in CLAUSES"
        return f"no twin for the clause {clause.text!r} {where}; add it, or run with opaque='true'"


class GlueMeaning:
    """The meaning a glue gives the clauses, asked through client as given
    (csdfbmc.protocol.checked is what the commands give)."""
    readable = False
    by_prose = False

    def __init__(self, client: ClientLike):
        self.client = client

    def _binding(self, clause: Clause, reads: tuple[str, ...], env: dict) -> dict:
        missing = [n for n in reads if n not in env]
        if missing:
            raise KeyError(f"the clause {clause.text!r} reads {missing}, which this step does not bind; it may read {sorted(env)}")
        return {n: env[n] for n in reads}

    def names(self, clause: Clause, role: str) -> set[str] | None:
        analysis = self.client.analyse(clause.text, role)
        return None if analysis.opaque else set(analysis.reads)

    def names_beyond(self, clause: Clause, own_id: str | None) -> set[str] | None:
        return None

    def _ask(self, clause: Clause, role: str, envs: list[dict]) -> list | None:
        analysis = self.client.analyse(clause.text, role)
        if analysis.opaque:
            return None
        bindings = [self._binding(clause, analysis.reads, env) for env in envs]
        ask = self.client.guard if role == "guard" else self.client.post
        return ask(clause.text, bindings)

    def prefetch(self, clause: Clause, role: str, envs: list[dict]) -> None:
        """Asks the glue about every env in one request."""
        self._ask(clause, role, envs)

    def evaluate(self, clause: Clause, role: str, env: dict):
        results = self._ask(clause, role, [env])
        return OPAQUE if results is None else results[0]

    def opaque_message(self, clause: Clause) -> str:
        return f"the {self.client.language} glue says the clause {clause.text!r} is not formalised; formalise it, or run with opaque='true'"


class ReservedOpaque:
    """inner, but the reserved clause opaque means nothing whatever inner
    would say: it is not formalised, and nobody is asked about it."""

    def __init__(self, inner: Meaning):
        self.inner = inner
        self.readable = inner.readable
        self.by_prose = inner.by_prose

    def names(self, clause: Clause, role: str) -> set[str] | None:
        return None if is_reserved_opaque(clause) else self.inner.names(clause, role)

    def names_beyond(self, clause: Clause, own_id: str | None) -> set[str] | None:
        return None if is_reserved_opaque(clause) else self.inner.names_beyond(clause, own_id)

    def prefetch(self, clause: Clause, role: str, envs: list[dict]) -> None:
        if not is_reserved_opaque(clause):
            self.inner.prefetch(clause, role, envs)

    def evaluate(self, clause: Clause, role: str, env: dict):
        return OPAQUE if is_reserved_opaque(clause) else self.inner.evaluate(clause, role, env)

    def opaque_message(self, clause: Clause) -> str:
        if is_reserved_opaque(clause):
            return f"the clause {clause.text!r} is not formalised yet; formalise it, or run with opaque='true'"
        return self.inner.opaque_message(clause)
