"""The checks module: the finite domains of the parameters, and what the
walk checks besides the clauses.

A checks module is ordinary Python restricted to a pure subset. It defines
DOMAINS (event parameter -> iterable of values) and, when wanted, GHOSTS,
INVARIANTS and SYMMETRIC. The clauses themselves are read through a glue of
the evaluator protocol (csdfbmc.protocol), so a module defining CLAUSES, which an
earlier csdfbmc read, is refused: write the clauses in the glue's language
instead.

A ghost is a value the diagram does not keep - a running total, a count -
that follows the events; an invariant is a predicate over the state
variables and ghosts it reads. Values must be hashable (tuples, frozenset,
FrozenMap); dicts, lists and sets are frozen into those. Nothing in the
module may assign inside a function, loop with a statement, raise, read or
write the outside world, or use floats, so that a check is deterministic;
comprehensions are expressions and are allowed. SYMMETRIC, when defined,
lists the instance-ID parameters whose values are interchangeable (see
csdfbmc.reduce).
"""
from __future__ import annotations

import ast
import importlib.util
import inspect
from dataclasses import dataclass, field
from typing import Callable, Iterable


class PurityError(Exception):
    """The checks module steps outside the pure subset, or lacks its tables."""


_FORBIDDEN_NODES = {
    ast.For: "a for loop", ast.AsyncFor: "an async for loop", ast.While: "a while loop",
    ast.Try: "a try statement", ast.TryStar: "a try statement", ast.Raise: "a raise statement",
    ast.Global: "a global statement", ast.Nonlocal: "a nonlocal statement", ast.Delete: "a delete statement",
    ast.AugAssign: "an augmented assignment", ast.Await: "await", ast.Yield: "yield", ast.YieldFrom: "yield from",
    ast.With: "a with statement", ast.AsyncWith: "a with statement",
}
_FORBIDDEN_MODULES = {"random", "os", "sys", "io", "subprocess", "time", "datetime", "socket", "secrets", "uuid"}
_FORBIDDEN_CALLS = {"print", "input", "open", "exec", "eval", "compile", "globals", "locals", "setattr", "delattr", "__import__"}


def check_pure(source: str, path: str = "<checks>") -> None:
    tree = ast.parse(source, filename=path)
    for node in ast.walk(tree):
        for kind, what in _FORBIDDEN_NODES.items():
            if isinstance(node, kind):
                raise PurityError(f"{path}:{node.lineno}: {what} is not allowed in a checks module")
        if isinstance(node, ast.Constant) and isinstance(node.value, float):
            raise PurityError(f"{path}:{node.lineno}: a float literal is not allowed; use integers or fractions")
        if isinstance(node, (ast.Import, ast.ImportFrom)):
            names = [a.name for a in node.names] if isinstance(node, ast.Import) else [node.module or ""]
            for name in names:
                if name.split(".")[0] in _FORBIDDEN_MODULES:
                    raise PurityError(f"{path}:{node.lineno}: importing {name} is not allowed in a checks module")
        if isinstance(node, ast.Call) and isinstance(node.func, ast.Name) and node.func.id in _FORBIDDEN_CALLS:
            raise PurityError(f"{path}:{node.lineno}: calling {node.func.id} is not allowed in a checks module")
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.Lambda)):
            body = node.body if isinstance(node.body, list) else [node.body]
            for stmt in body:
                if isinstance(stmt, (ast.Assign, ast.AnnAssign)):
                    raise PurityError(f"{path}:{stmt.lineno}: an assignment inside a function is not allowed; a clause is one expression")


@dataclass
class Checks:
    # types: a type -> the finite values it ranges over (its carrier set).
    types: dict[str, Iterable]
    # invariants: name -> predicate over the state variables and ghosts it
    # reads, checked at every reachable state that binds all of them.
    invariants: dict[str, Callable[..., bool]] = field(default_factory=dict)
    # ghosts: name -> (initial value, {event name: update}). A ghost is a value
    # the diagram does not keep - a running total, a count - that follows the
    # events; an update reads the ghosts, the event's parameters and the state
    # variables before the step, and returns the new value. A ghost with no
    # update for an event keeps its value.
    ghosts: dict[str, tuple[object, dict[str, Callable[..., object]]]] = field(default_factory=dict)
    # symmetric: the instance-ID parameters whose values are interchangeable,
    # which --symmetry lets the walk store one state per renaming of.
    symmetric: tuple[str, ...] = ()
    # sources: (path, source) of the module, which the symmetry check reads.
    sources: list[tuple[str, str]] = field(default_factory=list)
    # typeof: a name -> its type; a name it does not list has the type of
    # its own name.
    typeof: dict[str, str] = field(default_factory=dict)

    def type_of(self, name: str) -> str:
        return self.typeof.get(name, name)

    def declares(self, name: str) -> bool:
        """Whether name has a type, and so values to range over."""
        return self.type_of(name) in self.types

    def typed_names(self) -> dict[str, list]:
        """Every name that has a type, with its values: the names TYPEOF
        lists, and each type under its own name."""
        names = {name: list(values) for name, values in self.types.items()}
        names.update((name, list(self.types[t])) for name, t in self.typeof.items() if t in self.types)
        return names

    def domain(self, name: str) -> list:
        if self.declares(name):
            return list(self.types[self.type_of(name)])
        raise KeyError(f"no domain declared for {name!r}; add it to DOMAINS")

    def binds(self, fn: Callable, env: dict) -> bool:
        return all(p in env for p in inspect.signature(fn).parameters)


def reads(fn: Callable) -> set[str] | None:
    """The names an invariant or a ghost update reads: what its
    signature asks for; None when it takes any keyword, and so may read
    anything."""
    params = inspect.signature(fn).parameters.values()
    if any(p.kind is inspect.Parameter.VAR_KEYWORD for p in params):
        return None
    return {p.name for p in params}


def apply(fn: Callable, env: dict, *, what: str):
    """Calls fn with the names its signature asks for, taken from env."""
    params = inspect.signature(fn).parameters
    missing = [p for p in params if p not in env]
    if missing:
        raise KeyError(f"{what} reads {missing}, which this step does not bind; it may read {sorted(env)}")
    return fn(**{p: env[p] for p in params})


def load_checks(path: str) -> Checks:
    with open(path, encoding="utf-8") as f:
        source = f.read()
    check_pure(source, path)
    spec = importlib.util.spec_from_file_location("csdf_checks", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    if not isinstance(getattr(module, "DOMAINS", None), dict):
        raise PurityError(f"{path}: the module must define DOMAINS as a dict")
    if not isinstance(getattr(module, "SYMMETRIC", ()), (list, tuple, set, frozenset)):
        raise PurityError(f"{path}: SYMMETRIC must be a list of instance-ID parameters when defined")
    if hasattr(module, "CLAUSES"):
        raise PurityError(f"{path}: CLAUSES is no longer read; write the clauses in the "
                          "language of a glue (csdfbmc.python_glue reads Python) and read them with --evaluator-module")
    for table in ("INVARIANTS", "GHOSTS"):
        if not isinstance(getattr(module, table, {}), dict):
            raise PurityError(f"{path}: {table} must be a dict when defined")
    return Checks(
        types=dict(module.DOMAINS),
        invariants=dict(getattr(module, "INVARIANTS", {})), ghosts=dict(getattr(module, "GHOSTS", {})),
        symmetric=tuple(getattr(module, "SYMMETRIC", ())), sources=[(path, source)],
    )
