"""The checks module: the environment of the diagram, and what the walk
checks besides the clauses.

A checks module is ordinary Python restricted to a pure subset. It defines
TYPES (a type -> iterable of its values, its carrier set, or a sample of
them, csdfbmc.values.sample) and, when wanted,
TYPEOF (a name -> its type; a name it does not list has the type of its own
name), ENVIRONMENT (declares the atoms that are functions of the
environment, and gives them; see csdfbmc.environment),
GHOSTS and INVARIANTS. An earlier csdfbmc read DOMAINS; a module
defining it is refused. The clauses themselves are read through a glue of
the evaluator protocol (csdfbmc.protocol), so a module defining CLAUSES, which an
earlier csdfbmc read, is refused: write the clauses in the glue's language
instead.

A ghost is a value the diagram does not keep - a running total, a count -
that follows the events; an invariant is a predicate over the state
variables and ghosts it reads. Values must be hashable (tuples, FrozenSet,
FrozenMap); dicts, lists and sets are frozen into those, but a dict or set
Python builds has already taken 1, True and 1.0 for one key, so one that
holds keys of those kinds is built as a FrozenMap or FrozenSet. Nothing in the
module may assign inside a function, loop with a statement, raise, read or
write the outside world, or use floats, so that a check is deterministic;
comprehensions are expressions and are allowed. SYMMETRIC lists the types
whose values --symmetry may rename (csdfbmc.symmetry).
"""
from __future__ import annotations

import ast
import importlib.util
import inspect
from dataclasses import dataclass, field
from typing import TYPE_CHECKING, Callable, Iterable

from csdfbmc.values import samples_in

if TYPE_CHECKING:
    from csdfbmc.environment import Given


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
    # sources: (path, source) of the module, which the symmetry check reads.
    sources: list[tuple[str, str]] = field(default_factory=list)
    # typeof: a name -> its type; a name it does not list has the type of
    # its own name.
    typeof: dict[str, str] = field(default_factory=dict)
    # environment: the functions of the environment it declares, each by
    # its name or an atom of it -> its value, True or False, a pure function
    # of its arguments in order, or None to leave it to --env
    # (csdfbmc.environment).
    environment: dict[str, "Given | None"] = field(default_factory=dict)
    # symmetric: the types whose values, IDs, --symmetry may rename.
    symmetric: tuple[str, ...] = ()

    @property
    def samples(self) -> frozenset[str]:
        """The types whose values are a sample of the type, which has more
        (csdfbmc.values.sample), or are built of one by record or
        partial_map (csdfbmc.values.samples_in); computed from types, so it
        follows them."""
        return frozenset(t for t, vs in self.types.items() if samples_in(vs))

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

    def carrier(self, type_: str) -> list:
        """The values of the type type_, which an ∃ ranges over."""
        if type_ in self.types:
            return list(self.types[type_])
        raise KeyError(f"no type {type_!r} in TYPES, which an ∃ ranges over")

    def domain(self, name: str) -> list:
        if self.declares(name):
            return list(self.types[self.type_of(name)])
        raise KeyError(f"no type declared for {name!r}; add it to TYPES, or give it a type with TYPEOF")

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


def property_reads(checks: Checks, why: str) -> set[str]:
    """The names the invariants and the ghost updates read; refused when one
    takes any keyword, since what it reads cannot be told."""
    out: set[str] = set()
    what = [(f"the invariant {name!r}", fn) for name, fn in checks.invariants.items()]
    what += [(f"the update of ghost {g!r} on {event}", fn) for g, (_, updates) in checks.ghosts.items() for event, fn in updates.items()]
    for name, fn in what:
        names = reads(fn)
        if names is None:
            raise ValueError(f"{why} needs to know what {name} reads, but it takes any keyword; name its parameters")
        out |= names
    return out


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
    if hasattr(module, "DOMAINS"):
        raise PurityError(f"{path}: DOMAINS is no longer read; declare the values as TYPES (a type -> its values), "
                          "and give a name the type of another name with TYPEOF (a name -> a type)")
    if not isinstance(getattr(module, "TYPES", None), dict):
        raise PurityError(f"{path}: the module must define TYPES as a dict (a type -> its values)")
    typeof = getattr(module, "TYPEOF", {})
    if not isinstance(typeof, dict):
        raise PurityError(f"{path}: TYPEOF must be a dict (a name -> a type) when defined")
    for name, t in typeof.items():
        if t not in module.TYPES:
            raise PurityError(f"{path}: TYPEOF gives {name!r} the type {t!r}, which TYPES does not declare")
    symmetric = getattr(module, "SYMMETRIC", ())
    if not isinstance(symmetric, (list, tuple, set, frozenset)) or not all(isinstance(t, str) for t in symmetric):
        raise PurityError(f"{path}: SYMMETRIC must list the names of types when defined")
    if hasattr(module, "CLAUSES"):
        raise PurityError(f"{path}: CLAUSES is no longer read; write the clauses in the "
                          "language of a glue (csdfbmc.python_glue reads Python) and read them with --evaluator-module")
    for table in ("INVARIANTS", "GHOSTS", "ENVIRONMENT"):
        if not isinstance(getattr(module, table, {}), dict):
            raise PurityError(f"{path}: {table} must be a dict when defined")
    environment = dict(getattr(module, "ENVIRONMENT", {}))
    for name, value in environment.items():
        if not isinstance(name, str):
            raise PurityError(f"{path}: ENVIRONMENT names a function by {name!r}; name it by an atom of it, as text")
        if value is not None and not isinstance(value, bool) and not callable(value):
            raise PurityError(f"{path}: ENVIRONMENT gives {name!r} the value {value!r}; give True, False, a function of its arguments in order, "
                              "or None to leave it to --env")
    return Checks(
        types=dict(module.TYPES), typeof=dict(typeof), environment=environment,
        invariants=dict(getattr(module, "INVARIANTS", {})), ghosts=dict(getattr(module, "GHOSTS", {})),
        sources=[(path, source)], symmetric=tuple(sorted(symmetric)),
    )
