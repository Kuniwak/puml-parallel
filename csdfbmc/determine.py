"""What the equalities of a guard determine of the fields an event receives
(Q5, docs/CLAUSE_DESIGN.md), and the order a step asks the conjuncts in,
so that a term is computed before a conjunct reads what it binds
(csdfbmc.explore)."""
from __future__ import annotations

from dataclasses import dataclass

from csdfbmc.model import App, Bound, Builtin, Exists, Field, Free, Index, Lit, free_names, literal_parts, names_of


@dataclass(frozen=True)
class Bind:
    """The equality at its place in the guard binds the field, or its part
    key when key is not None, to the value of term."""
    field: str
    key: object
    term: object


def determine(guard: tuple, fields: list[str], known: set[str], ranged: frozenset) -> dict[int, Bind]:
    """The equalities of guard that bind a field, by their places in it.

    An equality builtin binds a field f, f = t, or its part f.k or f[k]
    for a literal key k, when t reads no value after the step, not f, and only
    the names known and the fields; a field it reads that nothing has bound
    yet ranges then. A part f[k] may be bound to a term over other parts of
    f, read f.j or f["j"]; a part the term of an equality before reads
    ranges, and reading a part alone does not read the field. A conjunct
    that binds nothing does not keep a field from binding, wherever it is:
    the conjuncts being pure and raising nothing (README), it is asked
    after the equalities binding what it reads (place). One equality binds
    a key, the first, and none binds a field whose parts one binds before.
    A field in ranged, or one the term of an equality before reads, ranges.
    Of the two sides of an equality, the one whose term reads only fields
    before it in fields is bound first."""
    order = {f: i for i, f in enumerate(fields)}
    reads_first: set[str] = set(ranged)
    whole: set[str] = set()
    parts: dict[str, list] = {}
    read_parts: dict[str, set] = {}
    closed: set[str] = set()
    binds: dict[int, Bind] = {}

    def bindable(f: str, key) -> bool:
        if f not in order or f in reads_first or f in whole or f in closed:
            return False
        if key is None:
            return f not in parts and f not in read_parts
        return key not in parts.get(f, []) and key not in read_parts.get(f, set())

    def read(names: set) -> None:
        for n in names:
            if isinstance(n, tuple) and n[0] in order:
                read_parts.setdefault(n[0], set()).add(n[1])
        for g in names & order.keys():
            if g in parts:
                closed.add(g)
            elif g not in whole:
                reads_first.add(g)

    for i, c in enumerate(guard):
        bind = _bind(c, order, known, bindable)
        if bind is None:
            continue
        binds[i] = bind
        if bind.key is None:
            whole.add(bind.field)
        else:
            parts.setdefault(bind.field, []).append(bind.key)
        read(_literal_reads(bind.term, primed=None))
    return binds


def blocked(guard: tuple, fields: list[str], known: set[str], f: str, key) -> object | None:
    """The first equality of guard that would bind the part key of the
    field f were it the guard alone, when the guard binds no such part, so
    that an equality before it, binding by a term that reads the part,
    keeps it from binding; None otherwise. No field ranges but those the guard reads first."""
    def binds(g: tuple) -> bool:
        return any((b.field, b.key) == (f, key) for b in determine(g, fields, known, frozenset()).values())
    if binds(guard):
        return None
    return next((c for c in guard if binds((c,))), None)


def _literal_reads(t, primed: bool | None) -> set:
    """free_names of t, each part with a literal key as (f, k)."""
    return free_names(t, primed=primed, parts=literal_parts(t))


def _bind(c, order: dict[str, int], known: set[str], bindable) -> Bind | None:
    """The Bind c makes, when it is an equality builtin one side of which
    bindable allows and the other a term it may be bound to."""
    if not (isinstance(c, Builtin) and c.op == "eq"):
        return None
    found = []
    for x, y in ((c.args[0], c.args[1]), (c.args[1], c.args[0])):
        level = _level(x)
        if level is None:
            continue
        f, key = level
        reads = _literal_reads(y, primed=False)
        names = names_of(reads)
        if free_names(y, primed=True) or f in reads or (f, key) in reads or not names <= known | order.keys() or not bindable(f, key):
            continue
        found.append((not all(order[g] < order[f] for g in names & order.keys()), Bind(f, key, y)))
    return min(found, key=lambda t: t[0])[1] if found else None


