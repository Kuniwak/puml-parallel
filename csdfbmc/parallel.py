"""The states of csdfbmc's walk expanded in worker processes (--jobs).

Expanding a state (csdfbmc.explore.Explorer.expand) reads nothing of the
states the walk met, so the states of the queue are expanded apart, each
worker by a walk of its own that it builds of the same arguments, glue
included. The walk itself stays in one process: it takes the expansions in
the order of its queue and decides alone which states it met, which it
stores, the ample sets and where it stops, so that it walks the states it
walks without workers, in the same order, and reports the same.
"""
from __future__ import annotations

import argparse
import contextlib
import multiprocessing
from collections.abc import Callable, Iterable, Iterator

from csdfbmc import cli
from csdfbmc.explore import Expansion, State
from csdfbmc.model import Diagram

# The walk of this worker, which its pool built (_start), or what building
# it raised; the index of each edge of its diagram, by identity; and what
# closes its glue.
_explorer = None
_failed: Exception | None = None
_index: dict[int, int] = {}
_stack = contextlib.ExitStack()


def _start(a: argparse.Namespace) -> None:
    """Builds the walk of this worker. What it raises is kept for the walk
    to raise: a pool restarts a worker whose start fails, forever."""
    global _explorer, _failed, _index
    try:
        _explorer = cli.explorer(a, *_stack.enter_context(cli.opened(a)))
        _index = {id(e): i for i, e in enumerate(_explorer.d.edges)}
    except Exception as e:  # noqa: BLE001 - passed on to the walk
        _failed = e


def _expand(state: State) -> Expansion | Exception:
    """The expansion of state, its edges by their index in the diagram, or
    what expanding it raised: the walk raises it only if it gets to the
    state."""
    if _failed is not None:
        return _failed
    try:
        x = _explorer.expand(state)
    except Exception as e:  # noqa: BLE001 - passed on to the walk
        return e
    x.transitions = [(name, label, t, _index[id(edge)], params) for name, label, t, edge, params in x.transitions]
    return x


def _received(x: Expansion | Exception, edges: list) -> Expansion:
    if isinstance(x, Exception):
        raise x
    x.transitions = [(name, label, t, edges[i], params) for name, label, t, i, params in x.transitions]
    return x


@contextlib.contextmanager
def expanding(a: argparse.Namespace, diagram: Diagram, jobs: int) -> Iterator[Callable[[list[State]], Iterable[Expansion]] | None]:
    """What expands the walk's states in jobs workers (Explorer.run's
    expand_all), each building its walk of the arguments a; None, the walk's
    own expansion, for one job. The workers stop on leaving."""
    if jobs == 1:
        yield None
        return
    with multiprocessing.get_context("spawn").Pool(jobs, initializer=_start, initargs=(a,)) as pool:
        def expand_all(states: list[State]) -> Iterable[Expansion]:
            chunk = max(1, len(states) // (jobs * 4))
            return (_received(x, diagram.edges) for x in pool.imap(_expand, states, chunk))
        yield expand_all
