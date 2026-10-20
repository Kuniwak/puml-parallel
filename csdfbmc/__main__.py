"""python -m csdfbmc <diagram.json> <twins.py> [--max-states N] [--opaque error|true] [--json]

Reads a diagram as csdfparse prints it and the twins of its clauses, walks the
reachable states within the bound, and reports the deadlocks and tau cycles it
finds. Exits 1 when it finds any, 2 on a usage or loading error.
"""
import argparse
import json
import sys

from csdfbmc.explore import Explorer
from csdfbmc.model import Diagram
from csdfbmc.twins import load_twins, PurityError


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(prog="csdfbmc", description=__doc__.split("\n\n")[1])
    ap.add_argument("diagram", help="the diagram as JSON (csdfparse's output)")
    ap.add_argument("twins", help="the Python module holding DOMAINS and CLAUSES")
    ap.add_argument("--max-states", type=int, default=100_000, help="stop after this many states (default 100000)")
    ap.add_argument("--opaque", choices=["error", "true"], default="error", help="what a clause without a twin means (default: refuse)")
    ap.add_argument("--json", action="store_true", help="print the result as JSON")
    a = ap.parse_args(argv)

    try:
        diagram = Diagram.load(a.diagram)
        twins = load_twins(a.twins)
        result = Explorer(diagram, twins, opaque=a.opaque, max_states=a.max_states).run()
    except (OSError, ValueError, KeyError, PurityError) as e:
        print(f"csdfbmc: {e}", file=sys.stderr)
        return 2

    if a.json:
        json.dump(result.to_json(), sys.stdout, ensure_ascii=False)
        print()
    else:
        print(f"states: {len(result.states)}" + (" (truncated at the bound)" if result.truncated else ""))
        for s in result.deadlocks:
            print(f"deadlock at {s[0]} {list(s[1])} after {result.trace_to(s)}")
        for cycle in result.tau_cycles:
            print(f"tau cycle through {[c[0] for c in cycle]} reached after {result.trace_to(cycle[0])}")
        for c in result.opaque_clauses:
            print(f"taken as true (no twin): {c}")
    return 1 if result.deadlocks or result.tau_cycles else 0


if __name__ == "__main__":
    sys.exit(main())
