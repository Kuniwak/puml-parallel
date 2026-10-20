"""python -m csdfbmc <diagram.json> <twins.py> [--local PATH=TWINS.py]... [--max-states N] [--opaque error|true] [--avoid EVENT] [--json]

Reads a diagram as csdfparse prints it, or an expansion as csdfpromote -format
json prints it, and the twins of its clauses, walks the reachable states within
the bound, and reports the deadlocks, the tau cycles, the broken invariants
and, with --avoid, the cycles that never perform the event. For an expansion,
--local names the twins of each family's local diagram by the path the
expansion uses. Exits 1 when it finds any, 2 on a usage or loading error.
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
    ap.add_argument("--local", metavar="PATH=TWINS.py", action="append", default=[],
                    help="the twins of the local diagram at PATH, as the expansion names it (repeatable)")
    ap.add_argument("--max-states", type=int, default=100_000, help="stop after this many states (default 100000)")
    ap.add_argument("--opaque", choices=["error", "true"], default="error", help="what a clause without a twin means (default: refuse)")
    ap.add_argument("--avoid", metavar="EVENT", help="also report the cycles that never perform EVENT (a livelock under a fairness bound)")
    ap.add_argument("--within", metavar="STATE", action="append", help="with --avoid, look for the cycles among these control states only (repeatable)")
    ap.add_argument("--json", action="store_true", help="print the result as JSON")
    a = ap.parse_args(argv)

    try:
        diagram = Diagram.load(a.diagram)
        twins = load_twins(a.twins)
        for spec in a.local:
            path, sep, module = spec.partition("=")
            if not sep or not path or not module:
                raise ValueError(f"--local expects PATH=TWINS.py, got {spec!r}")
            twins.locals[path] = load_twins(module)
        result = Explorer(diagram, twins, opaque=a.opaque, max_states=a.max_states, avoid=a.avoid, within=a.within).run()
    except (OSError, ValueError, KeyError, PurityError) as e:
        print(f"csdfbmc: {e}", file=sys.stderr)
        return 2

    if a.json:
        json.dump(result.to_json(), sys.stdout, ensure_ascii=False)
        print()
    else:
        print(f"states: {len(result.states)}" + (" (truncated at the bound)" if result.truncated else ""))
        for s in result.deadlocks:
            print(f"deadlock at {result.describe(s)} after {result.trace_to(s)}")
        for cycle in result.tau_cycles:
            print(f"tau cycle through {[c[0] for c in cycle]} reached after {result.trace_to(cycle[0])}")
        for name, s in result.violations:
            print(f"invariant {name!r} broken at {result.describe(s)} after {result.trace_to(s)}")
        for cycle in result.avoiding_cycles:
            print(f"cycle without {result.avoid} through {[c[0] for c in cycle]} reached after {result.trace_to(cycle[0])}")
        for s, label, names in result.unconstrained:
            print(f"post leaves {list(names)} without a value: {label} at {result.describe(s)}, after {result.trace_to(s)}")
        for s, label in result.empty_posts:
            print(f"post admits no successor: {label} at {result.describe(s)}, after {result.trace_to(s)}")
        for c in result.opaque_clauses:
            print(f"taken as true (no twin): {c}")
    found = result.deadlocks or result.tau_cycles or result.violations or result.avoiding_cycles or result.unconstrained
    return 1 if found else 0


if __name__ == "__main__":
    sys.exit(main())
