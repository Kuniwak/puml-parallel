"""csdfbmc stopped by a signal (SIGINT, SIGTERM): the walk asks, before
each state or walk, whether to stop, and ends where it is with what it
found so far, so that a run cut short by timeout or ^C leaves its report
(and its last checkpoint)."""
from __future__ import annotations

import contextlib
import signal
from collections.abc import Callable, Iterator

SIGNALS = (signal.SIGINT, signal.SIGTERM)


@contextlib.contextmanager
def on_signals() -> Iterator[Callable[[], str | None]]:
    """What the walk asks whether to stop (Explorer.run's stop): the name of
    the first of SIGNALS received while inside, or None. The handlers before
    are put back on leaving."""
    received: list[str] = []

    def handle(signum, _frame):
        if not received:
            received.append(signal.Signals(signum).name)

    before = {s: signal.signal(s, handle) for s in SIGNALS}
    try:
        yield lambda: received[0] if received else None
    finally:
        for s, h in before.items():
            signal.signal(s, h)
