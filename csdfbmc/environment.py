"""The environment: what a diagram takes from outside (docs/CLAUSE_DESIGN.md,
P15 and P16). Besides the types of the checks module, it holds the pure
functions that a clause not formalised denotes - an atom the glue says it
cannot read: a truth value for each valuation of the names the clause may
read, fixed for the whole run. The tool word @opaque is no longer read.

A function is named by its atom: the atom's text without its holes, its
arguments the values of the holes, in order (csdfbmc.atoms); atoms with the
same text are the same function. A name given may be written as any atom of
the function, `$在庫 が 十分` as well as `$_ が 十分`.

A function is given by the checks module (ENVIRONMENT) or the command line
(--env NAME=true|false), or chosen from a seed (--seed N): a seed picks one
function deterministically, by hashing the seed, the function's name and the
values of its arguments, and nothing else, so that the same seed walks the
same states again and different seeds widen what bounded checking covers. A
given function wins over the seed; a function given as Python takes the
arguments in order. The report names the seed and the given functions,
which is all a run needs to be reproduced.

What decides is a function (Decide) built from combinators - nothing,
seeded, given_over - and an Environment is that function together with what
reproduces it.
"""
from __future__ import annotations

import hashlib
from collections.abc import Mapping
from dataclasses import dataclass
from typing import Callable

from csdfbmc.atoms import function_of

Given = bool | Callable[..., bool]

# Decide is the value of the function named name for the values of its
# arguments, in order, or None when it decides nothing for that name.
Decide = Callable[[str, tuple], "bool | None"]


def nothing(name: str, args: tuple) -> bool | None:
    """Decides no function."""
    return None


def seeded(seed: int) -> Decide:
    """Every function, chosen from seed by hashing it with the function's
    name and its arguments."""
    def decide(name: str, args: tuple) -> bool | None:
        key = "\0".join((str(seed), name, canonical(tuple(args))))
        return hashlib.sha256(key.encode("utf-8")).digest()[0] & 1 == 1
    return decide


def named(given: Mapping[str, Given]) -> dict[str, Given]:
    """given keyed by the name of each function, whatever atom of it a key
    is written as."""
    return {function_of(n).name: g for n, g in given.items()}


def given_over(given: Mapping[str, Given], inner: Decide) -> Decide:
    """The functions given, a truth value or a pure function of the
    arguments in order, each under any atom of its function, and inner for
    the rest."""
    return _given_over_named(named(given), inner)


def _given_over_named(given: Mapping[str, Given], inner: Decide) -> Decide:
    def decide(name: str, args: tuple) -> bool | None:
        if name not in given:
            return inner(name, args)
        g = given[name]
        if isinstance(g, bool):
            return g
        value = g(*args)
        if not isinstance(value, bool):
            raise ValueError(f"the environment's {name!r} gave {value!r}; it gives True or False")
        return value
    return decide


def recording(decide: Decide, seen: dict[str, None]) -> Decide:
    """decide, noting in seen, in order, each name it decided."""
    def recorded(name: str, args: tuple) -> bool | None:
        value = decide(name, args)
        if value is not None:
            seen.setdefault(name)
        return value
    return recorded


@dataclass(frozen=True)
class Environment:
    """What decides the functions, and what reproduces it: the seed and the
    functions given, by name."""
    decide: Decide
    seed: int | None
    given: Mapping[str, Given]

    def to_json(self) -> dict:
        return {"seed": self.seed, "given": sorted(self.given)}


# NONE decides nothing: no function is given, and there is no seed.
NONE = Environment(decide=nothing, seed=None, given={})


def over(module: Mapping[str, Given], given: Mapping[str, Given], seed: int | None) -> Environment:
    """The environment of a run: given over the checks module's ENVIRONMENT
    (module), and the seed, when there is one, for the rest."""
    merged = named(module) | named(given)
    return Environment(decide=_given_over_named(merged, seeded(seed) if seed is not None else nothing),
                       seed=seed, given=merged)


def canonical(v) -> str:
    """v written so that equal values are written alike, whatever order a
    set or a mapping iterates in: a mapping as its sorted entries, a set as
    its sorted elements, a tuple or a list alike as the sequence of its
    elements, and anything else as its type's name and its repr. The seed's
    choices depend on this text, so it must not change between versions;
    tests pin it."""
    if isinstance(v, Mapping):
        return "{" + ",".join(sorted(f"{canonical(k)}:{canonical(x)}" for k, x in v.items())) + "}"
    if isinstance(v, (set, frozenset)):
        return "#{" + ",".join(sorted(canonical(x) for x in v)) + "}"
    if isinstance(v, (tuple, list)):
        return "(" + ",".join(canonical(x) for x in v) + ")"
    return f"{type(v).__name__}:{v!r}"
