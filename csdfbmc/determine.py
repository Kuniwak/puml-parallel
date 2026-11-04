"""What the equalities of a guard determine of the fields an event receives
(Q5, docs/CLAUSE_DESIGN.md), read in the guard's order, so that a term is
computed where evaluating the guard left to right would reach its equality
(csdfbmc.explore)."""
from __future__ import annotations

from dataclasses import dataclass

from csdfbmc.model import Builtin, Free, Index, Lit, free_names


@dataclass(frozen=True)
class Bind:
    """The equality at its place in the guard binds the field, or its part
    key when key is not None, to the value of term."""
    field: str
    key: object
    term: object


def determine(guard: tuple, fields: list[str], known: set[str], ranged: frozenset = frozenset()) -> dict[int, Bind]:
    """The equalities of guard that bind a field, by their places in it.

    An equality builtin binds a field f, f = t, or its part f[k] for a
    literal key k, when t reads no value after the step, not f, and only
    the names known and the fields, and no conjunct before it reads f; a
    field it reads that nothing has bound yet ranges then. The parts of a
    field are bound until a conjunct reads the field, which then holds
    the parts bound before; one equality binds a key, the first. A field
    in ranged, or one read before an equality binds it, ranges. Of the two
    sides of an equality, the one whose term reads only fields before it in
    fields is bound first."""
    order = {f: i for i, f in enumerate(fields)}
    reads_first: set[str] = set(ranged)
    whole: set[str] = set()
    parts: dict[str, list] = {}
    closed: set[str] = set()
    binds: dict[int, Bind] = {}

    def bindable(f: str, key) -> bool:
        if f not in order or f in reads_first or f in whole or f in closed:
            return False
        return key not in parts.get(f, []) if key is not None else f not in parts

    def read(names: set[str]) -> None:
        for g in names & order.keys():
            if g in parts:
                closed.add(g)
            elif g not in whole:
                reads_first.add(g)

    for i, c in enumerate(guard):
        bind = _bind(c, order, known, bindable)
        if bind is None:
            read(free_names(c))
            continue
        binds[i] = bind
        if bind.key is None:
            whole.add(bind.field)
        else:
            parts.setdefault(bind.field, []).append(bind.key)
        read(free_names(bind.term))
    return binds


def _bind(c, order: dict[str, int], known: set[str], bindable) -> Bind | None:
    """The Bind c makes, when it is an equality builtin one side of which
    bindable allows and the other a term it may be bound to."""
    if not (isinstance(c, Builtin) and c.op == "eq"):
        return None
    found = []
    for x, y in ((c.args[0], c.args[1]), (c.args[1], c.args[0])):
        if isinstance(x, Free) and not x.primed:
            f, key = x.name, None
        elif isinstance(x, Index) and isinstance(x.map, Free) and not x.map.primed and isinstance(x.key, Lit):
            f, key = x.map.name, x.key.value
        else:
            continue
        names = free_names(y, primed=False)
        if free_names(y, primed=True) or f in names or not names <= known | order.keys() or not bindable(f, key):
            continue
        found.append((not all(order[g] < order[f] for g in names & order.keys()), Bind(f, key, y)))
    return min(found, key=lambda t: t[0])[1] if found else None
