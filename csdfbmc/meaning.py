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
from csdfbmc.twins import Twins, apply, check_pure, reads

# What a clause with no meaning to evaluate stands for: no twin, a glue
# that says the clause is not formalised yet, or the tool word @opaque.
OPAQUE = object()


class Meaning(Protocol):
    """What the walk needs to know of the hand-written and constrain clauses."""
    # readable: the clauses can be checked for SYMMETRIC, by reading them as
    # source (twins) or by asking whoever reads them (tells_apart).
    readable: bool
    # by_prose: the twins' CLAUSES give the meaning.
    by_prose: bool

    def names(self, clause: Clause, role: str) -> set[str] | None: ...
    def names_beyond(self, clause: Clause, own_id: str | None) -> set[str] | None: ...
    def tells_apart(self, clause: Clause, role: str, params: set[str], maps: set[str], groups: list[frozenset]) -> str | None: ...
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
        return formal_names(clause.formal) if getattr(fn, "formal", False) else reads(fn)

    def names_beyond(self, clause: Clause, own_id: str | None) -> set[str] | None:
        """The names a clause reads but a map read only as m[own_id], when
        that can be told (a formal expression); None otherwise."""
        fn = self._twin(clause)
        if fn is not None and getattr(fn, "formal", False):
            return names_beyond(clause.formal, own_id)
        return None

    def prefetch(self, clause: Clause, role: str, envs: list[dict]) -> None:
        pass

    def tells_apart(self, clause: Clause, role: str, params: set[str], maps: set[str], groups: list[frozenset]) -> str | None:
        """None: the twins modules and formal expressions are read as source
        by the symmetry check itself."""
        return None

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
    (csdfbmc.protocol.checked is what the commands give). What the walk's
    reductions need of a clause is asked of the glue when it answers it, and
    is unknown otherwise."""
    by_prose = False

    def __init__(self, client: ClientLike):
        self.client = client
        self.readable = "symmetric" in client.supports

    def tells_apart(self, clause: Clause, role: str, params: set[str], maps: set[str], groups: list[frozenset]) -> str | None:
        return self.client.symmetric(clause.text, role, sorted(params), sorted(maps), [sorted(g) for g in groups])

    def _binding(self, clause: Clause, reads: tuple[str, ...], env: dict) -> dict:
        missing = [n for n in reads if n not in env]
        if missing:
            raise KeyError(f"the clause {clause.text!r} reads {missing}, which this step does not bind; it may read {sorted(env)}")
        return {n: env[n] for n in reads}

    def names(self, clause: Clause, role: str) -> set[str] | None:
        analysis = self.client.analyse(clause.text, role)
        return None if analysis.opaque else set(analysis.reads)

    def names_beyond(self, clause: Clause, own_id: str | None) -> set[str] | None:
        if "beyond" not in self.client.supports:
            return None
        return set(self.client.beyond(clause.text, "guard", own_id))

    def writes(self, clause: Clause) -> set[str] | None:
        """The variables a post clause gives a value to, as the glue says."""
        analysis = self.client.analyse(clause.text, "post")
        return None if analysis.opaque else set(analysis.writes)

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


class OpaqueWord:
    """inner, but the tool word @opaque means nothing whatever inner
    would say: it is not formalised, and nobody is asked about it."""

    def __init__(self, inner: Meaning):
        self.inner = inner
        self.readable = inner.readable
        self.by_prose = inner.by_prose

    def names(self, clause: Clause, role: str) -> set[str] | None:
        return None if clause.opaque else self.inner.names(clause, role)

    def names_beyond(self, clause: Clause, own_id: str | None) -> set[str] | None:
        return None if clause.opaque else self.inner.names_beyond(clause, own_id)

    def tells_apart(self, clause: Clause, role: str, params: set[str], maps: set[str], groups: list[frozenset]) -> str | None:
        return None if clause.opaque else self.inner.tells_apart(clause, role, params, maps, groups)

    def prefetch(self, clause: Clause, role: str, envs: list[dict]) -> None:
        if not clause.opaque:
            self.inner.prefetch(clause, role, envs)

    def evaluate(self, clause: Clause, role: str, env: dict):
        return OPAQUE if clause.opaque else self.inner.evaluate(clause, role, env)

    def writes(self, clause: Clause) -> set[str] | None:
        """What inner says a post clause gives a value to; for a meaning that
        can say it (GlueMeaning)."""
        return None if clause.opaque else self.inner.writes(clause)

    def opaque_message(self, clause: Clause) -> str:
        if clause.opaque:
            return f"the clause {clause.text!r} is not formalised yet; formalise it, or run with opaque='true'"
        return self.inner.opaque_message(clause)


class Renaming:
    """inner, with each renamed clause read in the names of the predicate it
    is in: a name it reads stands for the value its renaming binds it to."""

    def __init__(self, inner: Meaning):
        self.inner = inner
        self.readable = inner.readable
        self.by_prose = inner.by_prose

    @staticmethod
    def _outer(clause: Clause, names: set[str] | None) -> set[str] | None:
        to = dict(clause.rename)
        return None if names is None else {to.get(n, n) for n in names}

    @staticmethod
    def _env(clause: Clause, env: dict) -> dict:
        if not clause.rename:
            return env
        missing = [t for _, t in clause.rename if t not in env]
        if missing:
            raise KeyError(f"the clause {clause.text!r} is renamed onto {missing}, which this step does not bind; it may read {sorted(env)}")
        return {**env, **{f: env[t] for f, t in clause.rename}}

    def names(self, clause: Clause, role: str) -> set[str] | None:
        return self._outer(clause, self.inner.names(clause, role))

    def names_beyond(self, clause: Clause, own_id: str | None) -> set[str] | None:
        inner_id = next((f for f, t in clause.rename if t == own_id), own_id)
        return self._outer(clause, self.inner.names_beyond(clause, inner_id))

    def tells_apart(self, clause: Clause, role: str, params: set[str], maps: set[str], groups: list[frozenset]) -> str | None:
        """Asked in the clause's own names: a name renamed onto a symmetric
        one is symmetric there, and one renamed away is not the outer one."""
        def inner(names: set[str]) -> set[str]:
            renamed_from = {f for f, _ in clause.rename}
            return {f for f, t in clause.rename if t in names} | {n for n in names if n not in renamed_from}
        return self.inner.tells_apart(clause, role, inner(params), inner(maps), groups)

    def writes(self, clause: Clause) -> set[str] | None:
        """What inner says a post clause gives a value to; for a meaning that
        can say it (GlueMeaning)."""
        return self._outer(clause, self.inner.writes(clause))

    def prefetch(self, clause: Clause, role: str, envs: list[dict]) -> None:
        self.inner.prefetch(clause, role, [self._env(clause, env) for env in envs])

    def evaluate(self, clause: Clause, role: str, env: dict):
        result = self.inner.evaluate(clause, role, self._env(clause, env))
        if role == "post" and clause.rename and isinstance(result, (list, tuple)):
            to = dict(clause.rename)
            result = [{to.get(k, k): v for k, v in a.items()} for a in result]
        return result

    def opaque_message(self, clause: Clause) -> str:
        return self.inner.opaque_message(clause)


def same_value(a, b) -> bool:
    """Whether two values are one value: of one kind as well as equal, so
    that 1 is not True and not 1.0, all the way into tuples and sets."""
    if type(a) is not type(b):
        return False
    if isinstance(a, tuple):
        return len(a) == len(b) and all(same_value(x, y) for x, y in zip(a, b))
    return a == b


class EqualityWord:
    """inner, but the tool word @eq(a, b) means that the values named a and b
    are equal, whatever inner would say: nobody is asked about it. In a post
    it admits the one assignment of nothing where they are equal, and none
    where they are not."""

    def __init__(self, inner: Meaning):
        self.inner = inner
        self.readable = inner.readable
        self.by_prose = inner.by_prose

    def names(self, clause: Clause, role: str) -> set[str] | None:
        return set(clause.eq) if clause.eq else self.inner.names(clause, role)

    def names_beyond(self, clause: Clause, own_id: str | None) -> set[str] | None:
        return set(clause.eq) if clause.eq else self.inner.names_beyond(clause, own_id)

    def tells_apart(self, clause: Clause, role: str, params: set[str], maps: set[str], groups: list[frozenset]) -> str | None:
        # An equality of two names names no ID and orders none.
        return None if clause.eq else self.inner.tells_apart(clause, role, params, maps, groups)

    def writes(self, clause: Clause) -> set[str] | None:
        return set() if clause.eq else self.inner.writes(clause)

    def prefetch(self, clause: Clause, role: str, envs: list[dict]) -> None:
        if not clause.eq:
            self.inner.prefetch(clause, role, envs)

    def evaluate(self, clause: Clause, role: str, env: dict):
        if not clause.eq:
            return self.inner.evaluate(clause, role, env)
        missing = [n for n in clause.eq if n not in env]
        if missing:
            raise KeyError(f"the clause {clause.text!r} reads {missing}, which this step does not bind; it may read {sorted(env)}")
        equal = same_value(env[clause.eq[0]], env[clause.eq[1]])
        return equal if role == "guard" else ([{}] if equal else [])

    def opaque_message(self, clause: Clause) -> str:
        return self.inner.opaque_message(clause)


def standard(inner: Meaning) -> Meaning:
    """inner as every walk takes it: with the tool words @opaque and @eq, and
    each renamed clause read in the names of its predicate."""
    return OpaqueWord(EqualityWord(Renaming(inner)))
