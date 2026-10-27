"""What a clause evaluates to: a guard to a truth value, a post to the
valuations it allows after the step (docs/CLAUSE_DESIGN.md, O7).

A term reads the values before the step by name, those after it when
primed, and a bound index from the values the enclosing ∃s chose. A clause
that reads an entry its map does not have is false: a clause about an
instance holds only while the instance exists. A value is never defaulted.

A post is solved, not searched: the values after the step come from what
determines them - an equality builtin with a value after the step on one
side (Q5), @empty, @update, @frame with @at and @notin for its keys, and
the values a post atom allows - and only a variable nothing in the post
constrains ranges over its type in TYPES (refused without one). An
instance @at puts at a local state holds exactly that state's variables,
which the family of its map gives (Diagram.families): each the post leaves
open ranges over its type the same way, and a post that gives a field the
state does not hold is refused. So is a post that allows a map of a
family to hold what no instance is, however it wrote the value (@eq of the
map or of an entry, @update, or TYPES): the value is the post's, so it is
refused rather than taken for no successor. Every
valuation found is then checked against the whole post, so what
determines a value is an optimisation of the meaning, never a part of it.
"""
from __future__ import annotations

import itertools
from collections.abc import Mapping
from typing import Callable, Protocol

from csdfbmc.model import App, Atom, Bound, Builtin, Exists, Family, Field, Free, Index, Lit, is_primed
from csdfbmc.values import FrozenMap, FrozenSet, Rec, Strict, same_value


class Missing(Exception):
    """A term read an entry its map does not have."""


class Atoms(Protocol):
    def of_environment(self, atom: Atom) -> bool: ...
    def guard(self, atom: Atom, args: list) -> bool: ...
    def post(self, atom: Atom, before: list, names: list[str]) -> list[tuple]: ...
    def post_holds(self, atom: Atom, args: list) -> bool: ...


class Evaluator:
    """Evaluates clauses: atoms says what an atom means; carrier gives the
    values of a type, which an ∃ ranges over; apply gives the value of an
    environment function of a term (App) applied to values."""

    def __init__(self, atoms: Atoms, carrier: Callable[[str], list], apply: Callable[[str, tuple], object]):
        self.atoms = atoms
        self.carrier = carrier
        self.apply = apply

    # -- terms -------------------------------------------------------------------

    def value(self, t, before: Mapping, after: Mapping, bound: tuple):
        if isinstance(t, Free):
            values = after if t.primed else before
            if t.name not in values:
                raise ValueError(f"{t.name}{chr(39) if t.primed else ''} has no value here")
            return values[t.name]
        if isinstance(t, Bound):
            if t.index >= len(bound):
                raise ValueError(f"the bound index {t.index} is bound by no ∃")
            return bound[len(bound) - 1 - t.index]
        if isinstance(t, Index):
            m = self.value(t.map, before, after, bound)
            k = self.value(t.key, before, after, bound)
            if not isinstance(m, Mapping):
                raise ValueError(f"{m!r} is not a map, which an entry is read of")
            if k not in m:
                raise Missing()
            return m[k]
        if isinstance(t, Field):
            r = self.value(t.of, before, after, bound)
            fields = r.fields if isinstance(r, Rec) else r
            if not isinstance(fields, Mapping) or t.name not in fields:
                raise ValueError(f"{r!r} has no field {t.name}")
            return fields[t.name]
        if isinstance(t, App):
            return self.apply(t.fn, tuple(self.value(a, before, after, bound) for a in t.args))
        if isinstance(t, Lit):
            return t.value
        raise ValueError(f"unknown term {t!r}")

    # -- formulas ----------------------------------------------------------------

    def holds(self, clause: tuple, before: Mapping, after: Mapping = FrozenMap(), bound: tuple = ()) -> bool:
        return all(self._holds(c, before, after, bound) for c in clause)

    def _holds(self, f, before, after, bound) -> bool:
        if isinstance(f, Exists):
            return any(self.holds(f.body, before, after, bound + (v,)) for v in self.carrier(f.type))
        try:
            args = [self.value(a, before, after, bound) for a in f.args]
        except Missing:
            return False
        if isinstance(f, Atom):
            if any(f.primed):
                return self.atoms.post_holds(f, args)
            return self.atoms.guard(f, args)
        return builtin(f, args)


