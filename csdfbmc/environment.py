"""The environment: what a diagram takes from outside (docs/CLAUSE_DESIGN.md,
P15 and P16). Besides the types of the checks module, it holds the pure
functions that a clause not formalised denotes - the tool word @opaque, or a
clause the glue says it cannot read: a truth value for each valuation of the
names the clause may read, fixed for the whole run.

A function is given by the checks module (ENVIRONMENT) or the command line
(--env NAME=true|false), or chosen from a seed (--seed N): a seed picks one
function deterministically, by hashing the seed, the function's name and its
arguments, so that the same seed walks the same states again and different
seeds widen what bounded checking covers. A given function wins over the
seed. The report names the seed and the given functions, which is all a run
needs to be reproduced.
"""
from __future__ import annotations

import hashlib
from collections.abc import Mapping
from typing import Callable

from csdfbmc.checks import apply

Given = bool | Callable[..., bool]


class Environment:
    def __init__(self, *, given: dict[str, Given] | None = None, seed: int | None = None):
        self.given = dict(given or {})
        self.seed = seed

    def decide(self, name: str, args: dict) -> bool | None:
        """The value of the function name for args; None when nothing gives
        or seeds it."""
        if name in self.given:
            g = self.given[name]
            if isinstance(g, bool):
                return g
            value = apply(g, args, what=f"the environment's {name!r}")
            if not isinstance(value, bool):
                raise TypeError(f"the environment's {name!r} gave {value!r}; it gives True or False")
            return value
        if self.seed is None:
            return None
        key = "\0".join((str(self.seed), name, canonical(dict(args))))
        return hashlib.sha256(key.encode("utf-8")).digest()[0] & 1 == 1

    def to_json(self) -> dict:
        return {"seed": self.seed, "given": sorted(self.given)}


def canonical(v) -> str:
    """v written so that equal values are written alike, whatever order a
    set or a mapping iterates in."""
    if isinstance(v, Mapping):
        return "{" + ",".join(sorted(f"{canonical(k)}:{canonical(x)}" for k, x in v.items())) + "}"
    if isinstance(v, (set, frozenset)):
        return "#{" + ",".join(sorted(canonical(x) for x in v)) + "}"
    if isinstance(v, (tuple, list)):
        return "(" + ",".join(canonical(x) for x in v) + ")"
    return f"{type(v).__name__}:{v!r}"
