"""The reference glue (csdfbmc.python_glue) made slow for the tests: at
each guard it first says it is there, a line with its process ID appended
to the file args[0] names, and then takes far longer than a test may wait,
as a heavy step does, so that a test signals csdfbmc in the middle of a
step. args[1:] are the reference glue's."""
from __future__ import annotations

import os
import time
from typing import Callable

from csdfbmc import python_glue


def evaluator(args: list[str]) -> Callable[[dict], dict]:
    marker, inner = args[0], python_glue.evaluator(args[1:])

    def handle(request: dict) -> dict:
        if request.get("op") == "guard":
            with open(marker, "a", encoding="utf-8") as f:
                f.write(f"{os.getpid()}\n")
            time.sleep(600)
        return inner(request)

    return handle


if __name__ == "__main__":
    import sys

    from csdfbmc.protocol import serve
    serve(evaluator(sys.argv[1:]), sys.stdin, sys.stdout)
