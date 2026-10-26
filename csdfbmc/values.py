"""Hashable values for the state variables, and builders for their domains.

A state is compared and hashed, so every value in it must be hashable. Scalars
and tuples are; a record or a finite partial function is written as a
FrozenMap, an immutable mapping that hashes by content, and a set as a
FrozenSet. Python takes 1, True and 1.0 for one value; the language does not,
so every value is compared and hashed by its kind_key, and states are
Stricts. freeze turns the containers Python makes by default (dict, list,
set) into their hashable forms, so a clause may give them as it finds
natural.
"""
from __future__ import annotations

import itertools
from collections.abc import ItemsView, Mapping, Set as AbstractSet
from dataclasses import dataclass
from typing import Callable, Iterable


def kind_key(value):
    """value as a key that tells values apart by kind as well as by content:
    in Python, 1 == True == 1.0 and they hash alike, but they are three
    values of the language. Containers are keyed by the keys of what they
    hold, all the way down. A float's zero has no sign (docs/CLAUSE_DESIGN.md,
    Literals), so -0.0 and 0.0 have one key. Every value a state holds, every
    key and element of a map or set, and every state is compared and hashed
    by its kind_key."""
    if isinstance(value, Rec):
        return (Rec, value.state, kind_key(value.fields))
    if isinstance(value, FrozenMap):
        return value._key
    if isinstance(value, Mapping):
        return (Mapping, frozenset((kind_key(k), kind_key(v)) for k, v in value.items()))
    if isinstance(value, Strict):
        return value._key
    if isinstance(value, tuple):
        return (tuple, tuple(kind_key(v) for v in value))
    if isinstance(value, FrozenSet):
        return value._key
    if isinstance(value, AbstractSet):
        return (AbstractSet, frozenset(kind_key(v) for v in value))
    if isinstance(value, float):
        return (float, value + 0.0)
    return (type(value), value)


def same_value(a, b) -> bool:
    """Whether two values are one value: of one kind as well as equal, so
    that 1 is not True nor 1.0, all the way into containers (kind_key)."""
    return kind_key(a) == kind_key(b)


class FrozenMap(Mapping):
    """An immutable mapping, equal and hashable by content, its keys told
    apart by kind (kind_key): {1: a} and {True: a} are two maps of one
    entry each. Fields may also be read as attributes (m.残高), which keeps
    a clause readable."""

    __slots__ = ("_d", "_key")

    def __init__(self, items=(), /, **kwargs):
        pairs = items.items() if isinstance(items, Mapping) else items
        d: dict = {}
        for k, v in itertools.chain(pairs, kwargs.items()):
            d[kind_key(k)] = (k, v)
        object.__setattr__(self, "_d", d)
        object.__setattr__(self, "_key", (Mapping, frozenset((kk, kind_key(v)) for kk, (_, v) in d.items())))

    def __getitem__(self, key):
        return self._d[kind_key(key)][1]

    def __contains__(self, key):
        return kind_key(key) in self._d

    def __iter__(self):
        return (k for k, _ in self._d.values())

    def __len__(self):
        return len(self._d)

    def items(self):
        return _Items(self)

    def __hash__(self):
        # Python's hash of the entries: a map equal to this one has equal
        # entries, and so this hash, whatever its keys' kinds.
        return hash(frozenset(self._d.values()))

    def __eq__(self, other):
        if isinstance(other, Mapping):
            return self._key == kind_key(other)
        return NotImplemented

    def __getattr__(self, name):
        try:
            return self[name]
        except KeyError:
            raise AttributeError(name) from None

    def __setattr__(self, name, value):
        raise TypeError("FrozenMap is immutable; use set() or without()")

    def __repr__(self):
        return "{" + ", ".join(f"{k!r}: {v!r}" for k, v in self._d.values()) + "}"

    def set(self, key, value) -> "FrozenMap":
        return FrozenMap(itertools.chain(self._d.values(), [(key, value)]))

    def without(self, key) -> "FrozenMap":
        gone = kind_key(key)
        return FrozenMap(kv for kk, kv in self._d.items() if kk != gone)


class _Items(ItemsView):
    """The entries of a FrozenMap, an entry among them when its key and its
    value are of their kinds (kind_key)."""

    def __contains__(self, item):
        k, v = item
        return k in self._mapping and same_value(self._mapping[k], v)

    def __iter__(self):
        return iter(self._mapping._d.values())


