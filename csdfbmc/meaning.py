"""What the atoms of a diagram mean to the walk: a function of the
environment for an atom the checks module declares (csdfbmc.environment),
and the glue's answer for every other atom (csdfbmc.protocol). The checker
knows no clause language by name.

A glue receives the values of an atom's arguments by index, each instance
of a family as the map of its fields (csdfbmc.values.visible).
"""
from __future__ import annotations

import itertools
from collections.abc import Mapping
from typing import Callable, Iterable

from csdfbmc.environment import Environment
from csdfbmc.model import Atom
from csdfbmc.protocol import ClientLike
from csdfbmc.values import freeze, holds_fresh, same_value, visible


def apply_given(given: Mapping[str, object]) -> Callable[[str, tuple], object]:
    """The apply of an Evaluator: the environment function of a term (App)
    named fn, as the checks module's ENVIRONMENT gives it, applied to the
    values of its arguments as a glue sees them."""
    def apply(fn: str, args: tuple):
        if any(holds_fresh(a) for a in args):
            raise ValueError(f"the environment function {fn} was given a value beyond a sample, which it may tell apart from the others")
        g = given.get(fn)
        if not callable(g):
            raise ValueError(f"no environment function {fn}, which a term applies, is given in ENVIRONMENT")
        return freeze(g(*(visible(a) for a in args)))
    return apply


class AtomMeaning:
    """The meaning of the atoms: environment decides the declared ones, the
    glue behind client (as csdfbmc.protocol.checked gives it) the others.
    domain gives the values a name ranges over, for the values after the
    step of a post atom the environment decides. symmetric gives, per
    atom, the indices of the arguments the glue said it tells no two values
    of a sample apart by (csdfbmc.independence), the only arguments that
    may hold a value beyond a sample (csdfbmc.values.Fresh)."""

    def __init__(self, client: ClientLike, environment: Environment, domain: Callable[[str], list],
                 symmetric: Mapping[Atom, frozenset[int]] | None = None):
        self.client = client
        self.environment = environment
        self.domain = domain
        self.symmetric = dict(symmetric or {})
        self.decided: dict[str, None] = {}

    def _beyond(self, atom: Atom, indices: Iterable[int], args: list) -> None:
        """Refuses to show atom a value beyond a sample in an argument (of
        the indices, in order) not known to tell no two values of the sample
        apart."""
        asked = frozenset() if self.of_environment(atom) else self.symmetric.get(atom, frozenset())
        for i, a in zip(indices, args):
            if i not in asked and holds_fresh(a):
                raise ValueError(f"the atom {atom.function} would read a value beyond a sample in argument {i}, which "
                                 "is not known to tell no two values of the sample apart")

    def of_environment(self, atom: Atom) -> bool:
        return atom.function in self.environment.declared

    def _decide(self, atom: Atom, args: tuple) -> bool:
        value = self.environment.decide(atom.function, args)
        if value is None:
            raise ValueError(f"the environment decides nothing for {atom.function!r}; give it in ENVIRONMENT or with --env, or choose it with --seed N")
        self.decided.setdefault(atom.function)
        return value

    def guard(self, atom: Atom, args: list) -> bool:
        """Whether an atom that reads no value after the step holds of the
        values of its arguments."""
        self._beyond(atom, range(len(args)), args)
        if self.of_environment(atom):
            return self._decide(atom, tuple(visible(a) for a in args))
        return self.client.guard(atom.wire, [[visible(a) for a in args]])[0]

    def post(self, atom: Atom, before: list, names: list[str]) -> list[tuple]:
        """The values after the step of the primed arguments, in order, that
        an atom allows from the values before it of the others. names name
        the primed arguments' types, over which the environment chooses."""
        self._beyond(atom, (i for i, p in enumerate(atom.primed) if not p), before)
        if not self.of_environment(atom):
            return [tuple(freeze(v) for v in after) for after in self.client.post(atom.wire, [[visible(a) for a in before]])[0]]
        out = []
        for after in itertools.product(*(self.domain(n) for n in names)):
            if self._decide(atom, tuple(visible(a) for a in self.merge(atom, before, list(after)))):
                out.append(tuple(after))
        return out

    def post_holds(self, atom: Atom, args: list) -> bool:
        """Whether an atom that reads values after the step holds of the
        values of all its arguments."""
        self._beyond(atom, range(len(args)), args)
        if self.of_environment(atom):
            return self._decide(atom, tuple(visible(a) for a in args))
        before = [a for a, p in zip(args, atom.primed) if not p]
        after = tuple(visible(a) for a, p in zip(args, atom.primed) if p)
        return any(same_value(after, tuple(visible(v) for v in t)) for t in self.post(atom, before, []))

    @staticmethod
    def merge(atom: Atom, before: list, after: list) -> list:
        """The values of all the arguments of atom, from those before the
        step and those after it, each in order."""
        b, a = iter(before), iter(after)
        return [next(a) if p else next(b) for p in atom.primed]
