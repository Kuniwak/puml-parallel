"""The environment: what a diagram takes from outside (docs/CLAUSE_DESIGN.md,
P15 and P16). Besides the types of the checks module, it holds the pure
functions that atoms not formalised denote: a truth value for each
valuation of the atom's arguments, fixed for the whole run.

Which atoms are functions of the environment is declared, by the checks
module alone: every key of its ENVIRONMENT declares one, and every other
atom is read through the glue, which answers an atom it cannot read with
an error. A key gives the function True, False or a pure function of the
arguments in order, or None, which declares it and leaves it to
--env NAME=true|false. --env may name only a declared function; a declared
function nothing gives is refused before the walk, listing every one, and
so is a declared name that names no atom of the diagram. A function is
given, never chosen: nothing approximates an undecided one
(docs/CLAUSE_DESIGN.md, P16). A run that wants a pseudo-random function
writes it in ENVIRONMENT, where it stays visible and reviewable.

A function is named as the diagram's JSON names it (an atom's "function",
clause.FunctionOf): the atom's text with each hole written $_N, N the
position of the argument it reads, so `$在庫 が 十分` is the function
`$_0 が 十分`; atoms with the same text are the same function. A checks
module and --env name a function so, and csdfbmc never reads an atom's
text itself.

The report names the given functions, which with the checks module and
the command line is all a run needs to be reproduced.

What decides is a function (Decide) built from combinators - nothing,
given_over - and an Environment is that function together with
what it declares and what reproduces it.
"""
from __future__ import annotations

from collections.abc import Mapping
from dataclasses import dataclass
from typing import Callable


Given = bool | Callable[..., bool]
# A key of ENVIRONMENT whose value is None declares the function and gives
# nothing: --env gives it.

# Decide is the value of the function named name for the values of its
# arguments, in order, or None when it decides nothing for that name.
Decide = Callable[[str, tuple], "bool | None"]


def nothing(name: str, args: tuple) -> bool | None:
    """Decides no function."""
    return None


def given_over(given: Mapping[str, Given], inner: Decide) -> Decide:
    """The functions given by name, a truth value or a pure function of the
    arguments in order, and inner for the rest."""
    return _given_over_named(given, inner)


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
    """What decides the functions, and what reproduces it: the functions
    declared, by name, and the functions given, by name."""
    decide: Decide
    declared: frozenset[str]
    given: Mapping[str, Given]

    def to_json(self) -> dict:
        return {"given": sorted(self.given)}


# NONE declares nothing: every atom is the glue's.
NONE = Environment(decide=nothing, declared=frozenset(), given={})


def over(module: Mapping[str, "Given | None"], given: Mapping[str, Given]) -> Environment:
    """The environment of a run: the functions the checks module's
    ENVIRONMENT (module) declares, given by it or by given (--env) where
    either gives one, and undecided for the rest. given may name only a
    declared function."""
    declared = dict(module)
    cli = dict(given)
    undeclared = [n for n in given if n not in declared]
    if undeclared:
        raise ValueError(f"--env names {', '.join(map(repr, undeclared))}, which the checks module's ENVIRONMENT does not declare; "
                         "declare it there (None leaves it to --env)")
    merged = {n: g for n, g in declared.items() if g is not None} | cli
    return Environment(decide=_given_over_named(merged, nothing),
                       declared=frozenset(declared), given=merged)
