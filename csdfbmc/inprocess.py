"""csdfbmc's commands run in this interpreter rather than as a process, for
a host that starts no process: a web page running Python in WebAssembly
(csdfreplweb -checks). run(["next", ...]) is what "csdfbmc next ..." exits
with and prints on its standard output and standard error."""
from __future__ import annotations

import io
from contextlib import redirect_stderr, redirect_stdout

# The commands run takes, by the word that names them.
COMMANDS = ("next",)


def run(args: list[str]) -> list:
    """The exit code, standard output and standard error of the command
    args name, run in this interpreter."""
    out, err = io.StringIO(), io.StringIO()
    with redirect_stdout(out), redirect_stderr(err):
        if not args or args[0] not in COMMANDS:
            print(f"csdfbmc in-process runs only {', '.join(COMMANDS)}, not {args[:1]!r}", file=err)
            code = 2
        else:
            from csdfbmc.next import main
            code = main(list(args[1:]))
    return [code, out.getvalue(), err.getvalue()]
