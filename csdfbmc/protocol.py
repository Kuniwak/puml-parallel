"""The evaluator protocol: how the checker asks a clause language about its
clauses, independent of the language.

Values cross the protocol in a tagged JSON encoding, since JSON has no
tuples, sets or maps with keys other than strings: integers, strings,
booleans and null as themselves, and {"tuple": [...]}, {"set": [...]},
{"map": [[key, value], ...]} for the containers of csdfbmc.values.
"""
from __future__ import annotations

from collections.abc import Mapping

from csdfbmc.values import FrozenMap

_TAGS = ("tuple", "set", "map")


def to_wire(value):
    """value, a value of the checker's model, in the tagged encoding."""
    if value is None or isinstance(value, (bool, int, str)):
        return value
    if isinstance(value, Mapping):
        return {"map": [[to_wire(k), to_wire(v)] for k, v in value.items()]}
    if isinstance(value, tuple):
        return {"tuple": [to_wire(v) for v in value]}
    if isinstance(value, (set, frozenset)):
        items = [to_wire(v) for v in value]
        try:
            items.sort()
        except TypeError:
            pass
        return {"set": items}
    raise TypeError(f"{value!r} is not a value the protocol carries: integers, strings, booleans, null, tuples, sets and maps")


def from_wire(wire):
    """The value of the checker's model that wire encodes."""
    if wire is None or isinstance(wire, (bool, int, str)):
        return wire
    if isinstance(wire, dict) and len(wire) == 1:
        (tag, items), = wire.items()
        if tag in _TAGS and isinstance(items, list):
            if tag == "tuple":
                return tuple(from_wire(v) for v in items)
            if tag == "set":
                return frozenset(from_wire(v) for v in items)
            if all(isinstance(kv, list) and len(kv) == 2 for kv in items):
                return FrozenMap((from_wire(k), from_wire(v)) for k, v in items)
    raise ValueError(f"{wire!r} is not a value in the tagged encoding: a scalar, or one of {list(_TAGS)} as the only key")