def place(guard: tuple, binds: dict[int, Bind], levels: list) -> tuple[list, int]:
    """(the steps of guard, the number of leading steps the product asks).

    levels names the levels of the product in order: a field that ranges,
    or (field, key) for a part of one that ranges. A step is (the depth of
    the levels it reads, its place in guard, its Bind or None), in guard's
    order but for a conjunct that binds nothing and reads what a later
    equality binds, which follows that equality (_bound_first); the depth never decreases, and the last equality binding a part
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
    for i in _bound_first(guard, binds, last_part):
        c, b = guard[i], binds.get(i)
        reads: set = part_reads(b.term if b else c, depth.keys())
        if b is not None and last_part.get(b.field) == i:
            reads = reads | {n for n in levels if isinstance(n, tuple) and n[0] == b.field}
        at = max([at] + [depth[n] for n in reads if n in depth])
        steps.append((at, i, b))
    inside = max([sum(1 for d, _, _ in steps if d < len(levels))] + [j + 1 for j, (_, _, b) in enumerate(steps) if b is not None])
    return steps, inside


def _bound_first(guard: tuple, binds: dict[int, Bind], last_part: dict[str, int]) -> list[int]:
    """The places of guard in its order, but a conjunct that binds nothing
    and reads a field or a part a later equality binds right after the last
    such equality, so that it reads them bound; the conjuncts being pure
    and raising nothing (README), the order they are asked in does not
    matter."""
    at: dict = {}
    for j, b in binds.items():
        at[(b.field, b.key)] = j
        if b.key is None:
            at[b.field] = j
    after: dict[int, list] = {}
    order = []
    for i, c in enumerate(guard):
        if i not in binds:
            reads = _literal_reads(c, primed=False)
            places = [at[n] for n in reads | names_of(reads) if n in at]
            places += [last_part[f] for f in reads if f in last_part]
            j = max(places, default=-1)
            if j > i:
                after.setdefault(j, []).append(i)
                continue
        order.append(i)
        order += after.pop(i, [])
    return order


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
    level, a field or a part of one with a literal key, and m reads only
    the names known before the step. The values of such a level outside m
    make the guard false, so drawing the level from m drops only values
    evaluating the guard in full would reject; the conjuncts being pure
    and raising nothing (README), the order they are asked in does not
    matter."""
    names = set(levels)
    out: dict[int, object] = {}
    for c in guard:
        if isinstance(c, Builtin) and c.op == "in":
            t, m = c.args
            d = _level_of(t, levels, names)
            if d is not None and d not in out and not free_names(m, primed=True) and free_names(m, primed=False) <= known:
                out[d] = m
    return out


def _level_of(t, levels: list, names: set) -> int | None:
    """The place in levels of the level t is, a field or a part of one with
    a literal key; None when t is no level."""
    read = part_reads(t, names)
    return levels.index(next(iter(read))) if _is_level(t) and len(read) == 1 and read <= names else None




def _level(t) -> tuple[str, object] | None:
    """(f, None) when t is a variable f before the step, (f, k) when it is
    a part of one by a literal key, f.k or f[k]; None otherwise."""
    if isinstance(t, Free) and not t.primed:
        return t.name, None
    if isinstance(t, Field) and isinstance(t.of, Free) and not t.of.primed:
        return t.of.name, t.name
    if isinstance(t, Index) and isinstance(t.map, Free) and not t.map.primed and isinstance(t.key, Lit):
        return t.map.name, t.key.value
    return None


def _is_level(t) -> bool:
    """Whether t is a field, or a part of one by a literal key."""
    return _level(t) is not None




@dataclass(frozen=True)
class Keyed:
    """The conjunct at place in the guard reads the map at the field, m[f]
    or @in(f, m), so it is false of every value of f the map has not as a
    key: levels, the places of the parts of f in the levels of the
    product, and keys, the key of the record each of them is."""
    place: int
    field: str
    levels: tuple
    keys: tuple
    map: object


