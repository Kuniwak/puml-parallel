"""python -m csdfbmc.next <diagram.json> <checks.py> (--evaluator CMD | --evaluator-module MODULE) (--initial | --state JSON --edge N)

What a REPL asks the checker at a value prompt: with --initial, the values of
the start state the clauses admit; with --state and --edge, the values after
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

from csdfbmc.cli import LOADING_ERRORS, add_inputs, opened
from csdfbmc.explore import Explorer, as_met, every_transition


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(prog="csdfbmc next", description=__doc__.split("\n\n")[1])
    add_inputs(ap)
    ap.add_argument("--initial", action="store_true", help="the values of the start state")
    ap.add_argument("--state", metavar="JSON", help="the current state, as a runtime state or an array of values")
    ap.add_argument("--edge", metavar="N", type=int, help="the outgoing edge taken, numbered from 0 in the diagram's order")
    a = ap.parse_args(argv)
    if a.initial == (a.state is not None or a.edge is not None) or (a.state is None) != (a.edge is None):
        ap.error("give --initial, or both --state and --edge")

    try:
        with opened(a) as (diagram, checks, client, environment):
            candidates = _candidates(Explorer(diagram, checks, client, environment=environment, left_out=(), ample=every_transition,
                                                     canonical=as_met), a)
    except (*LOADING_ERRORS, IndexError, TypeError) as e:
        print(f"csdfbmc next: {e}", file=sys.stderr)
        return 2
    json.dump({"candidates": candidates}, sys.stdout, ensure_ascii=False)
    print()
    return 0


def _candidates(explorer: Explorer, a) -> list:
    if a.initial:
        return [{"params": {}, "values": v} for v in explorer.initial_candidates()]
    state = json.loads(a.state)
    if not isinstance(state, dict):
        raise ValueError("--state must be a runtime state object with state_id and values")
    return explorer.candidates(state["state_id"], [v["value"] for v in state.get("values", [])], a.edge)


if __name__ == "__main__":
    sys.exit(main())
