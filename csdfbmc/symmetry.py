"""Symmetry reduction: a state is stored by one representative of its
renamings of the values of the types the checks module declares SYMMETRIC
(docs/CLAUSE_DESIGN.md, Q2 and O10).

The values of each symmetric type are a group of IDs, strings, that the
diagram never tells apart: renaming them in a reachable state gives a
reachable state with the same findings. The walk stores a state by the
least of its renamings, and still expands the state it first met, not a
renamed copy, so every trace it reports is a run of the diagram.

The declaration is refused when anything could tell two IDs apart:

- a type whose values are not closed under renaming the IDs (a type that
  holds some of them only);
- an atom the glue says tells them apart: every atom is asked, its
  arguments that may hold an ID and those that are maps keyed by IDs named
  (the evaluator protocol's symmetric request); a glue that does not answer
  it is refused;
- a function of the environment one of whose arguments may hold an ID,
  unless it is given a constant or as Python in the checks module, which is
  read as the ghosts and invariants are;
- the checks module's source, the ghosts, the invariants and the functions
  it gives, naming an ID as a constant (a literal listing every ID of its
  group, as a type does, names none), ordering an ID, or sorting, taking
  the min or max of an ID or of a map keyed by them (csdfbmc.formal).

The builtins and ∃ are the checker's and never tell IDs apart: @eq and @ne
compare, @in, @notin, @at, @frame and @update look up, and ∃ ranges over a
whole type. What may hold an ID is read from the types and the builtins: a
name whose type holds one, a field of a record that does, a map an ID
indexes, and what a builtin of a post gives from any of them, followed to a
fixed point. What an atom gives is the glue's, which the tools do not read,
so every state the walk meets is checked instead: a variable, a field of an
instance or a ghost that holds an ID where none was foreseen is refused,
since the atoms reading it were not asked with it. The check of the sources
is syntactic, and does not follow a value through a helper.
"""
from __future__ import annotations

import ast
import itertools
from collections.abc import Mapping

from csdfbmc import formal
from csdfbmc.checks import Checks
from csdfbmc.environment import Environment
from csdfbmc.model import App, Atom, Bound, Builtin, Diagram, Exists, Field, Free, Index, root
from csdfbmc.protocol import ClientLike
from csdfbmc.values import FrozenMap, Rec


