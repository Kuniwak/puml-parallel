"""What the atoms of a diagram mean to the walk: a function of the
environment for an atom the checks module declares (csdfbmc.environment),
and the glue's answer for every other atom (csdfbmc.protocol). The checker
knows no clause language by name.

A glue receives the values of an atom's arguments by index, each instance
of a family as the map of the fields its family observes
(csdfbmc.values.visible).
"""
from __future__ import annotations

import itertools
from collections.abc import Mapping
from typing import Callable

from csdfbmc.environment import Environment
from csdfbmc.model import Atom
from csdfbmc.protocol import ClientLike
from csdfbmc.values import forget_views, freeze, kind_key, same_value, visible


class Answers:
    """The glue's answers about the atoms, by atom and the values of its
    arguments as the walk holds them (kind_key): guards whether it holds,
    posts the values after the step; and about the environment functions,
    by function and the values of its arguments (applied)."""

    def __init__(self):
        self.guards: dict[tuple, bool] = {}
        self.posts: dict[tuple, list] = {}
        self.applied: dict[tuple, object] = {}


def apply_given(given: Mapping[str, object], seen: Mapping, answers: Answers) -> Callable[[str, tuple], object]:
    """The apply of an Evaluator: the environment function of a term (App)
    named fn, as the checks module's ENVIRONMENT gives it, applied to the
    values of its arguments as a glue sees them, seen giving the fields
    observed at each local state (csdfbmc.model.observed_by_state).

    A function is applied once per list of values, which a pure function
    allows, keyed as AtomMeaning keys an atom's; answers keeps the values
    it gives."""
    applied = answers.applied

    def apply(fn: str, args: tuple):
        key = (fn, kind_key(args))
        if key not in applied:
            g = given.get(fn)
            if not callable(g):
                raise ValueError(f"no environment function {fn}, which a term applies, is given in ENVIRONMENT")
            applied[key] = forget_views(freeze(g(*(visible(a, seen) for a in args))))
        return applied[key]
    return apply


class AtomMeaning:
    """The meaning of the atoms: environment decides the declared ones, the
    glue behind client (as csdfbmc.protocol.checked gives it) the others.
    carrier gives the values of a type, for the values after the
    step of a post atom the environment decides, and seen the fields
    observed at each local state (csdfbmc.model.observed_by_state).

    The glue is asked about an atom once per list of values, which a pure
    atom allows. The values are keyed as the walk holds them, before they
    are made visible: what the glue sees of them is a function of them and
    seen, and their kind_key tells apart an instance's states and fields.
    answers keeps them, and may be shared by the meanings of one seen."""

    def __init__(self, client: ClientLike, environment: Environment, carrier: Callable[[str], list], seen: Mapping,
                 answers: Answers | None = None):
        self.client = client
        self.environment = environment
        self.carrier = carrier
        self.seen = seen
        self.decided: dict[str, None] = {}
        self.answers = Answers() if answers is None else answers

    def _visible(self, value):
        return visible(value, self.seen)

    def _asked(self, answers: dict, ask: Callable, atom: Atom, args: list):
        key = (atom, kind_key(tuple(args)))
        if key not in answers:
            answers[key] = ask(atom.wire, [[self._visible(a) for a in args]])[0]
        return answers[key]

    def of_environment(self, atom: Atom) -> bool:
        return atom.function in self.environment.declared

    def _decide(self, atom: Atom, args: tuple) -> bool:
        value = self.environment.decide(atom.function, args)
        if value is None:
            raise ValueError(f"the environment decides nothing for {atom.function!r}; give it in ENVIRONMENT or with --env")
        self.decided.setdefault(atom.function)
        return value

    def guard(self, atom: Atom, args: list) -> bool:
        """Whether an atom that reads no value after the step holds of the
        values of its arguments."""
        if self.of_environment(atom):
            return self._decide(atom, tuple(self._visible(a) for a in args))
        return self._asked(self.answers.guards, self.client.guard, atom, args)

    def post(self, atom: Atom, before: list, types: list[str]) -> list[tuple]:
        """The values after the step of the primed arguments, in order, that
        an atom allows from the values before it of the others. names name
        the primed arguments' types, over which the environment chooses."""
        if not self.of_environment(atom):
            return [tuple(forget_views(freeze(v)) for v in after) for after in self._asked(self.answers.posts, self.client.post, atom, before)]
        out = []
        for after in itertools.product(*(self.carrier(t) for t in types)):
            if self._decide(atom, tuple(self._visible(a) for a in self.merge(atom, before, list(after)))):
                out.append(tuple(after))
        return out

    def post_holds(self, atom: Atom, args: list) -> bool:
        """Whether an atom that reads values after the step holds of the
        values of all its arguments."""
        if self.of_environment(atom):
            return self._decide(atom, tuple(self._visible(a) for a in args))
        before = [a for a, p in zip(args, atom.primed) if not p]
        after = tuple(self._visible(a) for a, p in zip(args, atom.primed) if p)
        return any(same_value(after, tuple(self._visible(v) for v in t)) for t in self.post(atom, before, []))

    @staticmethod
    def merge(atom: Atom, before: list, after: list) -> list:
        """The values of all the arguments of atom, from those before the
        step and those after it, each in order."""
        b, a = iter(before), iter(after)
        return [next(a) if p else next(b) for p in atom.primed]
