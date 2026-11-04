"""The product of the values of fields enumerated depth first, a prefix
dropped as soon as a check of it fails, asked as checking every complete
tuple would (csdfbmc.explore, the received fields of an event)."""
from __future__ import annotations

from collections.abc import Callable, Iterator


def pruned_product(levels: list[Callable[[tuple], list]], checks: list[Callable[[tuple], bool] | None],
                   on_prune: Callable[[tuple], None]) -> Iterator[tuple]:
    """The tuples of the product levels give, in order: levels[d] gives the
    values at depth d after the prefix so far. checks[d], when not None,
    decides whether the prefixes of length d are kept; it is asked of a
    prefix once, at the first complete tuple below it, the shallowest first,
    so that the checks are asked in the order asking them of every complete
    tuple would, but not again, and never of a prefix no complete tuple
    extends. A prefix it rejects is told on_prune, and nothing below it is
    enumerated further."""
    n = len(levels)
    verdicts: list[bool | None] = [None] * (n + 1)
    prefix: list = []

    def rejected_at() -> int | None:
        for d in range(n + 1):
            if checks[d] is not None and verdicts[d] is None:
                verdicts[d] = checks[d](tuple(prefix[:d]))
            if verdicts[d] is False:
                return d
        return None

    def grow(d: int) -> Iterator[tuple]:
        if d == n:
            at = rejected_at()
            if at is None:
                yield tuple(prefix)
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
