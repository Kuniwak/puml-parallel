"""The reference glue of the evaluator protocol, for clauses written in the
pure subset of Python that csdfbmc.checks.check_pure enforces.

evaluator(args) returns the handler of the protocol's requests, which
csdfbmc loads in-process with --evaluator-module; run as a command
(python3 -m csdfbmc.python_glue), it speaks the same requests as JSON lines
on its standard input and output.
"""
from __future__ import annotations

import ast
import importlib.util
from dataclasses import dataclass
from typing import Callable

from csdfbmc.formal import FORMAL_GLOBALS, free_names, names_beyond, tells_apart
from csdfbmc.protocol import Analysis, handler
from csdfbmc.checks import check_pure
from csdfbmc.values import freeze

LANGUAGE = "python"
# What a clause not formalised yet was written as before the tool word
# @opaque, which the checker takes without asking a glue.
_FORMER_OPAQUE = ("OPAQUE", "opaque")


def _post_writes(tree: ast.Expression) -> list[str]:
    """The variables a post gives a value to: the keys of its assignments,
    which are dict displays with constant string keys, in a list, set or
    tuple display or a comprehension."""
    body = tree.body
    if isinstance(body, (ast.ListComp, ast.SetComp, ast.GeneratorExp)):
        dicts = [body.elt]
    elif isinstance(body, (ast.List, ast.Set, ast.Tuple)):
        dicts = body.elts
    else:
        raise ValueError("a post is a list or set of assignments, such as [{'x': x + 1}] or [{'y': v} for v in ys]")
    keys = []
    for d in dicts:
        if not isinstance(d, ast.Dict) or not all(isinstance(k, ast.Constant) and isinstance(k.value, str) for k in d.keys):
            raise ValueError("each assignment of a post is a dict display whose keys are the variables, written as strings")
        keys.append(sorted(k.value for k in d.keys))
    if any(k != keys[0] for k in keys):
        raise ValueError("the assignments of a post must give the same variables a value")
    return keys[0] if keys else []


class Glue:
    """Clauses in the pure subset of Python. defs are the paths of modules
    whose public names every clause may use: constants and helpers shared by
    the clauses, checked against the pure subset like a checks module."""

    def __init__(self, defs: list[str]):
        self.globals = dict(FORMAL_GLOBALS)
        for path in defs:
            self.globals.update(_load_defs(path))
        self._compiled: dict[tuple[str, str], tuple] = {}

    def _compile(self, clause: str, role: str):
        key = (clause, role)
        if key not in self._compiled:
            if clause.strip() in _FORMER_OPAQUE:
                raise ValueError(f"{clause.strip()!r} is not a clause of this glue; for a clause not formalised yet, write the tool word @opaque")
            check_pure(clause, f"<clause {clause!r}>")
            tree = ast.parse(clause, mode="eval")
            writes = _post_writes(tree) if role == "post" else []
            reads = sorted(free_names(tree) - set(self.globals))
            self._compiled[key] = (compile(tree, f"<clause {clause!r}>", "eval"), reads, writes)
        return self._compiled[key]

    def _eval(self, clause: str, role: str, binding: dict):
        code, _, _ = self._compile(clause, role)
        return eval(code, self.globals, dict(binding))

    def analyse(self, clause: str, role: str) -> Analysis:
        _, reads, writes = self._compile(clause, role)
        return Analysis(opaque=False, reads=tuple(reads), writes=tuple(writes))

    def beyond(self, clause: str, role: str, own_id: str | None) -> list[str]:
        """The names the clause reads, but a map read only at own_id."""
        tree = ast.parse(clause, mode="eval")
        return sorted(names_beyond(tree, own_id) - set(self.globals))

    def symmetric(self, clause: str, role: str, params: list[str], maps: list[str], groups: list[list[str]]) -> str | None:
        """How the clause tells two IDs of a group apart, or None."""
        found = tells_apart(ast.parse(clause, mode="eval"), set(params), set(maps), [frozenset(g) for g in groups])
        return None if found is None else f"{found[1]}, so the IDs are not interchangeable"

    def atom_guard(self, atom: dict, values: list) -> bool:
        source, names = _atom_source(atom)
        return bool(self._eval(source, "guard", dict(zip(names, values))))

    def atom_post(self, atom: dict, values: list) -> list[list]:
        """The values after the step, of the primed arguments in order, that
        the atom allows from values, those of the others in order."""
        source, names = _atom_source(atom)
        primed = [n for n, p in zip(names, atom["primed"]) if p]
        before = [n for n, p in zip(names, atom["primed"]) if not p]
        targets, chosen = _post_shape(source, primed)
        options = self._eval(chosen, "guard", dict(zip(before, values)))
        if targets.is_tuple:
            return [[freeze(v) for v in _ordered(targets.names, primed, o)] for o in options]
        return [[freeze(o)] for o in options]

    def guard(self, clause: str, binding: dict) -> bool:
        return bool(self._eval(clause, "guard", binding))

    def post(self, clause: str, binding: dict) -> list[dict]:
        return [{k: freeze(v) for k, v in a.items()} for a in self._eval(clause, "post", binding)]


