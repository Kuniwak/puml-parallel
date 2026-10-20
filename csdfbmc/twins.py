"""The formal twins of the clauses, and the finite domains the values live in.

A twins module is ordinary Python restricted to a pure subset: it defines
DOMAINS (name -> iterable of values, for state variables and event parameters)
and CLAUSES (the clause's prose -> a function of the names the clause reads:
event parameters, the variables before the step, and the variables after it
spelled with a trailing underscore). Nothing in it may assign inside a
function, loop, raise, read or write the outside world, or use floats, so that
evaluating a clause is deterministic and decidable by enumeration.
"""
from __future__ import annotations

import ast
import importlib.util
import inspect
from dataclasses import dataclass, field
from typing import Callable, Iterable


class PurityError(Exception):
    """The twins module steps outside the pure subset, or lacks its tables."""


_FORBIDDEN_NODES = {
    ast.For: "a for loop", ast.AsyncFor: "an async for loop", ast.While: "a while loop",
    ast.Try: "a try statement", ast.TryStar: "a try statement", ast.Raise: "a raise statement",
    ast.Global: "a global statement", ast.Nonlocal: "a nonlocal statement", ast.Delete: "a delete statement",
    ast.AugAssign: "an augmented assignment", ast.Await: "await", ast.Yield: "yield", ast.YieldFrom: "yield from",
    ast.With: "a with statement", ast.AsyncWith: "a with statement",
}
_FORBIDDEN_MODULES = {"random", "os", "sys", "io", "subprocess", "time", "datetime", "socket", "secrets", "uuid"}
_FORBIDDEN_CALLS = {"print", "input", "open", "exec", "eval", "compile", "globals", "locals", "setattr", "delattr", "__import__"}


def check_pure(source: str, path: str = "<twins>") -> None:
    tree = ast.parse(source, filename=path)
    for node in ast.walk(tree):
        for kind, what in _FORBIDDEN_NODES.items():
            if isinstance(node, kind):
                raise PurityError(f"{path}:{node.lineno}: {what} is not allowed in a twins module")
        if isinstance(node, ast.Constant) and isinstance(node.value, float):
            raise PurityError(f"{path}:{node.lineno}: a float literal is not allowed; use integers or fractions")
        if isinstance(node, (ast.Import, ast.ImportFrom)):
            names = [a.name for a in node.names] if isinstance(node, ast.Import) else [node.module or ""]
            for name in names:
                if name.split(".")[0] in _FORBIDDEN_MODULES:
                    raise PurityError(f"{path}:{node.lineno}: importing {name} is not allowed in a twins module")
        if isinstance(node, ast.Call) and isinstance(node.func, ast.Name) and node.func.id in _FORBIDDEN_CALLS:
            raise PurityError(f"{path}:{node.lineno}: calling {node.func.id} is not allowed in a twins module")
        if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef, ast.Lambda)):
            body = node.body if isinstance(node.body, list) else [node.body]
            for stmt in body:
                if isinstance(stmt, (ast.Assign, ast.AnnAssign)):
                    raise PurityError(f"{path}:{stmt.lineno}: an assignment inside a function is not allowed; a clause is one expression")


@dataclass
class Twins:
    domains: dict[str, Iterable]
    clauses: dict[str, Callable[..., bool]]
    # invariants: name -> predicate over the state variables and ghosts it
    # reads, checked at every reachable state that binds all of them.
    invariants: dict[str, Callable[..., bool]] = field(default_factory=dict)
    # ghosts: name -> (initial value, {event name: update}). A ghost is a value
    # the diagram does not keep - a running total, a count - that follows the
    # events; an update reads the ghosts, the event's parameters and the state
    # variables before the step, and returns the new value. A ghost with no
    # update for an event keeps its value.
    ghosts: dict[str, tuple[object, dict[str, Callable[..., object]]]] = field(default_factory=dict)

    def domain(self, name: str) -> list:
        if name not in self.domains:
            raise KeyError(f"no domain declared for {name!r}; add it to DOMAINS")
        return list(self.domains[name])

    def evaluate(self, clause: str, env: dict) -> bool:
        """Applies the clause's twin to the names it reads, taken from env."""
        return bool(apply(self.clauses[clause], env, what=f"the twin of {clause!r}"))

    def binds(self, fn: Callable, env: dict) -> bool:
        return all(p in env for p in inspect.signature(fn).parameters)


def apply(fn: Callable, env: dict, *, what: str):
    """Calls fn with the names its signature asks for, taken from env."""
    params = inspect.signature(fn).parameters
    missing = [p for p in params if p not in env]
    if missing:
        raise KeyError(f"{what} reads {missing}, which this step does not bind; it may read {sorted(env)}")
    return fn(**{p: env[p] for p in params})


def load_twins(path: str) -> Twins:
    with open(path, encoding="utf-8") as f:
        source = f.read()
    check_pure(source, path)
    spec = importlib.util.spec_from_file_location("csdf_twins", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    for table in ("DOMAINS", "CLAUSES"):
        if not isinstance(getattr(module, table, None), dict):
            raise PurityError(f"{path}: the module must define {table} as a dict")
    for table in ("INVARIANTS", "GHOSTS"):
        if not isinstance(getattr(module, table, {}), dict):
            raise PurityError(f"{path}: {table} must be a dict when defined")
    return Twins(
        domains=dict(module.DOMAINS), clauses=dict(module.CLAUSES),
        invariants=dict(getattr(module, "INVARIANTS", {})), ghosts=dict(getattr(module, "GHOSTS", {})),
    )
