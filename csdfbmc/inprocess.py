"""csdfbmc's commands run in this interpreter rather than as a process, for
a host that starts no process: a web page running Python in WebAssembly
(csdfreplweb -checks). run(["next", ...]) is what "csdfbmc next ..." exits
with and prints on its standard output and standard error, over the
inputs a Sessions keeps open.

A Sessions keeps the inputs the last call opened for the calls after it
that name them alike: the same arguments, and the same size and time of
change of each file among the diagram, the checks and the words of
--evaluator-module. It opens them again when any of these differ; a file
those files read in turn is not looked at, and close() is how a host
drops what is kept."""
from __future__ import annotations

import argparse
import io
import os
import shlex
from collections.abc import Callable, Iterator
from contextlib import AbstractContextManager, ExitStack, contextmanager, redirect_stderr, redirect_stdout

# The commands run takes, by the word that names them.
COMMANDS = ("next",)


class Sessions:
    """The Explorer the last call opened, kept for the calls after it that
    name the same inputs, unchanged on disk; opener opens one."""

    def __init__(self, opener: Callable[[argparse.Namespace], AbstractContextManager]):
        self._opener = opener
        self._key: tuple | None = None
        self._explorer = None
        self._stack = ExitStack()

    def __call__(self, a: argparse.Namespace) -> AbstractContextManager:
        return self._session(a)

    @contextmanager
    def _session(self, a: argparse.Namespace) -> Iterator:
        key = _key(a)
        if key != self._key:
            self.close()
            stack = ExitStack()
            self._explorer = stack.enter_context(self._opener(a))
            self._stack, self._key = stack, key
        yield self._explorer

    def close(self) -> None:
        """Close the Explorer kept, and the glue started for it."""
        self._key, self._explorer = None, None
        self._stack.close()


def _key(a: argparse.Namespace) -> tuple:
    """What the inputs a names are: the arguments that name them, and the
    size and time of change of each file among them."""
    words = [a.diagram, a.checks] + (shlex.split(a.evaluator_module) if a.evaluator_module else [])
    files = tuple((w, os.stat(w).st_mtime_ns, os.stat(w).st_size) for w in words if os.path.isfile(w))
    return (a.diagram, a.checks, a.evaluator, a.evaluator_module, tuple(a.env), files)


def run(args: list[str], sessions: Sessions) -> list:
    """The exit code, standard output and standard error of the command
    args name, run in this interpreter over the inputs sessions keeps."""
    out, err = io.StringIO(), io.StringIO()
    with redirect_stdout(out), redirect_stderr(err):
        if not args or args[0] not in COMMANDS:
            print(f"csdfbmc in-process runs only {', '.join(COMMANDS)}, not {args[:1]!r}", file=err)
            code = 2
        else:
            from csdfbmc.next import main
            code = main(list(args[1:]), sessions)
    return [code, out.getvalue(), err.getvalue()]