# The name an atom's argument i is read as; an atom's own text may not use it.
_HOLE = "_csdf_hole_"


def _atom_source(atom: dict) -> tuple[str, list[str]]:
    """The Python an atom is, its holes read as names, and those names by
    argument index."""
    text, holes = atom["text"], atom["holes"]
    if any(_HOLE in t for t in text):
        raise ValueError(f"an atom of this glue may not use the names {_HOLE}N, which stand for its holes")
    names = [f"{_HOLE}{i}" for i in range(len(atom["primed"]))]
    source = text[0] + "".join(names[h] + t for h, t in zip(holes, text[1:]))
    return source, names


_POST_SHAPE = ("a post atom of this glue is $x' == expression, $x' in expression, or a tuple of every "
               "value after the step, ($x', $y') == expression or in one, where expression reads none of them")


@dataclass(frozen=True)
class _Targets:
    names: tuple[str, ...]
    is_tuple: bool


def _post_shape(source: str, primed: list[str]) -> tuple[_Targets, str]:
    """What a post atom gives a value to, and the source of the collection
    of what it may choose for them."""
    body = ast.parse(source, mode="eval").body
    if not (isinstance(body, ast.Compare) and len(body.ops) == 1 and isinstance(body.ops[0], (ast.Eq, ast.In))):
        raise ValueError(_POST_SHAPE)
    left, right = body.left, body.comparators[0]
    if isinstance(left, ast.Name):
        targets = _Targets((left.id,), False)
    elif isinstance(left, ast.Tuple) and all(isinstance(e, ast.Name) for e in left.elts):
        targets = _Targets(tuple(e.id for e in left.elts), True)
    else:
        raise ValueError(_POST_SHAPE)
    if sorted(targets.names) != sorted(primed) or free_names(right) & set(primed):
        raise ValueError(_POST_SHAPE)
    right_source = ast.unparse(right)
    return targets, (f"[{right_source}]" if isinstance(body.ops[0], ast.Eq) else right_source)


def _ordered(names: tuple[str, ...], primed: list[str], option) -> list:
    """option, a tuple of values for names, reordered as primed."""
    by_name = dict(zip(names, option))
    return [by_name[n] for n in primed]


def _load_defs(path: str) -> dict:
    with open(path, encoding="utf-8") as f:
        check_pure(f.read(), path)
    spec = importlib.util.spec_from_file_location("csdf_clause_defs", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return {k: v for k, v in vars(module).items() if not k.startswith("_")}


def evaluator(args: list[str]) -> Callable[[dict], dict]:
    """The handler of the protocol's requests; args are the paths of the
    modules of shared definitions."""
    glue = Glue(args)
    return handler(LANGUAGE, analyse=glue.analyse, guard=glue.guard, post=glue.post,
                   beyond=glue.beyond, symmetric=glue.symmetric, atom_guard=glue.atom_guard,
                   atom_post=glue.atom_post)


if __name__ == "__main__":
    import sys

    from csdfbmc.protocol import serve
    serve(evaluator(sys.argv[1:]), sys.stdin, sys.stdout)