def keyed(guard: tuple, levels: list, known: set[str]) -> Keyed | None:
    """The first conjunct of guard, outside an ∃, that reads a map m at a
    field f whose parts range one by one, m[f] anywhere in its terms or
    @in(f, m), m over the names known before the step. Reading m at a key
    it has not makes the conjunct false (Missing), so the values of f's
    parts no key of m gives make the guard false, and drawing them from
    m's keys drops only values evaluating the guard in full would reject;
    the conjuncts being pure and raising nothing (README), the order they
    are asked in does not matter."""
    for i, c in enumerate(guard):
        if not isinstance(c, Exists):
            for f, m in _keyings(c):
                places = tuple(d for d, name in enumerate(levels) if isinstance(name, tuple) and name[0] == f)
                if places and _over(m, known, set()):
                    return Keyed(i, f, places, tuple(levels[d][1] for d in places), m)
    return None


def _keyings(c) -> list[tuple[str, object]]:
    """(f, m) for every m[f], f a field, among the terms of the conjunct c,
    and for @in(f, m), in the order they are read."""
    out: list = []
    if isinstance(c, Builtin) and c.op == "in" and isinstance(c.args[0], Free) and not c.args[0].primed:
        out.append((c.args[0].name, c.args[1]))

    def walk(t):
        if isinstance(t, Index):
            walk(t.map)
            walk(t.key)
            if isinstance(t.key, Free) and not t.key.primed:
                out.append((t.key.name, t.map))
        elif isinstance(t, Field):
            walk(t.of)
        elif hasattr(t, "args") and not isinstance(t, Exists):
            for a in t.args:
                walk(a)
    for a in getattr(c, "args", ()):
        walk(a)
    return out


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
    """The first ∃ of guard that draws levels from the entries of a map:
    its body is @in(j, m), j its bound name and m a term over the names
    known before the step, which may apply environment functions, as an
    equality that binds may (determine), then equalities, each of a level not bound before in
    it, a field or a part of one with a literal key, and a term over j
    and the names known, which may apply environment functions too; and the levels it binds are
    next to each other in levels. The values of those levels no entry
    gives make the ∃, and so the guard, false, so drawing them from the
    entries drops only values evaluating the guard in full would reject;
    the conjuncts being pure and raising nothing (README), the order they
    are asked in does not matter."""
    names = set(levels)
    for i, c in enumerate(guard):
        if isinstance(c, Exists):
            w = _witness(i, c, levels, names, known)
            if w is not None:
                return w
    return None


def _witness(i: int, c, levels: list, names: set, known: set[str]) -> Witness | None:
    """The Witness the ∃ c at place i makes, when its body has the shape
    witness asks, the levels set aside."""
    if len(c.body) < 2 or not (isinstance(c.body[0], Builtin) and c.body[0].op == "in"):
        return None
    j, m = c.body[0].args
    if j != Bound(0) or not _over(m, known, set(), apply=True):
        return None
    bound, terms = [], []
    for e in c.body[1:]:
        if not (isinstance(e, Builtin) and e.op == "eq"):
            return None
        found = _equal_level(e, levels, names, known)
        if found is None or found[0] in bound:
            return None
        bound.append(found[0])
        terms.append(found[1])
    if max(bound) - min(bound) + 1 != len(bound):
        return None
    return Witness(i, tuple(bound), m, c.type, tuple(terms))


def _equal_level(e, levels: list, names: set, known: set[str]) -> tuple[int, object] | None:
    """(the place of the level, the term) of the equality e, one side a
    level and the other a term over the bound index 0 and the names known,
    which may apply environment functions; None when it is not one."""
    for x, y in ((e.args[0], e.args[1]), (e.args[1], e.args[0])):
        d = _level_of(x, levels, names)
        if d is not None and _over(y, known, {0}, apply=True):
            return d, y
    return None


def _over(t, known: set[str], indices: set[int], apply: bool = False) -> bool:
    """Whether t is a term over the names known before the step and the
    bound indices given, applying environment functions only when apply
    says so."""
    if isinstance(t, Free):
        return not t.primed and t.name in known
    if isinstance(t, Bound):
        return t.index in indices
    if isinstance(t, Lit):
        return True
    if isinstance(t, Index):
        return _over(t.map, known, indices, apply) and _over(t.key, known, indices, apply)
    if isinstance(t, Field):
        return _over(t.of, known, indices, apply)
    if isinstance(t, App):
        return apply and all(_over(a, known, indices, apply) for a in t.args)
    return False
