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
import collections
import contextlib
import multiprocessing
import multiprocessing.context
import multiprocessing.pool
import signal
from collections.abc import Callable, Iterable, Iterator

from csdfbmc import cli, stopping
from csdfbmc.explore import Expansion, State
from csdfbmc.model import Diagram
from csdfbmc.randomwalk import Outcome
from csdfbmc.stopping import never

# The walk of this worker, which its pool built (_start), or what building
# it raised; the index of each edge of its diagram, by identity; and what
# closes its glue.
_explorer = None
_failed: Exception | None = None
_index: dict[int, int] = {}
_stack = contextlib.ExitStack()

# The states a task expands, and the tasks given out ahead of the walk per
# worker: small enough that the walk soon has the first of them, enough to
# keep every worker busy.
CHUNK = 8
AHEAD = 4

# How often, in seconds, the walk waiting for a worker asks whether to stop:
# a worker killed by a signal sent to the whole group never answers.
POLL = 1.0


class _Worker(multiprocessing.context.SpawnProcess):
    """A worker that the signals which stop the walk (csdfbmc.stopping)
    never reach: a ^C, or timeout's SIGTERM, goes to the whole process
    group, and a pool whose worker it killed hangs on leaving, waiting for
    a lock the worker held. It is born with them blocked, ignores them once
    started (_start), and is killed when the pool stops it."""

    def start(self) -> None:
        # The signal mask, unlike the handlers, outlives the exec: the
        # worker has them blocked until it ignores them.
        before = signal.pthread_sigmask(signal.SIG_BLOCK, stopping.SIGNALS)
        try:
            super().start()
        finally:
            signal.pthread_sigmask(signal.SIG_SETMASK, before)

    def terminate(self) -> None:
        self.kill()


class _Context(multiprocessing.context.SpawnContext):
    Process = _Worker


def _start(a: argparse.Namespace) -> None:
    """Builds the walk of this worker. What it raises is kept for the walk
    to raise: a pool restarts a worker whose start fails, forever."""
    global _explorer, _failed, _index
    # The walk alone stops on a signal, and leaving it stops the workers.
    for s in stopping.SIGNALS:
        signal.signal(s, signal.SIG_IGN)
    signal.pthread_sigmask(signal.SIG_UNBLOCK, stopping.SIGNALS)
    try:
        _explorer = cli.explorer(a, *_stack.enter_context(cli.opened(a)))
        _index = {id(e): i for i, e in enumerate(_explorer.d.edges)}
    except Exception as e:  # noqa: BLE001 - passed on to the walk
        _failed = e


def _in_worker(work: Callable, arg):
    """What work gives for arg with its edges sent by their index in the
    diagram (_sent), or what building the walk or working raised: the walk
    raises it only if it gets to the result."""
    if _failed is not None:
        return _failed
    try:
        return _sent(work(arg))
    except Exception as e:  # noqa: BLE001 - passed on to the walk
        return e


def _sent(x: Expansion | Outcome) -> Expansion | Outcome:
    """x with its edges by their index in the diagram, to send to the walk."""
    if isinstance(x, Expansion):
        x.transitions = [(name, label, t, _index[id(edge)], params) for name, label, t, edge, params in x.transitions]
    elif isinstance(x, Outcome):
        x.steps = [(label, _index[id(edge)], s) for label, edge, s in x.steps]
    else:
        raise TypeError(f"no edges of a {type(x).__name__} to send")
    return x


def _received(x: Expansion | Outcome | Exception, edges: list) -> Expansion | Outcome:
    """x as _in_worker sent it, its edges back from their indices; what
    the worker raised is raised."""
    if isinstance(x, Exception):
        raise x
    if isinstance(x, Expansion):
        x.transitions = [(name, label, t, edges[i], params) for name, label, t, i, params in x.transitions]
    elif isinstance(x, Outcome):
        x.steps = [(label, edges[i], s) for label, i, s in x.steps]
    else:
        raise TypeError(f"no edges of a {type(x).__name__} to take back")
    return x


def _expand_chunk(states: list[State]) -> list:
    return [_in_worker(_explorer.expand, s) for s in states]


# The random walks of this worker, which its pool built (_start_walks).
_walks = None


def _start_walks(a: argparse.Namespace) -> None:
    """Builds the random walks of this worker, as _start its walk."""
    global _walks
    _start(a)
    if _failed is None:
        _walks = cli.random_walks(a, _explorer)


def _walk_chunk(indices: range) -> list:
    return [_in_worker(_walks.walk, i) for i in indices]


def _waited(result: multiprocessing.pool.AsyncResult, stop: Callable[[], str | None]):
    """What result gives once a worker has it, or None when stop names a
    signal first."""
    while True:
        try:
            return result.get(timeout=POLL)
        except multiprocessing.TimeoutError:
            if stop() is not None:
                return None


def _in_order(pool: multiprocessing.pool.Pool, task: Callable, next_chunk: Callable, jobs: int,
              stop: Callable[[], str | None]) -> Iterator:
    """What task gives for each chunk next_chunk gives (None while it has
    none), the chunks given out AHEAD per worker ahead and the results
    taken back in order. It ends early when stop names a signal."""
    pending: collections.deque = collections.deque()
    while True:
        while len(pending) < jobs * AHEAD and (chunk := next_chunk()) is not None:
            pending.append(pool.apply_async(task, (chunk,)))
        if not pending:
            return
        results = _waited(pending.popleft(), stop)
        if results is None:
            return
        yield from results


@contextlib.contextmanager
def walking(a: argparse.Namespace, diagram: Diagram, jobs: int, stop: Callable[[], str | None] = never):
    """What takes the random walks in jobs workers (RandomWalks.run's take),
    in order, each worker building its walks of the arguments a; None, the
    walks' own, for one job. It ends early when stop names a signal. The
    workers stop on leaving."""
    if jobs == 1:
        yield None
        return
    with _Context().Pool(jobs, initializer=_start_walks, initargs=(a,)) as pool:
        def take(indices: range):
            chunks = iter(indices[i:i + CHUNK] for i in range(0, len(indices), CHUNK))
            for out in _in_order(pool, _walk_chunk, lambda: next(chunks, None), jobs, stop):
                yield _received(out, diagram.edges)
        yield take


@contextlib.contextmanager
def expanding(a: argparse.Namespace, diagram: Diagram, jobs: int, stop: Callable[[], str | None] = never) -> Iterator[Callable[[list[State], int], Iterable[Expansion]] | None]:
    """What expands the walk's states in jobs workers (Explorer.run's
    expand_all), each building its walk of the arguments a; None, the walk's
    own expansion, for one job. The workers stop on leaving.

    The states are given out in chunks of CHUNK, AHEAD per worker ahead of
    the walk, as the walk adds them, and taken back in order: a level of
    the walk follows the one before without waiting for the workers to
    start on it. It ends early when stop names a signal."""
    if jobs == 1:
        yield None
        return
    with _Context().Pool(jobs, initializer=_start, initargs=(a,)) as pool:
        def expand_all(queue: list[State], start: int) -> Iterator[Expansion]:
            given = start

            def added():
                # The walk adds to the queue as it takes the expansions in.
                nonlocal given
                if given >= len(queue):
                    return None
                chunk, given = queue[given:given + CHUNK], min(given + CHUNK, len(queue))
                return chunk

            for x in _in_order(pool, _expand_chunk, added, jobs, stop):
                yield _received(x, diagram.edges)
        yield expand_all
