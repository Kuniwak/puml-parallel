"""csdfbmc stopped by a signal (SIGINT, SIGTERM): the walk ends where it
is with what it found so far, so that a run cut short by timeout or ^C
leaves its report (and its last checkpoint). It asks, before each state or
walk, whether to stop; and a signal that comes while the next is being made
(interrupted), however long that takes, ends it there."""
from __future__ import annotations

import contextlib
import signal
from collections.abc import Callable, Iterable, Iterator
from typing import TypeVar

SIGNALS = (signal.SIGINT, signal.SIGTERM)

T = TypeVar("T")

# The name of the first of SIGNALS received inside on_signals, or None
# (outside it too); and
# whether one received now raises Stopped where the program is
# (interrupted). Signal handlers are the process's: so are these.
_received: str | None = None
_interruptible = False


class Stopped(BaseException):
    """What the first signal raises while the next item is being made
    (interrupted). Not an Exception, so that nothing that catches the
    errors of a step catches it."""


def never() -> None:
    """What a walk asks whether to stop when nothing stops it."""
    return None


@contextlib.contextmanager
def on_signals() -> Iterator[Callable[[], str | None]]:
    """What the walk asks whether to stop (Explorer.run's stop): the name of
    the first of SIGNALS received while inside, or None; those after it are
    ignored, so that the report and the checkpoint are written whole. The
    handlers before are put back on leaving."""
    global _received
    received: list[str] = []

    def handle(signum, _frame):
        global _received
        if not received:
            received.append(signal.Signals(signum).name)
            _received = received[0]
            if _interruptible:
                raise Stopped(_received)

    before = {s: signal.signal(s, handle) for s in SIGNALS}
    try:
        yield lambda: received[0] if received else None
    finally:
        for s, h in before.items():
            signal.signal(s, h)
        _received = None


def interrupted(items: Iterable[T]) -> Iterator[T]:
    """items, ending at the first signal of on_signals: one that comes while
    the next item is being made ends them there, the item left unmade, and
    one that comes while the item given is being used ends them before the
    next."""
    global _interruptible
    it = iter(items)
    while _received is None:
        try:
            _interruptible = True
            try:
                x = next(it)
            finally:
                _interruptible = False
        except (StopIteration, Stopped):
            return
        yield x

