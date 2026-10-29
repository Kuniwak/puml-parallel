"""How far csdfbmc's walk got, said on stderr every so many seconds
(--progress): the states it met, the frontier, the depth, the states per
second and the field values tried since the last line, and the time spent.
The field values tried tell a walk stuck enumerating a transition from one
whose states grow: the first tries many while it meets few states.
"""
from __future__ import annotations

import contextlib
import threading
import time
from collections.abc import Callable, Iterator
from typing import TextIO


class Progress:
    """The counts the walk last gave (update), said by report. A report
    reads them as they are, so that a walk stuck in one state is said to
    be."""

    def __init__(self, out: TextIO, clock: Callable[[], float] = time.monotonic):
        self.out, self.clock = out, clock
        self.started = self.last_time = clock()
        self.visited = self.frontier = self.depth = self.tried = 0
        self.last_visited = self.last_tried = 0

    def update(self, visited: int, frontier: int, depth: int, tried: int) -> None:
        self.visited, self.frontier, self.depth, self.tried = visited, frontier, depth, tried

    def report(self) -> None:
        now = self.clock()
        visited, tried = self.visited, self.tried
        span = now - self.last_time
        rate = (visited - self.last_visited) / span if span > 0 else 0.0
        elapsed = int(now - self.started)
        print(f"csdfbmc: {visited} states, frontier {self.frontier}, depth {self.depth}, {rate:.1f} states/s, "
              f"{tried - self.last_tried} field values tried in the last {span:.0f}s, "
              f"{elapsed // 3600}:{elapsed // 60 % 60:02}:{elapsed % 60:02} elapsed", file=self.out, flush=True)
        self.last_time, self.last_visited, self.last_tried = now, visited, tried


@contextlib.contextmanager
def reporting(out: TextIO, seconds: float) -> Iterator[Progress | None]:
    """A Progress said on out every seconds by a thread of its own, which
    stops on leaving; None for 0 seconds."""
    if seconds == 0:
        yield None
        return
    p, stop = Progress(out), threading.Event()

    def tick():
        while not stop.wait(seconds):
            p.report()

    t = threading.Thread(target=tick, daemon=True)
    t.start()
    try:
        yield p
    finally:
        stop.set()
        t.join()
