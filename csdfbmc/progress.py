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


class Progress:
    """The counts the walk last gave (update), said by report. A report
    reads them as they are, so that a walk stuck in one state is said to
    be."""

    def __init__(self, out: TextIO, clock: Callable[[], float] = time.monotonic,
                 resident: Callable[[], int | None] = resident):
        self.out, self.clock, self.resident = out, clock, resident
        self.started = self.last_time = clock()
        self.visited = self.frontier = self.depth = self.tried = self.checked = 0
        self.last_visited = self.last_tried = self.last_checked = 0

    def update(self, visited: int, frontier: int, depth: int, tried: int, checked: int) -> None:
        self.visited, self.frontier, self.depth, self.tried, self.checked = visited, frontier, depth, tried, checked

    def report(self) -> None:
        now = self.clock()
        visited, tried, checked = self.visited, self.tried, self.checked
        span = now - self.last_time
        rate = (visited - self.last_visited) / span if span > 0 else 0.0
        elapsed = int(now - self.started)
        held = self.resident()
        memory = f", {held / 1e9:.1f} GB resident, {held / max(visited, 1) / 1e3:.1f} KB/state" if held is not None else ""
        print(f"csdfbmc: {visited} states, frontier {self.frontier}, depth {self.depth}, {rate:.1f} states/s, "
              f"{tried - self.last_tried} field values tried and {checked - self.last_checked} valuations checked in the last {span:.0f}s, "
              f"{elapsed // 3600}:{elapsed // 60 % 60:02}:{elapsed % 60:02} elapsed{memory}", file=self.out, flush=True)
        self.last_time, self.last_visited, self.last_tried, self.last_checked = now, visited, tried, checked


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