class Symmetry:
    def __init__(self, d: Diagram, checks: Checks, client: ClientLike, environment: Environment):
        if not checks.symmetric:
            raise ValueError("--symmetry needs the checks module to declare SYMMETRIC, the types whose values may be renamed")
        groups: list[frozenset] = []
        for t in checks.symmetric:
            if t not in checks.types:
                raise ValueError(f"SYMMETRIC names {t!r}, which TYPES does not declare")
            values = frozenset(checks.types[t])
            if not all(isinstance(v, str) for v in values):
                raise ValueError(f"SYMMETRIC names {t!r}, whose values are not all strings; an ID that is a number cannot be told from a count")
            for g in groups:
                if g & values and g != values:
                    raise ValueError(f"the values of {t!r} overlap those of another symmetric type without being the same")
            if values not in groups:
                groups.append(values)
        self.groups = groups
        self.values = frozenset().union(*groups)
        self.perms = [dict(itertools.chain.from_iterable(zip(sorted(g), perm) for g, perm in zip(groups, combo)))
                      for combo in itertools.product(*(itertools.permutations(sorted(g)) for g in groups))]
        self.d = d
        self._check_types(checks)
        self.ids, self.fields, self.maps = self._holding(d, checks)
        self._check_atoms(d, client, environment)
        self._check_sources(checks)

    # -- what may hold an ID -------------------------------------------------------

    def _holds(self, v) -> bool:
        if isinstance(v, str):
            return v in self.values
        if isinstance(v, Rec):
            return self._holds(v.fields)
        if isinstance(v, Mapping):
            return any(self._holds(k) or self._holds(x) for k, x in v.items())
        if isinstance(v, (tuple, frozenset)):
            return any(self._holds(x) for x in v)
        return False

    def _holding(self, d: Diagram, checks: Checks) -> tuple[set[str], set[str], set[str]]:
        """The names that may hold an ID, the fields that may, and the maps
        an ID indexes."""
        ids: set[str] = set()
        fields: set[str] = set()
        for name, values in checks.typed_names().items():
            for v in values:
                if self._holds(v):
                    ids.add(name)
                if isinstance(v, Mapping):
                    fields |= {f for f, x in v.items() if isinstance(f, str) and self._holds(x)}
        maps: set[str] = set()
        clauses = [d.start_post] + [c for e in d.edges for c in (e.guard, e.post)] + [x.guard for x in d.ends]
        grown = True
        while grown:
            before = (len(ids), len(fields), len(maps))
            for clause in clauses:
                for c, types in _conjuncts(clause, ()):
                    for t in _terms(c):
                        if isinstance(t, Index) and isinstance(t.map, Free) and self._carries(t.key, types, ids, fields):
                            maps.add(t.map.name)
                    ids |= maps
                    if isinstance(c, Builtin) and any(self._carries(a, types, ids, fields) for a in c.args if not _primed(a)):
                        for a in c.args:
                            if _primed(a):
                                ids.add(root(a).name)
                                if isinstance(a, Field):
                                    fields.add(a.name)
            grown = (len(ids), len(fields), len(maps)) != before
        return ids, fields, maps

    def _carries(self, t, types: tuple, ids: set[str], fields: set[str]) -> bool:
        """Whether the term t may hold an ID; types are those of the
        enclosing ∃s, innermost last."""
        if isinstance(t, Free):
            return t.name in ids
        if isinstance(t, Bound):
            return types[len(types) - 1 - t.index] in ids
        if isinstance(t, Index):
            return self._carries(t.map, types, ids, fields)
        if isinstance(t, Field):
            return t.name in fields
        if isinstance(t, App):
            return any(self._carries(a, types, ids, fields) for a in t.args)
        return False

    # -- the checks --------------------------------------------------------------

    def _check_types(self, checks: Checks) -> None:
        for t, values in checks.types.items():
            held = set(values)
            for perm in self.perms:
                if {rename(v, perm) for v in held} != held:
                    raise ValueError(f"the values of {t!r} are not closed under renaming the symmetric IDs, so they are not interchangeable")

    def _check_atoms(self, d: Diagram, client: ClientLike, environment: Environment) -> None:
        asked: set = set()
        groups = [sorted(g) for g in self.groups]
        for clause in [d.start_post] + [c for e in d.edges for c in (e.guard, e.post)] + [x.guard for x in d.ends]:
            for c, types in _conjuncts(clause, ()):
                for t in _terms(c):
                    if isinstance(t, App) and any(self._carries(a, types, self.ids, self.fields) for a in t.args):
                        self._check_environment(t.fn, environment)
                if not isinstance(c, Atom):
                    continue
                maps = [i for i, a in enumerate(c.args) if isinstance(a, Free) and a.name in self.maps]
                params = [i for i, a in enumerate(c.args) if i not in maps and self._carries(a, types, self.ids, self.fields)]
                if c.function in environment.declared:
                    if params or maps:
                        self._check_environment(c.function, environment)
                    continue
                key = (c.function, tuple(c.primed), tuple(params), tuple(maps))
                if key in asked:
                    continue
                asked.add(key)
                why = client.symmetric(c.wire, params, maps, groups)
                if why is not None:
                    raise ValueError(f"the atom {c.function!r} tells the symmetric IDs apart: {why}; drop its type from SYMMETRIC or change the atom")

    def _check_environment(self, name: str, environment: Environment) -> None:
        given = environment.given.get(name)
        if not isinstance(given, bool) and not callable(given):
            raise ValueError(f"the function of the environment {name!r} reads a symmetric ID, and a seed would tell the IDs apart; "
                             "give it in ENVIRONMENT, or drop its type from SYMMETRIC")

    def _check_sources(self, checks: Checks) -> None:
        if not checks.sources:
            raise ValueError("--symmetry reads the checks module to check SYMMETRIC, and none was given as source")
        for path, source in checks.sources:
            found = formal.tells_apart(ast.parse(source, filename=path), self.ids - self.maps, self.maps, self.groups)
            if found is not None:
                line, how = found
                raise ValueError(f"{path}:{line}: {how}, so the IDs are not interchangeable; drop its type from SYMMETRIC or change the checks module")

    # -- the walk ----------------------------------------------------------------

    def check(self, state) -> None:
        """Refuses a state holding an ID where nothing foresaw one: the atoms
        reading it were asked whether they tell IDs apart without it."""
        sid, values, ghosts = state
        for name, v in zip(self.d.vars[sid], values):
            if name not in self.ids and self._holds(v):
                self._unforeseen(f"{name} holds {v!r}", state)
            if name in self.maps and isinstance(v, Mapping):
                for x in v.values():
                    fields = x.fields if isinstance(x, Rec) else {}
                    for f, fv in fields.items():
                        if f not in self.fields and self._holds(fv):
                            self._unforeseen(f"the field {f} of an instance in {name} holds {fv!r}", state)
        for g in ghosts:
            if self._holds(g):
                self._unforeseen(f"a ghost holds {g!r}", state)

    def _unforeseen(self, what: str, state) -> None:
        raise ValueError(f"at {state[0]}, {what}, an ID of a symmetric type that no type or builtin puts there, so the atoms reading it "
                         "were not asked whether they tell the IDs apart; give it through a builtin or a typed name, or drop the type from SYMMETRIC")

    def canonical(self, state):
        """The representative of state's renamings: the least under a fixed
        order of values. A state holding an ID where nothing foresaw one is
        refused (check)."""
        self.check(state)
        sid, values, ghosts = state
        return min(((sid, rename(values, p), rename(ghosts, p)) for p in self.perms), key=lambda s: _order_key(s[1:]))


