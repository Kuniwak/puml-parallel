"""How far csdfbmc's walk got, said on stderr every so many seconds
(--progress): the states it met, the frontier, the depth, the states per
second, the field values tried and the valuations checked since the
last line, the time spent, and the memory the walk holds, in all and per
state, on macOS and Linux.
The values tried and checked tell a walk stuck enumerating a transition
from one whose states grow: the first tries many while it meets few states.
"""
from __future__ import annotations

import contextlib
import ctypes
import os
import sys
import threading
import time
from collections.abc import Callable, Iterator
from typing import TextIO


def resident() -> int | None:
    """The bytes of memory this process holds (its resident set size), or
    None where it cannot be read: on macOS and Linux only."""
    try:
        if sys.platform == "linux":
            with open("/proc/self/statm") as f:
                return int(f.read().split()[1]) * os.sysconf("SC_PAGE_SIZE")
        if sys.platform == "darwin":
            return _mach_resident()
    except (OSError, ValueError, AttributeError):
        pass
    return None


class _MachTaskBasicInfo(ctypes.Structure):
    _fields_ = [("virtual_size", ctypes.c_uint64), ("resident_size", ctypes.c_uint64),
                ("resident_size_max", ctypes.c_uint64), ("user_time", ctypes.c_int32 * 2),
                ("system_time", ctypes.c_int32 * 2), ("policy", ctypes.c_int32), ("suspend_count", ctypes.c_int32)]


# task_info's flavor for a _MachTaskBasicInfo.
_MACH_TASK_BASIC_INFO = 20


def _mach_resident() -> int | None:
    libc = ctypes.CDLL(None)
    info = _MachTaskBasicInfo()
    count = ctypes.c_uint32(ctypes.sizeof(info) // 4)
    task = ctypes.c_uint32.in_dll(libc, "mach_task_self_")
    if libc.task_info(task, _MACH_TASK_BASIC_INFO, ctypes.byref(info), ctypes.byref(count)) != 0:
        return None
    return info.resident_size


def unsaid(**counts: int) -> None:
    """Says nothing of how far a walk got."""


def _hms(seconds: float) -> str:
    s = int(seconds)
    return f"{s // 3600}:{s // 60 % 60:02}:{s % 60:02}"


class _Line:
    """The counts last given (update), said in a line by report every so
    often. A report reads them as they are, so that a walk stuck is said to
    be. The counts are kept as one tuple, so that a report never mixes two
    updates'."""

    def __init__(self, out: TextIO, counts: tuple[int, ...], clock: Callable[[], float] = time.monotonic,
                 resident: Callable[[], int | None] = resident):
        self.out, self.clock, self.resident = out, clock, resident
        self.started = self.last_time = clock()
        self.counts = self.last = counts

    def report(self) -> None:
        now, counts = self.clock(), self.counts
        span = now - self.last_time
        print(f"csdfbmc: {self.said(counts, self.last, span)}, {_hms(now - self.started)} elapsed{self.memory(self.resident(), counts)}",
              file=self.out, flush=True)
        self.last_time, self.last = now, counts

    def said(self, counts: tuple[int, ...], last: tuple[int, ...], span: float) -> str:
        raise NotImplementedError

    def memory(self, held: int | None, counts: tuple[int, ...]) -> str:
        return f", {held / 1e9:.1f} GB resident" if held is not None else ""


def _rate(now: int, last: int, span: float) -> float:
    return (now - last) / span if span > 0 else 0.0


class Progress(_Line):
    """How far a breadth-first walk got: the states it met, the frontier,
    the depth, the states per second, the values tried and checked since
    the last line, and the memory in all and per state."""

    def __init__(self, out: TextIO, clock: Callable[[], float] = time.monotonic,
                 resident: Callable[[], int | None] = resident):
        super().__init__(out, (0, 0, 0, 0, 0), clock, resident)

    def update(self, visited: int, frontier: int, depth: int, tried: int, checked: int) -> None:
        self.counts = (visited, frontier, depth, tried, checked)

    def said(self, counts, last, span):
        visited, frontier, depth, tried, checked = counts
        return (f"{visited} states, frontier {frontier}, depth {depth}, {_rate(visited, last[0], span):.1f} states/s, "
                f"{tried - last[3]} field values tried and {checked - last[4]} valuations checked in the last {span:.0f}s")

    def memory(self, held, counts):
        if held is None:
            return ""
        return f"{super().memory(held, counts)}, {held / max(counts[0], 1) / 1e3:.1f} KB/state"


class WalkProgress(_Line):
    """How far random walks (--random-walk) got: the walks taken in, their
    steps, the deepest, the walks per second, the findings and the memory
    in all."""

    def __init__(self, out: TextIO, clock: Callable[[], float] = time.monotonic,
                 resident: Callable[[], int | None] = resident):
        super().__init__(out, (0, 0, 0, 0), clock, resident)

    def update(self, walks: int, steps: int, deepest: int, findings: int) -> None:
        self.counts = (walks, steps, deepest, findings)

    def said(self, counts, last, span):
        walks, steps, deepest, findings = counts
        return f"{walks} walks, {steps} steps, deepest {deepest}, {_rate(walks, last[0], span):.1f} walks/s, {findings} findings"


@contextlib.contextmanager
def reporting(out: TextIO, seconds: float, make: Callable[[TextIO], _Line]) -> Iterator[_Line]:
    """A progress line made by make (Progress or WalkProgress) said on out every
    seconds by a thread of its own, which stops on leaving; never said for
    0 seconds."""
    p, stop = make(out), threading.Event()
    if seconds == 0:
        yield p
        return

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
