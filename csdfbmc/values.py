"""Hashable values for the state variables, and builders for their domains.

A state is compared and hashed, so every value in it must be hashable. Scalars
and tuples are; a record or a finite partial function is written as a
FrozenMap, an immutable mapping that hashes by content. freeze turns the
containers Python makes by default (dict, list, set) into their hashable forms,
so a twin may return them as it finds natural.
"""
from __future__ import annotations

import itertools
from collections.abc import Mapping
from typing import Iterable


class FrozenMap(Mapping):
    """An immutable mapping, equal and hashable by content. Fields may also be
    read as attributes (m.残高), which keeps a twin readable."""

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