def _primed(t) -> bool:
    r = root(t)
    return isinstance(r, Free) and r.primed


def _conjuncts(clause, types: tuple):
    """The conjuncts of clause but ∃, each with the types of the ∃s
    enclosing it."""
    for c in clause:
        if isinstance(c, Exists):
            yield from _conjuncts(c.body, types + (c.type,))
        else:
            yield c, types


def _terms(c):
    """Every term c holds, at any depth."""
    stack = list(getattr(c, "args", ()))
    while stack:
        t = stack.pop()
        yield t
        if isinstance(t, Index):
            stack += [t.map, t.key]
        elif isinstance(t, Field):
            stack.append(t.of)
        elif isinstance(t, App):
            stack += list(t.args)


def rename(v, perm: dict):
    if isinstance(v, Rec):
        return Rec(v.state, rename(v.fields, perm))
    if isinstance(v, FrozenMap):
        return FrozenMap({rename(k, perm): rename(x, perm) for k, x in v.items()})
    if isinstance(v, tuple):
        return tuple(rename(x, perm) for x in v)
    if isinstance(v, frozenset):
        return frozenset(rename(x, perm) for x in v)
    if isinstance(v, str):
        return perm.get(v, v)
    return v


def _order_key(v):
    if isinstance(v, Rec):
        return (2, v.state, _order_key(v.fields))
    if isinstance(v, Mapping):
        return (3, tuple(sorted((_order_key(k), _order_key(x)) for k, x in v.items())))
    if isinstance(v, tuple):
        return (4, tuple(_order_key(x) for x in v))
    if isinstance(v, frozenset):
        return (5, tuple(sorted(_order_key(x) for x in v)))
    return (0, type(v).__name__, repr(v))
