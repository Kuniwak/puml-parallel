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
from typing import Callable, Iterable


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
    values than these, which the walk ranges over, and beyond gives some of
    them. A Sample is the tuple of its values, so it is written wherever
    the values are."""

    def __new__(cls, values: Iterable, beyond: tuple):
        self = super().__new__(cls, values)
        self.beyond = beyond
        return self


def sample(values: Iterable, beyond: Iterable) -> Sample:
    """The values of a type of TYPES given as a sample of the type, and
    beyond, values of the type that are not in the sample, at least one: at
    a state with no way out, csdfbmc tries them (README, Bounded model
    checking). They must be values of the type; csdfbmc cannot tell."""
    values, beyond = tuple(values), tuple(beyond)
    if not beyond:
        raise ValueError("sample(values, beyond) needs at least one value beyond the sample")
    for b in beyond:
        if any(same_value(b, v) for v in values):
            raise ValueError(f"sample(values, beyond) is given {b!r} beyond the sample, which is a value of it")
    return Sample(values, beyond)


class Built(list):
    """The values of a type record or partial_map built from the values of
    other types, parts (a parameter of the builder -> the values it was
    given), which build makes them from again: a type built of a sample is
    built again of the sample with the values beyond it."""

    def __init__(self, values: Iterable, build: Callable[..., "Built"], parts: dict):
        super().__init__(values)
        self.build = build
        self.parts = parts


def _kept(values: Iterable):
    """values as a part of a Built: a Sample or a Built as itself, so that
    what it is built of stays known; any other iterable as a tuple."""
    return values if isinstance(values, (Sample, Built)) else tuple(values)


def samples_in(values: Iterable) -> list[Sample]:
    """The samples the values of a type are, or are built of by record and
    partial_map, each once. Values listed by hand are built of nothing."""
    if isinstance(values, Sample):
        return [values]
    if isinstance(values, Built):
        found: dict[int, Sample] = {}
        for part in values.parts.values():
            found.update((id(x), x) for x in samples_in(part))
        return list(found.values())
    return []


def _widened(values: Iterable) -> Iterable:
    """values with each sample they are, or are built of, extended by the
    values beyond it, and built again."""
    if isinstance(values, Sample):
        return tuple(values) + values.beyond
    if isinstance(values, Built) and samples_in(values):
        return values.build(**{k: _widened(v) for k, v in values.parts.items()})
    return values


def beyond_samples(types: dict[str, Iterable]) -> dict[str, Iterable]:
    """types with each sample extended by all the values beyond it at once,
    and every type built of a sample built again of the wider one, so that
    a record of a sample also takes the values beyond it."""
    return {t: _widened(vs) for t, vs in types.items()}


def record(**fields: Iterable) -> Built:
    """Every record whose fields range over the given domains."""
    fields = {n: _kept(vs) for n, vs in fields.items()}
    names = list(fields)
    return Built([FrozenMap(zip(names, combo)) for combo in itertools.product(*(fields[n] for n in names))], record, fields)


def partial_map(keys: Iterable, values: Iterable, max_size: int | None = None) -> Built:
    """Every partial function from keys to values with at most max_size
    entries (all sizes when max_size is None), the empty one included."""
    keys = _kept(keys)
    values = _kept(values)
    limit = len(keys) if max_size is None else min(max_size, len(keys))
    out: list[FrozenMap] = []
    for size in range(0, limit + 1):
        for chosen in itertools.combinations(keys, size):
            for vals in itertools.product(values, repeat=size):
                out.append(FrozenMap(zip(chosen, vals)))
    return Built(out, lambda keys, values: partial_map(keys, values, max_size), {"keys": keys, "values": values})
