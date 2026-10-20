"""python -m csdfbmc <diagram.json> <twins.py> [--local PATH=TWINS.py]... [--evaluator CMD | --evaluator-module MODULE] [--max-states N] [--opaque error|true] [--avoid EVENT] [--first] [--all] [--por] [--symmetry] [--json]

Reads a diagram as csdfparse prints it, or an expansion as csdfpromote -format
json prints it, and the twins of its clauses, walks the reachable states within
the bound, and reports the deadlocks, the tau cycles, the broken invariants
and, with --avoid, the cycles that never perform the event. Findings of one
shape are reported once with their count and the shortest trace; --all lists
every one, and --first stops the walk at the first deadlock, broken invariant
or unconstrained post. For an expansion, --local names the twins of each
family's local diagram by the path the expansion uses. --evaluator and
--evaluator-module read every clause through a glue of the evaluator
protocol, in place of twins (see csdfbmc.protocol). --por and --symmetry
reduce the states walked without changing what is found. Exits 1 when it finds
any, 2 on a usage or loading error.
"""
import argparse
import json
import sys

from csdfbmc.cli import LOADING_ERRORS, add_inputs, opened
from csdfbmc.explore import Explorer


def print_groups(result) -> None:
    for g in result.groups():
        n = f" ×{g['count']}" if g["count"] > 1 else ""
        first = f"; first after {g['trace']}" if g["count"] > 1 else f" after {g['trace']}"
        if g["kind"] == "deadlock":
            if g["count"] == 1:
                print(f"deadlock at {g['example']} after {g['trace']}")
            else:
                print(f"deadlock{n} at {g['at']} after {g['after']}{first}")
        elif g["kind"] == "violation":
            if g["count"] == 1:
                print(f"invariant {g['invariant']!r} broken at {g['example']} after {g['trace']}")
            else:
                print(f"invariant {g['invariant']!r} broken{n} at {g['at']}{first}")
        elif g["kind"] == "tau_cycle":
            print(f"tau cycle{n} through {g['through']}; reached after {g['trace']}")
        elif g["kind"] == "avoiding_cycle":
            print(f"cycle without {g['avoid']}{n} through {g['through']}; reached after {g['trace']}")
        elif g["kind"] == "unconstrained":
            print(f"post leaves {g['variables']} without a value{n}: {g['event']} at {g['at']}{first}")
        elif g["kind"] == "empty_post":
            print(f"post admits no successor{n}: {g['event']} at {g['at']}{first}")


def print_every(result) -> None:
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


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(prog="csdfbmc", description=__doc__.split("\n\n")[1])
    add_inputs(ap)
    ap.add_argument("--max-states", type=int, default=100_000, help="stop after this many states (default 100000)")
    ap.add_argument("--opaque", choices=["error", "true"], default="error", help="what a clause without a twin means (default: refuse)")
    ap.add_argument("--avoid", metavar="EVENT", help="also report the cycles that never perform EVENT (a livelock under a fairness bound)")
    ap.add_argument("--within", metavar="STATE", action="append", help="with --avoid, look for the cycles among these control states only (repeatable)")
    ap.add_argument("--first", action="store_true", help="stop at the first deadlock, broken invariant or unconstrained post (cycles are then not looked for)")
    ap.add_argument("--all", action="store_true", help="list every finding instead of one per shape with a count")
    ap.add_argument("--por", action="store_true", help="take the moves of one independent instance at a time (partial-order reduction)")
    ap.add_argument("--symmetry", action="store_true", help="store one state per renaming of the IDs the twins declare SYMMETRIC")
    ap.add_argument("--json", action="store_true", help="print the result as JSON")
    a = ap.parse_args(argv)

    try:
        with opened(a) as (diagram, twins, meaning):
            result = Explorer(diagram, twins, opaque=a.opaque, max_states=a.max_states, avoid=a.avoid, within=a.within, first=a.first,
                              por=a.por, symmetry=a.symmetry, meaning=meaning).run()
    except LOADING_ERRORS as e:
        print(f"csdfbmc: {e}", file=sys.stderr)
        return 2

    if a.json:
        json.dump(result.to_json(), sys.stdout, ensure_ascii=False)
        print()
    else:
        head = f"states: {len(result.states)}"
        if result.truncated:
            head += " (truncated at the bound)"
        if result.stopped_early:
            head += " (stopped at the first finding)"
        print(head)
        if a.all:
            print_every(result)
        else:
            print_groups(result)
        for c in result.opaque_clauses:
            print(f"taken as true (no twin): {c}")
        for text, maps in result.dropped_clauses:
            print(f"taken as true (reads {', '.join(maps)}, left out by csdfpromote -only): {text}")
    found = result.deadlocks or result.tau_cycles or result.violations or result.avoiding_cycles or result.unconstrained
    return 1 if found else 0


if __name__ == "__main__":
    sys.exit(main())
