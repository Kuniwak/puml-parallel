"""python -m csdfbmc.serve MODULE [ARGS...]

Runs the glue MODULE's evaluator(ARGS) gives, the one --evaluator-module
loads in-process, as a command of the evaluator protocol: one JSON request
per line on standard input, one JSON answer per line on standard output
(csdfbmc.protocol). A tool that cannot load Python in-process, such as
csdflint -evaluator-module, runs a module glue so.
"""
import sys

from csdfbmc.protocol import module_handle, serve


def main(argv: list[str], stdin, stdout) -> None:
    if not argv:
        raise SystemExit("usage: python -m csdfbmc.serve MODULE [ARGS...]")
    serve(module_handle(argv[0], argv[1:]), stdin, stdout)


if __name__ == "__main__":
    main(sys.argv[1:], sys.stdin, sys.stdout)