class FrozenSet(AbstractSet):
    """An immutable set, equal and hashable by content, its elements told
    apart by kind (kind_key): {1, True, 1.0} has three elements."""

    __slots__ = ("_d", "_key")

    def __init__(self, items=()):
        d = {kind_key(x): x for x in items}
        object.__setattr__(self, "_d", d)
        object.__setattr__(self, "_key", (AbstractSet, frozenset(d)))

    @classmethod
    def _from_iterable(cls, it):
        return cls(it)

    def __contains__(self, x):
        return kind_key(x) in self._d

    def __iter__(self):
        return iter(self._d.values())

    def __len__(self):
        return len(self._d)

    def __hash__(self):
        # Python's hash of the elements, as a frozenset equal to this set has.
        return hash(frozenset(self._d.values()))

    def __eq__(self, other):
        if isinstance(other, AbstractSet):
            return self._key == kind_key(other)
        return NotImplemented

    def __setattr__(self, name, value):
        raise TypeError("FrozenSet is immutable")

    def __repr__(self):
        return "{" + ", ".join(map(repr, self._d.values())) + "}" if self._d else "set()"


class Strict(tuple):
    """A tuple equal and hashable by kind_key: a state, or what else the
    walk keys by values (a place of a post, a group of transitions), so that
    states holding 1 and True are two states."""

    def __new__(cls, items=()):
        self = super().__new__(cls, items)
        self._key = (tuple, tuple(kind_key(v) for v in self))
        return self

    def __hash__(self):
        # Python's hash of the tuple: a tuple equal to this one by kind is
        # equal to it in Python too, and so hashes alike.
        return tuple.__hash__(self)

    def __eq__(self, other):
        if isinstance(other, tuple):
            return self._key == kind_key(other)
        return NotImplemented

    def __ne__(self, other):
        eq = self.__eq__(other)
        return eq if eq is NotImplemented else not eq


@dataclass(frozen=True)
class Rec:
    """An instance of a family, the value of an entry of a map: the state of
    its local diagram and the values of its fields. The state is a position,
    not a value (docs/CLAUSE_DESIGN.md, control state): only the builtin @at
    reads it, and a glue, a ghost or an invariant sees the fields alone
    (visible)."""
    state: str
    fields: "FrozenMap"


class _LeftOut:
    """The value the walk keeps for a variable the cone of influence leaves
    out (csdfbmc.cone): one value, whatever the variable held."""

    def __repr__(self):
        return "left out"


LEFT_OUT = _LeftOut()


def unsigned_zero(f: float) -> float:
    """f, with negative zero read as zero: nothing in the language can tell
    them apart (docs/CLAUSE_DESIGN.md, Literals)."""
    return f + 0.0


def visible(value):
    """value as a glue, a ghost update or an invariant sees it: each
    instance as the map of its fields, without its state."""
    if isinstance(value, Rec):
        return FrozenMap((k, visible(v)) for k, v in value.fields.items())
    if isinstance(value, FrozenMap):
        return FrozenMap((visible(k), visible(v)) for k, v in value.items())
    if isinstance(value, tuple):
        return tuple(visible(v) for v in value)
    if isinstance(value, AbstractSet):
        return FrozenSet(visible(v) for v in value)
    return value


def freeze(value):
    """Returns value in a hashable form: dicts become FrozenMaps, lists tuples,
    sets FrozenSets, recursively. Anything else must already be hashable."""
    if isinstance(value, FrozenMap):
        return value
    if isinstance(value, Mapping):
        return FrozenMap((freeze(k), freeze(v)) for k, v in value.items())
    if isinstance(value, (list, tuple)):
        return tuple(freeze(v) for v in value)
    if isinstance(value, AbstractSet):
        return FrozenSet(freeze(v) for v in value)
    hash(value)  # raises TypeError for what cannot be a state value
    return value


def encode(value):
    """value as JSON would carry it: FrozenMaps as objects, tuples and
    sets as arrays (sorted when they can be), an instance as
    {"at": its state, "fields": its fields}, and a value left out as
    {"left_out": true}."""
    if value is LEFT_OUT:
        return {"left_out": True}
    if isinstance(value, Rec):
        return {"at": value.state, "fields": encode(value.fields)}
    if isinstance(value, Mapping):
        return {str(k): encode(v) for k, v in value.items()}
    if isinstance(value, (list, tuple)):
        return [encode(v) for v in value]
    if isinstance(value, AbstractSet):
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
