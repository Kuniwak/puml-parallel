"""Hashable values for the state variables, and builders for their domains.

A state is compared and hashed, so every value in it must be hashable. Scalars
and tuples are; a record or a finite partial function is written as a
FrozenMap, an immutable mapping that hashes by content. freeze turns the
containers Python makes by default (dict, list, set) into their hashable forms,
so a clause may give them as it finds natural.
"""
from __future__ import annotations

import itertools
from collections.abc import Mapping
from dataclasses import dataclass
from typing import Iterable


class FrozenMap(Mapping):
    """An immutable mapping, equal and hashable by content. Fields may also be
    read as attributes (m.残高), which keeps a clause readable."""

    __slots__ = ("_d", "_hash")

    def __init__(self, items=(), /, **kwargs):
        d = dict(items, **kwargs)
        object.__setattr__(self, "_d", d)
        object.__setattr__(self, "_hash", hash(frozenset(d.items())))

    def __getitem__(self, key):
        return self._d[key]

    def __iter__(self):
        return iter(self._d)

    def __len__(self):
        return len(self._d)

    def __hash__(self):
        return self._hash

    def __eq__(self, other):
        if isinstance(other, FrozenMap):
            return self._d == other._d
        if isinstance(other, Mapping):
            return self._d == dict(other)
        return NotImplemented

    def __getattr__(self, name):
        try:
            return self._d[name]
        except KeyError:
            raise AttributeError(name) from None

    def __setattr__(self, name, value):
        raise TypeError("FrozenMap is immutable; use set() or without()")

    def __repr__(self):
        return "{" + ", ".join(f"{k!r}: {v!r}" for k, v in self._d.items()) + "}"

    def set(self, key, value) -> "FrozenMap":
        d = dict(self._d)
        d[key] = value
        return FrozenMap(d)

    def without(self, key) -> "FrozenMap":
        d = dict(self._d)
        d.pop(key, None)
        return FrozenMap(d)


@dataclass(frozen=True)
class Rec:
    """An instance of a family, the value of an entry of a map: the state of
    its local diagram and the values of its fields. The state is a position,
    not a value (docs/CLAUSE_DESIGN.md, control state): only the builtin @at
    reads it, and a glue, a ghost or an invariant sees the fields alone
    (visible)."""
    state: str
    fields: "FrozenMap"


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


def visible(value):
    """value as a glue, a ghost update or an invariant sees it: each
    instance as the map of its fields, without its state."""
    if isinstance(value, Rec):
        return FrozenMap({k: visible(v) for k, v in value.fields.items()})
    if isinstance(value, FrozenMap):
        return FrozenMap({visible(k): visible(v) for k, v in value.items()})
    if isinstance(value, tuple):
        return tuple(visible(v) for v in value)
    if isinstance(value, frozenset):
        return frozenset(visible(v) for v in value)
    return value


def freeze(value):
    """Returns value in a hashable form: dicts become FrozenMaps, lists tuples,
    sets frozensets, recursively. Anything else must already be hashable."""
    if isinstance(value, FrozenMap):
        return value
    if isinstance(value, Mapping):
        return FrozenMap({k: freeze(v) for k, v in value.items()})
    if isinstance(value, (list, tuple)):
        return tuple(freeze(v) for v in value)
    if isinstance(value, (set, frozenset)):
        return frozenset(freeze(v) for v in value)
    hash(value)  # raises TypeError for what cannot be a state value
    return value


def encode(value):
    """value as JSON would carry it: FrozenMaps as objects, tuples and
    frozensets as arrays (a frozenset sorted when it can be), an instance as
    {"at": its state, "fields": its fields}."""
    if isinstance(value, Rec):
        return {"at": value.state, "fields": encode(value.fields)}
    if isinstance(value, Mapping):
        return {str(k): encode(v) for k, v in value.items()}
    if isinstance(value, (list, tuple)):
        return [encode(v) for v in value]
    if isinstance(value, (set, frozenset)):
        items = [encode(v) for v in value]
        try:
            return sorted(items)
        except TypeError:
            return items
    return value


class Sample(tuple):
    """The values of a type that are a sample of it: the type has more
    values than these, which the walk ranges over. A Sample is the tuple
    of its values, so it is written wherever the values are."""


def sample(values: Iterable) -> Sample:
    """The values of a type of TYPES given as a sample of the type: at a
    state with no way out, csdfbmc asks whether a value of the type that
    occurs nowhere would give one (README, Bounded model checking)."""
    return Sample(values)


def record(**fields: Iterable) -> list[FrozenMap]:
    """Every record whose fields range over the given domains."""
    names = list(fields)
    return [FrozenMap(zip(names, combo)) for combo in itertools.product(*(list(fields[n]) for n in names))]


def partial_map(keys: Iterable, values: Iterable, max_size: int | None = None) -> list[FrozenMap]:
    """Every partial function from keys to values with at most max_size
    entries (all sizes when max_size is None), the empty one included."""
    keys = list(keys)
    values = list(values)
    limit = len(keys) if max_size is None else min(max_size, len(keys))
    out: list[FrozenMap] = []
    for size in range(0, limit + 1):
        for chosen in itertools.combinations(keys, size):
            for vals in itertools.product(values, repeat=size):
                out.append(FrozenMap(zip(chosen, vals)))
    return out
