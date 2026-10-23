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
constrains ranges over its type in TYPES (refused without one). Every
valuation found is then checked against the whole post, so what
determines a value is an optimisation of the meaning, never a part of it.
"""
from __future__ import annotations

import itertools
from collections.abc import Mapping
from typing import Callable, Protocol

from csdfbmc.model import App, Atom, Bound, Builtin, Exists, Field, Free, Index, is_primed
from csdfbmc.values import FrozenMap, Rec


class Missing(Exception):
    """A term read an entry its map does not have."""


class Atoms(Protocol):
    def guard(self, atom: Atom, args: list) -> bool: ...
    def post(self, atom: Atom, before: list, names: list[str]) -> list[tuple]: ...
    def post_holds(self, atom: Atom, args: list) -> bool: ...


def same_value(a, b) -> bool:
    """Whether two values are one value: of one kind as well as equal, so
    that 1 is not True, all the way into containers."""
    if type(a) is not type(b):
        return False
    if isinstance(a, tuple):
        return len(a) == len(b) and all(same_value(x, y) for x, y in zip(a, b))
    if isinstance(a, Mapping):
        return a.keys() == b.keys() and all(same_value(a[k], b[k]) for k in a)
    if isinstance(a, Rec):
        return a.state == b.state and same_value(a.fields, b.fields)
    return a == b


class Evaluator:
    """Evaluates clauses: atoms says what an atom means; carrier gives the
    values of a type, which an ∃ ranges over; apply gives the value of an
    environment function applied to values."""

    def __init__(self, atoms: Atoms, carrier: Callable[[str], list], apply: Callable[[str, tuple], object] | None = None):
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
            if self.apply is None:
                raise ValueError(f"no environment function {t.fn} is given")
            return self.apply(t.fn, tuple(self.value(a, before, after, bound) for a in t.args))
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
        for k in set(m0) | set(m1):
            if k in keys:
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

ABSENT = object()  # an entry a post says is not in its map


class PostError(ValueError):
    """A post that leaves part of the state after the step unconstrained in a
    way the walk cannot enumerate."""


def _loc(t, ev: Evaluator, before, bound):
    """The location a primed term names: ("var", x) for x', ("entry", m, k)
    for m'[k], ("field", m, k, f) for m'[k].f; None for another shape."""
    if isinstance(t, Free) and t.primed:
        return ("var", t.name)
    if isinstance(t, Index) and isinstance(t.map, Free) and t.map.primed and not is_primed(t.key):
        return ("entry", t.map.name, ev.value(t.key, before, {}, bound))
    if isinstance(t, Field) and isinstance(t.of, Index) and isinstance(t.of.map, Free) and t.of.map.primed \
            and not is_primed(t.of.key):
        return ("field", t.of.map.name, ev.value(t.of.key, before, {}, bound), t.name)
    return None


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
          domain: Callable[[str], list], where: str) -> list[dict]:
    """The valuations of names, the variables of the state after the step,
    that post allows from before. domain gives the values of a name's type;
    where names the edge in a refusal."""
    out: list[dict] = []
    for branch in _branches(post, (), ev.carrier):
        for after in _solve_branch(ev, branch, names, before, domain, where):
            if all(ev._holds(f, before, after, bound) for f, bound in branch):
                if not any(all(same_value(after[n], o[n]) for n in names) for o in out):
                    out.append(after)
    return out


def _solve_branch(ev: Evaluator, branch: list[tuple], names, before, domain, where) -> list[dict]:
    options: list[list[dict]] = []
    frames: dict[str, tuple] = {}
    mentioned: set[str] = set()
    for f, bound in branch:
        for t in getattr(f, "args", ()):
            if is_primed(t):
                r = t
                while not isinstance(r, Free):
                    r = r.map if isinstance(r, Index) else r.of
                mentioned.add(r.name)
        try:
            got = _determines(ev, f, before, bound, domain, frames)
        except Missing:
            return []  # the branch reads an entry its map does not have: false
        if got is not None:
            options.append(got)
    joined: list[dict] = [{}]
    for o in options:
        joined = _join(joined, o)
        if not joined:
            return []
    for x in mentioned:
        if x not in names:
            raise PostError(f"{where}: the post gives {x}' a value, but {x} is no variable of the state after the step")
    out = []
    for a in joined:
        out.extend(_assemble(a, names, frames, domain, where))
    return out


def _determines(ev: Evaluator, f, before, bound, domain, frames) -> list[dict] | None:
    """The values after the step f determines, as options of assignments to
    locations; None when it determines none and is only checked."""
    def val(t):
        return ev.value(t, before, {}, bound)

    if isinstance(f, Builtin):
        a = f.args
        if f.op == "eq":
            for x, y in ((a[0], a[1]), (a[1], a[0])):
                if is_primed(x) and not is_primed(y):
                    loc = _loc(x, ev, before, bound)
                    if loc is not None:
                        return [{loc: val(y)}]
            return None
        if f.op == "empty" and isinstance(a[0], Free) and a[0].primed:
            return [{("var", a[0].name): FrozenMap()}]
        if f.op == "update" and isinstance(a[0], Free) and a[0].primed and not any(is_primed(t) for t in a[1:]):
            m0 = val(a[1])
            return [{("var", a[0].name): FrozenMap(m0).set(val(a[2]), val(a[3]))}]
        if f.op == "frame" and isinstance(a[0], Free) and a[0].primed and not any(is_primed(t) for t in a[1:]):
            frames.setdefault(a[0].name, (val(a[1]), [val(k) for k in a[2:]]))
            return None
        if f.op == "at":
            loc = _loc(a[0], ev, before, bound)
            if loc is not None and loc[0] == "entry":
                return [{loc: ("at", f.state)}]
            return None
        if f.op == "notin" and isinstance(a[1], Free) and a[1].primed and not is_primed(a[0]):
            return [{("entry", a[1].name, val(a[0])): ABSENT}]
        return None
    if isinstance(f, Atom) and any(f.primed):
        locs, before_args = [], []
        for t, p in zip(f.args, f.primed):
            if p:
                loc = _loc(t, ev, before, bound)
                if loc is None or loc[0] == "entry":
                    raise PostError(f"the post atom {f.function!r} reads a value after the step as {t!r}, "
                                    "which is neither a variable nor a field of an entry")
                locs.append(loc)
            else:
                before_args.append(val(t))
        names = [loc[1] if loc[0] == "var" else loc[3] for loc in locs]
        return [dict(zip(locs, after)) for after in ev.atoms.post(f, before_args, names)]
    return None


def _assemble(a: dict, names, frames, domain, where) -> list[dict]:
    """The valuations after the step that the assignment a to locations
    makes: none when a contradicts itself, and one per value of its type for
    a variable nothing determines."""
    afters: list[dict] = [{}]
    for x in names:
        parts = [loc for loc in a if loc[0] != "var" and loc[1] == x]
        if ("var", x) in a:
            afters = [dict(f, **{x: a[("var", x)]}) for f in afters]
            continue
        if x not in frames:
            if parts:
                raise PostError(f"{where}: the post gives {x}' only in part; say what the rest is with @frame or @update")
            try:
                values = domain(x)
            except KeyError:
                raise PostError(f"{where}: nothing in the post determines {x}', so it ranges over its type, and {x} has none "
                                "in TYPES; constrain it, or declare its type") from None
            afters = [dict(f, **{x: v}) for f in afters for v in values]
            continue
        base, keys = frames[x]
        entries = {k: v for k, v in base.items() if not any(same_value(k, j) for j in keys)}
        for k in keys:
            entry = a.get(("entry", x, k))
            fields = {loc[3]: v for loc, v in a.items() if loc[0] == "field" and loc[1] == x and same_value(loc[2], k)}
            if entry is None:
                if fields:
                    raise PostError(f"{where}: the post gives fields of {x}'[{k!r}] but not its state; say @at({x}'[…], STATE)")
                raise PostError(f"{where}: the post leaves {x}'[{k!r}] unconstrained; say @notin(…, {x}') or @at({x}'[…], STATE)")
            if entry is ABSENT:
                if fields:
                    return []
                continue
            entries[k] = Rec(entry[1], FrozenMap(fields))
        afters = [dict(f, **{x: FrozenMap(entries)}) for f in afters]
    return afters
