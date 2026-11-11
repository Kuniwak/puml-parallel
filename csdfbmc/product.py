"""The product of the values of fields enumerated depth first, a prefix
dropped as soon as a step of it fails, asked as asking every complete
tuple would (csdfbmc.explore, the received fields of an event)."""
from __future__ import annotations

from collections.abc import Callable, Iterator

from csdfbmc.values import kind_key

# A step of the prefixes of one length: given the prefix and what the
# steps of the shorter prefixes bound, what it binds besides, or None when
# it rejects the prefix.
Step = Callable[[tuple, dict], "dict | None"]


def pruned_product(levels: list[Callable[[tuple], list]], steps: list[Step | None],
                   on_prune: Callable[[tuple], None] = lambda prefix: None) -> Iterator[tuple[tuple, dict]]:
    """The tuples of the product levels give, in order, each with what the
    steps bound of it: levels[d] gives the values at depth d after the
    prefix so far. steps[d], when not None, is asked of the prefixes of
    length d, with what the steps of the shorter ones bound; it is asked of
    a prefix once, at the first complete tuple below it, the shallowest
    first, so that the steps are asked in the order asking them of every
    complete tuple would, but not again, and never of a prefix no complete
    tuple extends. A prefix it rejects is told on_prune, and nothing below
    it is enumerated further."""
    n = len(levels)
    # verdicts[d]: None unasked, False rejected, else what steps[d] bound.
    verdicts: list = [None] * (n + 1)
    prefix: list = []

    def bound_at() -> tuple[int | None, dict]:
        bound: dict = {}
        for d in range(n + 1):
            if steps[d] is None:
                continue
            if verdicts[d] is None:
                verdicts[d] = steps[d](tuple(prefix[:d]), dict(bound))
                if verdicts[d] is None:
                    verdicts[d] = False
            if verdicts[d] is False:
                return d, bound
            bound.update(verdicts[d])
        return None, bound

    def grow(d: int) -> Iterator[tuple[tuple, dict]]:
        if d == n:
            at, bound = bound_at()
            if at is None:
                yield tuple(prefix), bound
            else:
                on_prune(tuple(prefix[:at]))
            return
        for v in levels[d](tuple(prefix)):
            prefix.append(v)
            verdicts[d + 1] = None
            yield from grow(d + 1)
            prefix.pop()
            if False in verdicts[:d + 1]:
                return

    yield from grow(0)


def keys_among(values: list, m) -> list:
    """The values m has as keys, in values' order; the first of values
    when it has none, so that a step reading the level still meets one
    value and can reject it (csdfbmc.explore, a level drawn from a map)."""
    return [v for v in values if v in m] or values[:1]


def entry_levels(values: dict[int, list], entries) -> dict[int, Callable[[tuple], list]]:
    """The levels values names, by their depths, drawn from entries: each
    entry gives a value at every one of those depths, and the level at a
    depth gives, after a prefix, the values of its list that the entries
    agreeing with the prefix at the shallower of those depths give, in the
    list's order; the first of its list when none does, so that a step
    reading the level still meets one value and can reject it. A value an
    entry gives outside the list of its depth makes the entry give none.
    Values are told apart by kind_key."""
    ordered = sorted(values)
    at = {d: {kind_key(v): n for n, v in enumerate(values[d])} for d in ordered}
    places = set()
    for got in entries:
        p = tuple(at[d].get(kind_key(got[d])) for d in ordered)
        if None not in p:
            places.add(p)

    def level(s: int) -> Callable[[tuple], list]:
        vs = values[ordered[s]]

        def of(prefix: tuple) -> list:
            head = tuple(at[d][kind_key(prefix[d])] for d in ordered[:s])
            return [vs[n] for n in sorted({p[s] for p in places if p[:s] == head})] or vs[:1]
        return of
    return {d: level(s) for s, d in enumerate(ordered)}
