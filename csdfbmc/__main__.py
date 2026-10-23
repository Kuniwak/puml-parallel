"""python -m csdfbmc <diagram.json> <checks.py> (--evaluator CMD | --evaluator-module MODULE) [--max-states N] [--seed N] [--env NAME=true|false] [--avoid EVENT] [--first] [--all] [--json]

Reads a diagram as csdfparse prints it (or csdfpromote -format json), its
clauses as formulas: the builtins and ∃ are the checker's, the atoms the
checks module declares are functions of the environment, and every other
atom is read through a glue of the evaluator protocol (see csdfbmc.protocol;
csdfbmc.python_glue reads Python). The checks module gives the types, the
environment, the ghosts and the invariants. It walks the reachable states
within the bound, and reports the deadlocks, the tau cycles, the broken
invariants and, with --avoid, the cycles that never perform the event.
A state with no way out that one more value of each type given as a sample
(csdfbmc.values.sample) would give one is reported as the bound reached,
not as a deadlock, and is no finding.
Findings of one shape are reported once with their count and the shortest
trace; --all lists every one, and --first stops the walk at the first
deadlock or broken invariant. Exits 1 when it finds any, 2 on a usage or
loading error.
"""
import argparse
import json
import sys

from csdfbmc.cli import LOADING_ERRORS, add_inputs, opened
from csdfbmc.explore import Explorer
from csdfbmc.independence import check_independent


# Why a state where the bound was reached is no deadlock.
MORE = " (a sample type has more values)"


def print_groups(result) -> None:
    for g in result.groups():
        n = f" ×{g['count']}" if g["count"] > 1 else ""
        first = f"; first after {g['trace']}" if g["count"] > 1 else f" after {g['trace']}"
        if g["kind"] == "deadlock":
            if g["count"] == 1:
                print(f"deadlock at {g['example']} after {g['trace']}")
            else:
                print(f"deadlock{n} at {g['at']} after {g['after']}{first}")
        elif g["kind"] == "bound_reached":
            if g["count"] == 1:
                print(f"bound reached at {g['example']} after {g['trace']}{MORE}")
            else:
                print(f"bound reached{n} at {g['at']} after {g['after']}{first}{MORE}")
        elif g["kind"] == "violation":
            if g["count"] == 1:
                print(f"invariant {g['invariant']!r} broken at {g['example']} after {g['trace']}")
            else:
                print(f"invariant {g['invariant']!r} broken{n} at {g['at']}{first}")
        elif g["kind"] == "tau_cycle":
            print(f"tau cycle{n} through {g['through']}; reached after {g['trace']}")
        elif g["kind"] == "avoiding_cycle":
            print(f"cycle without {g['avoid']}{n} through {g['through']}; reached after {g['trace']}")
        elif g["kind"] == "empty_post":
            print(f"post admits no successor{n}: {g['event']} at {g['at']}{first}")


def print_every(result) -> None:
    for s in result.deadlocks:
        print(f"deadlock at {result.describe(s)} after {result.trace_to(s)}")
    for s in result.bound_reached:
        print(f"bound reached at {result.describe(s)} after {result.trace_to(s)}{MORE}")
    for cycle in result.tau_cycles:
        print(f"tau cycle through {[c[0] for c in cycle]} reached after {result.trace_to(cycle[0])}")
    for name, s in result.violations:
        print(f"invariant {name!r} broken at {result.describe(s)} after {result.trace_to(s)}")
    for cycle in result.avoiding_cycles:
        print(f"cycle without {result.avoid} through {[c[0] for c in cycle]} reached after {result.trace_to(cycle[0])}")
    for s, label in result.empty_posts:
        print(f"post admits no successor: {label} at {result.describe(s)}, after {result.trace_to(s)}")


def print_environment(env: dict) -> None:
    """What reproduces the run: the seed and the functions given, then the
    functions the environment decided a clause by."""
    if env["seed"] is not None:
        print(f"environment: seed {env['seed']}")
    if env["given"]:
        print(f"environment: given {', '.join(env['given'])}")
    for name in env["decided"]:
        print(f"decided by the environment: {name}")


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(prog="csdfbmc", description=__doc__.split("\n\n")[1])
    add_inputs(ap)
    ap.add_argument("--max-states", type=int, default=100_000, help="stop after this many states (default 100000)")
    ap.add_argument("--avoid", metavar="EVENT", help="also report the cycles that never perform EVENT (a livelock under a fairness bound)")
    ap.add_argument("--within", metavar="STATE", action="append", help="with --avoid, look for the cycles among these control states only (repeatable)")
    ap.add_argument("--first", action="store_true", help="stop at the first deadlock or broken invariant (cycles are then not looked for)")
    ap.add_argument("--all", action="store_true", help="list every finding instead of one per shape with a count")
    ap.add_argument("--json", action="store_true", help="print the result as JSON")
    a = ap.parse_args(argv)

    try:
        with opened(a) as (diagram, checks, client, environment):
            symmetric = check_independent(diagram, checks, client, environment)
            result = Explorer(diagram, checks, client, environment=environment, max_states=a.max_states, avoid=a.avoid,
                              within=a.within, first=a.first, symmetric=symmetric).run()
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
        print_environment(result.environment)
    found = result.deadlocks or result.tau_cycles or result.violations or result.avoiding_cycles
    return 1 if found else 0


if __name__ == "__main__":
    sys.exit(main())
