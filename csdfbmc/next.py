"""python -m csdfbmc.next <diagram.json> <twins.py> [--local PATH=TWINS.py]... (--initial | --state JSON --edge N)

What a REPL asks the checker at a value prompt: with --initial, the values of
the start state the twins admit; with --state and --edge, the values after
the N-th outgoing edge (in the diagram's order) of the given state, per
parameter valuation. The state is a runtime state as csdfrepl and csdfreplcmd
print it ({"state_id": ..., "values": [{"name": ..., "value": ...}, ...]}),
or just the array of values in declaration order. Prints
{"candidates": [{"params": {...}, "values": [...]}, ...]} and exits 0; exits
2 when the inputs cannot be read.
"""
import argparse
import json
import sys

from csdfbmc.explore import Explorer
from csdfbmc.model import Diagram
from csdfbmc.twins import load_twins, PurityError


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(prog="csdfbmc next", description=__doc__.split("\n\n")[1])
    ap.add_argument("diagram")
    ap.add_argument("twins")
    ap.add_argument("--local", metavar="PATH=TWINS.py", action="append", default=[])
    ap.add_argument("--initial", action="store_true", help="the values of the start state")
    ap.add_argument("--state", metavar="JSON", help="the current state, as a runtime state or an array of values")
    ap.add_argument("--edge", metavar="N", type=int, help="the outgoing edge taken, numbered from 0 in the diagram's order")
    a = ap.parse_args(argv)
    if a.initial == (a.state is not None or a.edge is not None) or (a.state is None) != (a.edge is None):
        ap.error("give --initial, or both --state and --edge")

    try:
        diagram = Diagram.load(a.diagram)
        twins = load_twins(a.twins)
        for spec in a.local:
            path, sep, module = spec.partition("=")
            if not sep or not path or not module:
                raise ValueError(f"--local expects PATH=TWINS.py, got {spec!r}")
            twins.locals[path] = load_twins(module)
        explorer = Explorer(diagram, twins)
        if a.initial:
            candidates = [{"params": {}, "values": v} for v in explorer.initial_candidates()]
        else:
            state = json.loads(a.state)
            if isinstance(state, dict):
                state_id = state["state_id"]
                values = [v["value"] for v in state.get("values", [])]
            else:
                raise ValueError("--state must be a runtime state object with state_id and values")
            candidates = explorer.candidates(state_id, values, a.edge)
    except (OSError, ValueError, KeyError, IndexError, TypeError, PurityError) as e:
        print(f"csdfbmc next: {e}", file=sys.stderr)
        return 2
    json.dump({"candidates": candidates}, sys.stdout, ensure_ascii=False)
    print()
    return 0


if __name__ == "__main__":
    sys.exit(main())
