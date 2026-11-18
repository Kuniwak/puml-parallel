"""A count that processes add to and read: made before the workers and given
to them as they start (multiprocessing.Pool's initargs)."""
from __future__ import annotations

import multiprocessing


class SharedCounter:
    def __init__(self):
        self._value = multiprocessing.get_context("spawn").Value("q", 0)

    def add(self) -> None:
        """Adds one."""
        with self._value.get_lock():
            self._value.value += 1

    def __call__(self) -> int:
        return self._value.value
