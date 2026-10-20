"""What the clauses of a diagram mean to the walk: what a hand-written or
constrain clause reads, and what it evaluates to. The generated clauses of
an expansion have their meaning built into the walk.

TwinsMeaning takes it from a twins module, keyed by the clause's prose (or a
constrain's formal expression); GlueMeaning from a glue of the evaluator
protocol, by the clause's text.
"""
from __future__ import annotations

import inspect

from csdfbmc.formal import FORMAL_GLOBALS
from csdfbmc.model import Clause, Diagram
from csdfbmc.protocol import Memoising, Validating
from csdfbmc.reduce import formal_names, names_beyond
from csdfbmc.twins import Twins, apply, check_pure

# What a clause with no meaning to evaluate stands for: no twin, or a glue
# that says the clause is not formalised yet.
OPAQUE = object()


class TwinsMeaning:
    # readable: the clauses can be read, as source, to check SYMMETRIC.
    readable = True
    # by_prose: the twins' CLAUSES give the meaning.
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
    """The meaning a glue gives the clauses. client is a Client of the
    protocol; it is asked through Memoising and Validating."""
    readable = False
    by_prose = False

    def __init__(self, client):
        self.client = Memoising(Validating(client))

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