def builtin(b: Builtin, args: list) -> bool:
    op = b.op
    if op == "eq":
        return same_value(args[0], args[1])
    if op == "ne":
        return not same_value(args[0], args[1])
    if op in ("in", "notin"):
        _map(b, args[1])
        return (args[0] in args[1]) == (op == "in")
    if op == "empty":
        return len(_map(b, args[0])) == 0
    if op == "update":
        m1, m0 = _map(b, args[0]), _map(b, args[1])
        return same_value(m1, m0.set(args[2], args[3]) if isinstance(m0, FrozenMap) else FrozenMap(m0).set(args[2], args[3]))
    if op == "frame":
        m1, m0 = _map(b, args[0]), _map(b, args[1])
        keys = args[2:]
        for k in FrozenSet(m0) | FrozenSet(m1):
            if _among(k, keys):
                continue
            if (k in m0) != (k in m1) or not same_value(m0[k], m1[k]):
                return False
        return True
    if op == "at":
        if not isinstance(args[0], Rec):
            raise ValueError(f"@at of {args[0]!r}, which is not an instance")
        return args[0].state == b.state
    raise ValueError(f"unknown builtin @{op}")


def _map(b: Builtin, v) -> Mapping:
    if not isinstance(v, Mapping):
        raise ValueError(f"@{b.op} of {v!r}, which is not a map")
    return v


# -- posts --------------------------------------------------------------------


class _Absent:
    """What a post says an entry is when its key is not in its map."""

    def __repr__(self):
        return "absent"


ABSENT = _Absent()


class PostError(ValueError):
    """A post that leaves part of the state after the step unconstrained in a
    way the walk cannot enumerate."""


# A location is what a primed term names, a Strict, so that the entries of
# the keys 1 and True are two: ("var", x) for x'; ("entry", m, k)
# for the entry m'[k], an instance or ABSENT; ("state", m, k) for the state
# @at says the instance m'[k] is at; ("field", m, k, f) for m'[k].f.


def _loc(t, ev: Evaluator, before, bound):
    """The location a primed term names; None for another shape."""
    if isinstance(t, Free) and t.primed:
        return Strict(("var", t.name))
    if isinstance(t, Index) and isinstance(t.map, Free) and t.map.primed and not is_primed(t.key):
        return Strict(("entry", t.map.name, ev.value(t.key, before, {}, bound)))
    if isinstance(t, Field) and isinstance(t.of, Index) and isinstance(t.of.map, Free) and t.of.map.primed \
            and not is_primed(t.of.key):
        return Strict(("field", t.of.map.name, ev.value(t.of.key, before, {}, bound), t.name))
    return None


def _primed_terms(t):
    """The primed terms t is or holds, outermost first."""
    if is_primed(t):
        yield t
    elif isinstance(t, App):
        for a in t.args:
            yield from _primed_terms(a)


def _branches(clause: tuple, bound: tuple, carrier) -> list[list[tuple]]:
    """The conjuncts of clause with the values bound for each, one list per
    choice of the values its ∃s bind: (formula, bound) pairs, no ∃ among
    them."""
    out: list[list[tuple]] = [[]]
    for f in clause:
        if isinstance(f, Exists):
            grown = []
            for v in carrier(f.type):
                for inner in _branches(f.body, bound + (v,), carrier):
                    grown.extend(b + inner for b in out)
            out = grown
        else:
            out = [b + [(f, bound)] for b in out]
    return out


def _join(joined: list[dict], options: list[dict]) -> list[dict]:
    return [{**j, **o} for j in joined for o in options
            if all(same_value(j[k], v) for k, v in o.items() if k in j)]


def solve(ev: Evaluator, post: tuple, names: tuple[str, ...], before: Mapping,
          types: Mapping[str, str], where: str, families: Mapping[str, Family]) -> list[dict]:
    """The valuations of names, the variables of the state after the step,
    that post allows from before. types gives the type the diagram declares
    of each variable, whose values ev.carrier gives; where names the edge in
    a refusal; families gives, for each map, what an instance in it may be:
    at a state @at gives, it holds exactly that state's variables, of the
    types the family declares."""
    out: list[dict] = []
    for branch in _branches(post, (), ev.carrier):
        for after in _solve_branch(ev, branch, names, before, types, where, families):
            if all(ev._holds(f, before, after, bound) for f, bound in branch):
                _check_instances(after, families, where)
                if not any(all(same_value(after[n], o[n]) for n in names) for o in out):
                    out.append(after)
    return out


def _solve_branch(ev: Evaluator, branch: list[tuple], names, before, types, where, families) -> list[dict]:
    options: list[list[dict]] = []
    frames: dict[str, list[tuple]] = {}
    mentioned: set[tuple] = set()
    try:
        for f, bound in branch:
            for t in (u for a in getattr(f, "args", ()) for u in _primed_terms(a)):
                loc = _loc(t, ev, before, bound)
                if loc is None:
                    raise PostError(f"{where}: the post reads a value after the step as {t!r}, which is neither a variable, "
                                    "an entry of a map nor a field of an entry")
                mentioned.add(loc)
            got = _determines(ev, f, before, bound, frames, where, types, families)
            if got is not None:
                options.append(got)
    except Missing:
        return []  # the branch reads an entry its map does not have: false
    for loc in mentioned:
        if loc[1] not in names:
            raise PostError(f"{where}: the post gives {loc[1]}' a value, but {loc[1]} is no variable of the state after the step")
    joined: list[dict] = [{}]
    for o in options:
        joined = _join(joined, o)
        if not joined:
            return []
    out = []
    for a in joined:
        out.extend(_assemble(ev, a, names, frames, mentioned, types, where, families))
    return out


