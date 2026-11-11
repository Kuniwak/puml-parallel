"""What the equalities of a guard determine of the fields an event receives
(Q5, docs/CLAUSE_DESIGN.md), read in the guard's order, so that a term is
computed where evaluating the guard left to right would reach its equality
(csdfbmc.explore)."""
from __future__ import annotations

from dataclasses import dataclass

from csdfbmc.model import Bound, Builtin, Exists, Field, Free, Index, Lit, free_names


@dataclass(frozen=True)
class Bind:
    """The equality at its place in the guard binds the field, or its part
    key when key is not None, to the value of term."""
    field: str
    key: object
    term: object


def determine(guard: tuple, fields: list[str], known: set[str], ranged: frozenset) -> dict[int, Bind]:
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


def place(guard: tuple, binds: dict[int, Bind], levels: list) -> tuple[list, int]:
    """(the steps of guard, the number of leading steps the product asks).

    levels names the levels of the product in order: a field that ranges,
    or (field, key) for a part of one that ranges. A step is (the depth of
    the levels it reads, its place in guard, its Bind or None), in guard's
    order; the depth never decreases, and the last equality binding a part
    of a field reads the levels of its other parts. A conjunct reads the
    level of a part where it reads the part, f.k or f["k"] (part_reads),
    and every level of the field where it reads the field otherwise. The product asks every
    step of a depth below the last, and those of the last up to the last
    Bind; the rest are the caller's."""
    depth = {name: d + 1 for d, name in enumerate(levels)}
    for d, name in enumerate(levels):
        if isinstance(name, tuple):
            depth[name[0]] = d + 1
    last_part = {b.field: i for i, b in binds.items() if b.key is not None}
    steps, at = [], 0
    for i, c in enumerate(guard):
        b = binds.get(i)
        reads: set = part_reads(b.term if b else c, depth.keys())
        if b is not None and last_part.get(b.field) == i:
            reads = reads | {n for n in levels if isinstance(n, tuple) and n[0] == b.field}
        at = max([at] + [depth[n] for n in reads if n in depth])
        steps.append((at, i, b))
    inside = max([sum(1 for d, _, _ in steps if d < len(levels))] + [j + 1 for j, (_, _, b) in enumerate(steps) if b is not None])
    return steps, inside


def part_reads(t, parts) -> set:
    """What t reads before the step: free_names, with a part f.k or
    f["k"] of f, k a literal, as (f, k) when (f, k) is in parts; a part of
    a part reads the outer part."""
    return free_names(t, primed=False, parts=parts)


def decides_early(guard: tuple, levels: list, steps: list, f: str) -> bool:
    """Whether one of steps, as place makes them of guard over levels,
    reads a part of the field f at a depth before the level of its last
    part, so that ranging over f's parts one by one decides it earlier."""
    parts = {name for name in levels if isinstance(name, tuple) and name[0] == f}
    last = max(d + 1 for d, name in enumerate(levels) if name in parts)
    return any(d < last and part_reads(guard[i], parts) & parts for d, i, _ in steps)


def draws(guard: tuple, levels: list, known: set[str]) -> dict[int, object]:
    """The levels a membership of guard decides, by their places in
    levels, each with the map term m of the first @in(t, m) where t is the
    level, a field or a part of one with a literal key, m reads only the
    names known before the step, and no conjunct before it reads the level
    or a later one. The values of such a level outside m are rejected by
    that conjunct before anything else of the guard reads them, so drawing
    the level from m drops only values evaluating the guard in full would
    reject there."""
    names = set(levels)
    out: dict[int, object] = {}
    deepest = -1
    for c in guard:
        if isinstance(c, Builtin) and c.op == "in" and _is_level(c.args[0]):
            t, m = c.args
            read = part_reads(t, names)
            d = levels.index(next(iter(read))) if len(read) == 1 and read <= names else None
            if d is not None and d > deepest and d not in out and not free_names(m, primed=True) and free_names(m, primed=False) <= known:
                out[d] = m
        deepest = max([deepest] + [d for d, name in enumerate(levels) if _reads(part_reads(c, names), name)])
    return out

def _is_level(t) -> bool:
    """Whether t is a field, or a part of one by a literal key."""
    if isinstance(t, Free):
        return not t.primed
    if isinstance(t, Field):
        return isinstance(t.of, Free) and not t.of.primed
    return isinstance(t, Index) and isinstance(t.map, Free) and not t.map.primed and isinstance(t.key, Lit)


def _reads(read: set, level) -> bool:
    """Whether what a conjunct reads, as part_reads says it, reads level."""
    return level in read or (isinstance(level, tuple) and level[0] in read)


@dataclass(frozen=True)
class Witness:
    """The conjunct at place in the guard, ∃ j : type. @in(j, map) ∧ l = t
    ∧ …, holds of the values of the levels it binds exactly when they are
    those the terms take at an entry j of map: levels, by their places in
    the levels of the product, in the order of its equalities, and terms,
    the term each is equal to, over j and the state."""
    place: int
    levels: tuple
    map: object
    type: str
    terms: tuple


def witness(guard: tuple, levels: list, known: set[str]) -> Witness | None:
    """The first ∃ of guard that draws levels from the entries of a map,
    when no conjunct before it reads one of those levels or a later one:
    its body is @in(j, m), j its bound name and m over the names known
    before the step, then equalities, each of a level not bound before in
    it, a field or a part of one with a literal key, and a term over j
    and the names known, applying no function; and the levels it binds are
    next to each other in levels. The values of those levels no entry
    gives make the ∃ false before anything else of the guard reads them,
    so drawing them from the entries drops only values evaluating the
    guard in full would reject there."""
    names = set(levels)
    deepest = -1
    for i, c in enumerate(guard):
        if isinstance(c, Exists):
            w = _witness(i, c, levels, names, known)
            if w is not None and min(w.levels) > deepest:
                return w
        deepest = max([deepest] + [d for d, name in enumerate(levels) if _reads(part_reads(c, names), name)])
    return None


def _witness(i: int, c, levels: list, names: set, known: set[str]) -> Witness | None:
    """The Witness the ∃ c at place i makes, when its body has the shape
    witness asks, the levels set aside."""
    if len(c.body) < 2 or not (isinstance(c.body[0], Builtin) and c.body[0].op == "in"):
        return None
    j, m = c.body[0].args
    if j != Bound(0) or not _over(m, known, set()):
        return None
    bound, terms = [], []
    for e in c.body[1:]:
        if not (isinstance(e, Builtin) and e.op == "eq"):
            return None
        found = None
        for x, y in ((e.args[0], e.args[1]), (e.args[1], e.args[0])):
            read = part_reads(x, names)
            if _is_level(x) and len(read) == 1 and read <= names and _over(y, known, {0}):
                found = (levels.index(next(iter(read))), y)
                break
        if found is None or found[0] in bound:
            return None
        bound.append(found[0])
        terms.append(found[1])
    if max(bound) - min(bound) + 1 != len(bound):
        return None
    return Witness(i, tuple(bound), m, c.type, tuple(terms))


def _over(t, known: set[str], indices: set[int]) -> bool:
    """Whether t is a term over the names known before the step and the
    bound indices given, applying no function."""
    if isinstance(t, Free):
        return not t.primed and t.name in known
    if isinstance(t, Bound):
        return t.index in indices
    if isinstance(t, Lit):
        return True
    if isinstance(t, Index):
        return _over(t.map, known, indices) and _over(t.key, known, indices)
    if isinstance(t, Field):
        return _over(t.of, known, indices)
    return False
