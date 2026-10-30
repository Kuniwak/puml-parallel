"""The reference glue of the evaluator protocol, for atoms written in the
pure subset of Python that csdfbmc.checks.check_pure enforces.

evaluator(args) returns the handler of the protocol's requests, which
csdfbmc loads in-process with --evaluator-module; run as a command
(python3 -m csdfbmc.python_glue), it speaks the same requests as JSON lines
on its standard input and output.

An atom is a Python expression whose holes are names of their own. A guard
atom is any expression; a post atom is $x' == expression, $x' in
expression, or a tuple of every value after the step on the left, where
expression reads none of them. An atom that is not Python of the pure
subset is an error: a clause not formalised is a function of the
environment, which the checks module declares (csdfbmc.environment).
"""
from __future__ import annotations

import ast
import importlib.util
import re
from dataclasses import dataclass
from typing import Callable

from csdfbmc.formal import FORMAL_GLOBALS, free_names, tells_apart
from csdfbmc.protocol import atom_handler, atom_text
from csdfbmc.checks import check_pure
from csdfbmc.values import freeze

LANGUAGE = "python"


class Glue:
    """Atoms in the pure subset of Python. defs are the paths of modules
    whose public names every atom may use: constants and helpers shared by
    the atoms, checked against the pure subset like a checks module."""

    def __init__(self, defs: list[str]):
        self.globals = dict(FORMAL_GLOBALS)
        for path in defs:
            self.globals.update(_load_defs(path))
        self._compiled: dict[str, object] = {}

    def _eval(self, source: str, binding: dict):
        if source not in self._compiled:
            check_pure(source, f"<atom {source!r}>")
            self._compiled[source] = compile(ast.parse(source, mode="eval"), f"<atom {source!r}>", "eval")
        # The holes are globals, not locals, so that a comprehension or a
        # generator expression, a scope of its own, reads them too.
        return eval(self._compiled[source], {**self.globals, **binding})

    def atom_guard(self, atom: dict, values: list) -> bool:
        source, names = _atom_source(atom)
        if len(values) != len(names):
            raise ValueError(f"the atom has {len(names)} arguments, but was given {len(values)} values")
        return bool(self._eval(source, dict(zip(names, values))))

    def atom_symmetric(self, atom: dict, params: list[int], maps: list[int], groups: list[list[str]]) -> str | None:
        """How the atom tells two IDs of a group apart, its arguments params
        ranging over them and maps keyed by them, or None."""
        source, names = _atom_source(atom)
        found = tells_apart(ast.parse(source, mode="eval"), {names[i] for i in params}, {names[i] for i in maps},
                            [frozenset(g) for g in groups])
        if found is None:
            return None
        return re.sub(rf"{_HOLE}(\d+)", r"argument \1", found[1]) + ", so the IDs are not interchangeable"

    def atom_post(self, atom: dict, values: list) -> list[list]:
        """The values after the step, of the primed arguments in order, that
        the atom allows from values, those of the others in order."""
        source, names = _atom_source(atom)
        primed = [n for n, p in zip(names, atom["primed"]) if p]
        before = [n for n, p in zip(names, atom["primed"]) if not p]
        if len(values) != len(before):
            raise ValueError(f"the atom has {len(before)} arguments before the step, but was given {len(values)} values")
        targets, chosen = _post_shape(source, primed, before)
        options = self._eval(chosen, dict(zip(before, values)))
        if targets.is_tuple:
            return [[freeze(v) for v in _ordered(targets.names, primed, o)] for o in options]
        return [[freeze(o)] for o in options]


# The name an atom's argument i is read as; an atom's own text may not use it.
_HOLE = "_csdf_hole_"


def _atom_source(atom: dict) -> tuple[str, list[str]]:
    """The Python an atom is, its holes read as names, and those names by
    argument index."""
    text, holes, primed = atom["text"], atom["holes"], atom["primed"]
    if len(text) != len(holes) + 1:
        raise ValueError(f"an atom has one piece of text more than it has holes, not {len(text)} pieces for {len(holes)} holes")
    for k, h in enumerate(holes):
        if not isinstance(h, int) or isinstance(h, bool) or not 0 <= h < len(primed):
            raise ValueError(f"hole {k} reads argument {h!r}, but the atom has {len(primed)} arguments")
    if any(_HOLE in t for t in text):
        raise ValueError(f"an atom of this glue may not use the names {_HOLE}N, which stand for its holes")
    names = [f"{_HOLE}{i}" for i in range(len(primed))]
    return atom_text(atom, lambda i: names[i]), names


_POST_SHAPE = ("a post atom of this glue is $x' == expression, $x' in expression, or a tuple of every "
               "value after the step, ($x', $y') == expression or in one, where expression reads none of them")


@dataclass(frozen=True)
class _Targets:
    names: tuple[str, ...]
    is_tuple: bool


def _post_shape(source: str, primed: list[str], before: list[str]) -> tuple[_Targets, str]:
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
    for n in targets.names:
        if n in before:
            raise ValueError(f"argument {n[len(_HOLE):]} is a value before the step; the left of a post atom holds only "
                             f"the values after it")
    if sorted(targets.names) != sorted(primed) or free_names(right) & set(primed):
        raise ValueError(_POST_SHAPE)
    right_source = ast.unparse(right)
    return targets, (f"[{right_source}]" if isinstance(body.ops[0], ast.Eq) else right_source)


def _ordered(names: tuple[str, ...], primed: list[str], option) -> list:
    """option, a tuple of values for names, reordered as primed."""
    if len(option) != len(names):
        raise ValueError(f"the post atom chose {option!r}, {len(option)} values for {len(names)} values after the step")
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
    return atom_handler(LANGUAGE, guard=glue.atom_guard, post=glue.atom_post, symmetric=glue.atom_symmetric)


if __name__ == "__main__":
    import sys

    from csdfbmc.protocol import serve
    serve(evaluator(sys.argv[1:]), sys.stdin, sys.stdout)