def _determines(ev: Evaluator, f, before, bound, frames, where, types, families) -> list[dict] | None:
    """The values after the step f determines, as options of assignments to
    locations; None when it determines none and is only checked."""
    def val(t):
        return ev.value(t, before, {}, bound)

    if isinstance(f, Builtin):
        a = f.args
        if f.op == "eq":
            for x, y in ((a[0], a[1]), (a[1], a[0])):
                if is_primed(x) and not is_primed(y):
                    return [{_loc(x, ev, before, bound): val(y)}]
            return None
        if f.op == "empty" and isinstance(a[0], Free) and a[0].primed:
            return [{Strict(("var", a[0].name)): FrozenMap()}]
        if f.op == "update" and isinstance(a[0], Free) and a[0].primed and not any(is_primed(t) for t in a[1:]):
            return [{Strict(("var", a[0].name)): FrozenMap(val(a[1])).set(val(a[2]), val(a[3]))}]
        if f.op == "frame" and isinstance(a[0], Free) and a[0].primed and not any(is_primed(t) for t in a[1:]):
            frames.setdefault(a[0].name, []).append((val(a[1]), [val(k) for k in a[2:]]))
            return None
        if f.op == "at":
            loc = _loc(a[0], ev, before, bound)
            if loc is not None and loc[0] == "entry":
                return [{Strict(("state",) + loc[1:]): f.state}]
            return None
        if f.op == "notin" and isinstance(a[1], Free) and a[1].primed and not is_primed(a[0]):
            return [{Strict(("entry", a[1].name, val(a[0]))): ABSENT}]
        return None
    if isinstance(f, Atom) and any(f.primed):
        locs, before_args = [], []
        for t, p in zip(f.args, f.primed):
            if p:
                locs.append(_loc(t, ev, before, bound))
            else:
                before_args.append(val(t))
        if ev.atoms.of_environment(f):
            for loc in locs:
                if loc[0] == "entry":
                    raise PostError(f"{where}: the environment decides {f.function!r} over the types of its values after the step, "
                                    f"but {loc[1]}'[{loc[2]!r}], an entry, has no type; give its state with @at and its fields instead")
        try:
            options = ev.atoms.post(f, before_args, [_loc_type(loc, types, families) for loc in locs])
        except KeyError as e:
            raise PostError(f"{where}: the environment decides {f.function!r} over the types of its values after the step: {e.args[0]}") from None
        for after in options:
            if len(after) != len(locs):
                raise PostError(f"{where}: the post atom {f.function!r} gave {after!r}, not one value for each of its {len(locs)} values after the step")
        return [dict(zip(locs, after)) for after in options]
    return None


def _loc_type(loc: tuple, types: Mapping[str, str], families: Mapping[str, Family]) -> str:
    """The type of the location loc, a variable or a field of an instance."""
    try:
        if loc[0] == "var":
            return types[loc[1]]
        return families[loc[1]].types[loc[3]]
    except KeyError:
        raise PostError(f"{loc[1]}{'' if loc[0] == 'var' else '.' + str(loc[3])} has no declared type") from None


