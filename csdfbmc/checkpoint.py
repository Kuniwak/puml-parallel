"""csdfbmc's walk written to a file as it goes (--checkpoint) and taken up
again from it (--resume).

The walk is breadth first and expanding a state reads nothing of the states
it met, so a walk taken up after its k-th state walks the rest as the whole
walk would have, and says the same. A checkpoint is written after a state is
expanded, to a file beside the one named, then put in its place, so that a
walk stopped while writing leaves the one before.

A checkpoint names the inputs it was written for (fingerprint): the diagram,
the checks module and the options that change the walk; another is refused.
It cannot tell a glue that reads atoms otherwise. A checkpoint is a pickle,
which runs code as it is read: take up only those you wrote.
"""
from __future__ import annotations

import argparse
import dataclasses
import hashlib
import json
import os
import pickle
import time
import traceback
import warnings
from collections.abc import Callable

from csdfbmc.explore import Walk
from csdfbmc.model import Edge

# What a checkpoint starts with; one written by another layout is refused.
MAGIC = b"csdfbmc-checkpoint 1\n"

# The options that change what the walk meets or says.
WALK_OPTIONS = ("env", "evaluator", "evaluator_module", "max_states", "avoid", "within", "first", "por", "symmetry", "cone", "stats")


class CheckpointError(ValueError):
    pass


def fingerprint(a: argparse.Namespace) -> str:
    """The inputs a walk of the arguments a reads, as one digest."""
    h = hashlib.sha256()
    for path in (a.diagram, a.checks):
        with open(path, "rb") as f:
            h.update(hashlib.sha256(f.read()).digest())
    h.update(json.dumps({k: getattr(a, k, None) for k in WALK_OPTIONS}, sort_keys=True).encode())
    return h.hexdigest()


def dumps(w: Walk, edges: list[Edge]) -> bytes:
    """w as bytes, each edge by its index in edges."""
    index = {id(e): i for i, e in enumerate(edges)}
    parents = {s: p and (p[0], p[1], index[id(p[2])]) for s, p in w.parents.items()}
    return pickle.dumps(dataclasses.replace(w, parents=parents), protocol=pickle.HIGHEST_PROTOCOL)


def loads(data: bytes, edges: list[Edge]) -> Walk:
    """The walk dumps gave data of, each edge out of edges."""
    w = pickle.loads(data)
    w.parents = {s: p and (p[0], p[1], edges[p[2]]) for s, p in w.parents.items()}
    return w


def save(path: str, digest: str, data: bytes) -> None:
    tmp = path + ".tmp"
    with open(tmp, "wb") as f:
        f.write(MAGIC + digest.encode() + b"\n" + data)
    os.replace(tmp, path)


def load(path: str, digest: str) -> bytes:
    """The walk saved in path, which must have been written for digest."""
    with open(path, "rb") as f:
        head = f.read(len(MAGIC))
        if head != MAGIC:
            raise CheckpointError(f"{path} is not a checkpoint csdfbmc can take up")
        if f.readline().rstrip(b"\n").decode() != digest:
            raise CheckpointError(f"{path} was written for other inputs: the diagram, the checks module or the options that change the walk differ")
        return f.read()


class Every:
    """Writes the walk it is given (Explorer.run's checkpoint) with write,
    at most every seconds; flush writes the last given, once the write
    going on is done.

    With fork, a write runs in a child forked for it, which shares the
    walk's memory until the walk changes it: the walk goes on at once
    instead of waiting for a write of all it met. A write due while the
    last is going on waits for the next state."""

    def __init__(self, seconds: float, write: Callable[[Walk], None], clock: Callable[[], float] = time.monotonic,
                 fork: bool = hasattr(os, "fork")):
        self.seconds, self.write, self.clock, self.fork = seconds, write, clock, fork
        self.last = clock()
        self.walk: Walk | None = None
        self.child: int | None = None

    def __call__(self, w: Walk) -> None:
        self.walk = w
        now = self.clock()
        if now - self.last < self.seconds or self._writing():
            return
        if self.fork:
            self._fork(w)
        else:
            self.write(w)
        self.last = now

    def flush(self) -> None:
        self._wait()
        if self.walk is not None:
            self.write(self.walk)

    def _fork(self, w: Walk) -> None:
        with warnings.catch_warnings():
            # The child only writes w: it takes no lock another thread holds.
            warnings.simplefilter("ignore", DeprecationWarning)
            pid = os.fork()
        if pid == 0:
            code = 0
            try:
                self.write(w)
            except BaseException:  # noqa: BLE001 - said, and the walk goes on
                traceback.print_exc()
                code = 1
            finally:
                os._exit(code)
        self.child = pid

    def _writing(self) -> bool:
        if self.child is None:
            return False
        pid, _ = os.waitpid(self.child, os.WNOHANG)
        if pid == 0:
            return True
        self.child = None
        return False

    def _wait(self) -> None:
        if self.child is not None:
            os.waitpid(self.child, 0)
            self.child = None