def _assemble(ev: Evaluator, a: dict, names, frames, mentioned, types, where, families) -> list[dict]:
    """The valuations after the step that the assignment a to locations
    makes: none when a contradicts itself, and one per value of its type for
    a variable or a field of an instance nothing determines. An instance
    @at puts at a state holds exactly the variables its family gives that
    state."""
    afters: list[dict] = [{}]

    def typed(type_: str, what: str) -> list:
        try:
            return ev.carrier(type_)
        except KeyError as e:
            raise PostError(f"{where}: nothing in the post determines {what}, so it ranges over its type {type_}: "
                            f"{e.args[0]}") from None

    for x in names:
        if Strict(("var", x)) in a:
            afters = [dict(f, **{x: a[Strict(("var", x))]}) for f in afters]
            continue
        parts = [loc for loc in set(a) | mentioned if loc[0] != "var" and loc[1] == x]
        if x not in frames:
            if parts:
                raise PostError(f"{where}: the post gives {x}' only in part; say what the rest is with @frame or @update")
            afters = [dict(f, **{x: v}) for f in afters for v in typed(_loc_type(("var", x), types, families), f"{x}'")]
            continue
        kept = _kept(frames[x])
        if kept is None:
            return []
        choices: list[FrozenMap] = [kept]
        open_keys = [k for k in frames[x][0][1] if all(_among(k, ks) for _, ks in frames[x])]
        for k in open_keys:
            entry, state = a.get(Strict(("entry", x, k))), a.get(Strict(("state", x, k)))
            given = {loc[3]: v for loc, v in a.items() if loc[0] == "field" and loc[1] == x and same_value(loc[2], k)}
            fields = sorted({loc[3] for loc in mentioned if loc[0] == "field" and loc[1] == x and same_value(loc[2], k)} | set(given))
            if entry is ABSENT:
                if state is not None or fields:
                    return []
                continue
            if entry is not None:
                choices = [c.set(k, entry) for c in choices]
                continue
            if state is None:
                if fields:
                    raise PostError(f"{where}: the post gives fields of {x}'[{k!r}] but not its state; say @at({x}'[…], STATE)")
                raise PostError(f"{where}: the post leaves {x}'[{k!r}] unconstrained; say @notin(…, {x}') or @at({x}'[…], STATE)")
            own = _own_fields(families, x, k, state, where)
            stray = [n for n in fields if n not in own]
            if stray:
                raise PostError(f"{where}: the post gives {x}'[{k!r}].{stray[0]}, but an instance at {state} holds only "
                                f"{', '.join(own) or 'no field'}")
            records: list[dict] = [dict(given)]
            for name in own:
                if name not in given:
                    records = [dict(r, **{name: v}) for r in records for v in typed(families[x].types[name], f"{x}'[{k!r}].{name}")]
            choices = [c.set(k, Rec(state, FrozenMap(r))) for c in choices for r in records]
        afters = [dict(f, **{x: c}) for f in afters for c in choices]
    return afters


def _check_instances(after: dict, families: Mapping[str, Family], where: str) -> None:
    """Refuses a valuation the post allows in which the map of a family holds
    what no instance of it is: an instance is at a state of its family other
    than the start state, which is absence, and holds exactly that state's
    variables. However the post wrote the value - @eq of the map or an
    entry, @update, or over TYPES - the value is the post's, not the
    environment's, so it is the author's mistake and refused rather than
    taken for no successor."""
    for x, fam in families.items():
        if x not in after:
            continue
        m = after[x]
        if not isinstance(m, Mapping):
            raise PostError(f"{where}: the post gives {x}' the value {m!r}, which is no map")
        for k, v in m.items():
            why = _not_an_instance(fam, v)
            if why:
                raise PostError(f"{where}: the post gives {x}' the entry {k!r} = {v!r}, which no instance of {x} is: {why}")


def _not_an_instance(fam: Family, v) -> str | None:
    """Why v is no instance of fam, or None when it is one."""
    if not isinstance(v, Rec):
        return "it is no instance at all"
    if v.state not in fam.states:
        return f"{v.state} is no state of its family"
    if v.state == fam.init:
        return f"{v.state} is the start state, which is absence"
    own = fam.vars[v.state]
    if set(v.fields) != set(own):
        return f"at {v.state} it holds {', '.join(own) or 'no field'}, not {', '.join(sorted(v.fields)) or 'no field'}"
    return None


def _own_fields(families: Mapping[str, Family], x: str, k, state: str, where: str) -> tuple[str, ...]:
    """The variables an instance of the map x holds at state."""
    fam = families.get(x)
    if fam is None:
        raise PostError(f"{where}: the post puts {x}'[{k!r}] at {state}, but {x} is the map of no family, so nothing says "
                        "what an instance there holds; check the expansion csdfpromote -format json writes")
    if state not in fam.vars or state == fam.init:
        raise PostError(f"{where}: the post puts {x}'[{k!r}] at {state}, which is no state an instance of {x} may be at")
    return fam.vars[state]


def _among(k, keys) -> bool:
    return any(same_value(k, j) for j in keys)


def _kept(frames: list[tuple]) -> FrozenMap | None:
    """The entries the frames of one map fix: each key a frame does not open
    has the value its base gives it, or none when its base has no entry for
    it. None when two bases disagree on a key both fix: no map is both."""
    kept = FrozenMap()
    keys = [k for base, _ in frames for k in base]
    for k in keys:
        fixing = [base for base, ks in frames if not _among(k, ks)]
        if not fixing:
            continue
        first = fixing[0]
        for base in fixing[1:]:
            if (k in first) != (k in base) or (k in first and not same_value(first[k], base[k])):
                return None
        if k in first:
            kept = kept.set(k, first[k])
    return kept
