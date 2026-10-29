"""python -m csdfbmc <diagram.json> <checks.py> (--evaluator CMD | --evaluator-module MODULE) [--max-states N] [--env NAME=true|false] [--avoid EVENT] [--first] [--all] [--json] [--cone] [--por] [--symmetry] [--stats] [--jobs N] [--progress SECONDS] [--checkpoint FILE [--checkpoint-every SECONDS]] [--resume FILE]

Reads a diagram as csdfparse prints it (or csdfpromote -format json), its
clauses as formulas: the builtins and ∃ are the checker's, the atoms the
checks module declares are functions of the environment, and every other
atom is read through a glue of the evaluator protocol (see csdfbmc.protocol;
csdfbmc.python_glue reads Python). The checks module gives the types, the
environment, the ghosts and the invariants. It walks the reachable states
within the bound, and reports the deadlocks, the tau cycles, the broken
invariants and, with --avoid, the cycles that never perform the event.
A state with no way out that the values beyond a type given as a sample
(csdfbmc.values.sample) give one is reported as the bound reached, with
the step they enable as its witness, not as a deadlock, and is no finding.
Findings of one shape are reported once with their count and the shortest
trace; --all lists every one, and --first stops the walk at the first
deadlock or broken invariant. --cone leaves out the variables nothing that
decides a step or a property reads (csdfbmc.cone), and --por takes the
transitions of an instance that moves independently of everything else
alone (csdfbmc.por), and --symmetry stores a state by one renaming of the
IDs of the types SYMMETRIC lists (csdfbmc.symmetry); each finds the same
findings; their counts, the states and --all are those of the reduced
walk, which a line naming the reductions says.
Exits 1 when it finds any, 2 on a usage or
loading error.
"""
import argparse
import json
import sys

from csdfbmc import checkpoint, parallel, progress
from csdfbmc.cli import LOADING_ERRORS, add_inputs, explorer, opened


# The reductions, by their options: each walks fewer states and finds the
# shapes of the full walk's findings, not their counts.
REDUCTIONS = ("cone", "por", "symmetry")


def beyond(witness: str) -> str:
    """Why a state where the bound was reached is no deadlock."""
    return f" ({witness} is enabled by a value beyond a sample)"


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
                print(f"bound reached at {g['example']} after {g['trace']}{beyond(g['witness'])}")
            else:
                print(f"bound reached{n} at {g['at']} after {g['after']}{first}{beyond(g['witness'])}")
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
    for s, w in result.bound_reached:
        print(f"bound reached at {result.describe(s)} after {result.trace_to(s)}{beyond(w)}")
    for cycle in result.tau_cycles:
        print(f"tau cycle through {[c[0] for c in cycle]} reached after {result.trace_to(cycle[0])}")
    for name, s in result.violations:
        print(f"invariant {name!r} broken at {result.describe(s)} after {result.trace_to(s)}")
    for cycle in result.avoiding_cycles:
        print(f"cycle without {result.avoid} through {[c[0] for c in cycle]} reached after {result.trace_to(cycle[0])}")
    for s, label in result.empty_posts:
        print(f"post admits no successor: {label} at {result.describe(s)}, after {result.trace_to(s)}")


def print_stats(stats: list[dict]) -> None:
    """What each edge took, the most valuations checked first."""
    print("stats per edge, most valuations checked first:")
    for s in stats:
        ranged = f"; ranged over their types: {', '.join(s['ranged'])}" if s["ranged"] else ""
        print(f"  {s['edge']}: tried {s['tried']}, guard held {s['guard_held']}, valuations checked {s['checked']}, "
              f"successors {s['successors']}{ranged}")


def print_environment(env: dict) -> None:
    """What reproduces the run: the functions given, then the
    functions the environment decided a clause by."""
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
    ap.add_argument("--por", action="store_true", help="take the transitions of an instance that moves independently alone: fewer states, the same findings")
    ap.add_argument("--symmetry", action="store_true", help="store a state by one renaming of the IDs of the types the checks module's SYMMETRIC lists: fewer states, the same findings")
    ap.add_argument("--cone", action="store_true", help="leave out the variables outside the cone of influence: fewer states, the same findings")
    ap.add_argument("--stats", action="store_true",
                    help="say, per edge, how many values it tried and checked, and what nothing determined, which ranged over its type")
    ap.add_argument("--jobs", type=int, default=1, metavar="N",
                    help="expand the states in N worker processes, each loading the inputs and the glue anew (default 1): the same walk, sooner when it is large")
    ap.add_argument("--progress", type=float, default=30, metavar="SECONDS",
                    help="say on stderr every SECONDS how far the walk got: the states, the frontier, the depth, "
                         "the states per second, the field values tried and the valuations checked since the last line (default 30; 0 says nothing)")
    ap.add_argument("--checkpoint", metavar="FILE",
                    help="write the walk to FILE as it goes, every --checkpoint-every seconds and at its end, for --resume to take up")
    ap.add_argument("--checkpoint-every", type=float, default=60, metavar="SECONDS",
                    help="with --checkpoint, write it at most every SECONDS (default 60)")
    ap.add_argument("--resume", metavar="FILE",
                    help="take up the walk --checkpoint wrote to FILE, of the same inputs: it says what the whole walk says. "
                         "FILE is a pickle, run as it is read: take up only those you wrote")
    a = ap.parse_args(argv)
    if a.jobs < 1:
        ap.error(f"--jobs expects a positive number, not {a.jobs}")
    if a.progress < 0:
        ap.error(f"--progress expects 0 or more seconds, not {a.progress:g}")
    if a.checkpoint_every < 0:
        ap.error(f"--checkpoint-every expects 0 or more seconds, not {a.checkpoint_every:g}")

    try:
        with opened(a) as (diagram, checks, client, environment):
            walk = explorer(a, diagram, checks, client, environment)
            digest = checkpoint.fingerprint(a) if a.checkpoint or a.resume else None
            resume = checkpoint.loads(checkpoint.load(a.resume, digest), diagram.edges) if a.resume else None
            saving = None
            if a.checkpoint:
                saving = checkpoint.Every(a.checkpoint_every,
                                          lambda w: checkpoint.save(a.checkpoint, digest, checkpoint.dumps(w, diagram.edges)))
            with parallel.expanding(a, diagram, a.jobs) as expand_all, progress.reporting(sys.stderr, a.progress) as said:
                result = walk.run(expand_all, said, resume=resume, checkpoint=saving)
            if saving is not None:
                saving.flush()
    except LOADING_ERRORS as e:
        print(f"csdfbmc: {e}", file=sys.stderr)
        return 2

    used = [name for name in REDUCTIONS if getattr(a, name)]
    if a.json:
        json.dump({**result.to_json(), "reductions": used}, sys.stdout, ensure_ascii=False)
        print()
    else:
        head = f"states: {len(result.states)}"
        if result.truncated:
            head += " (truncated at the bound)"
        if result.stopped_early:
            head += " (stopped at the first finding)"
        print(head)
        if used:
            print(f"reduced by {', '.join('--' + name for name in used)}: the findings are those of the full walk; "
                  "the counts and --all list the states of the reduced one")
        if result.left_out:
            print(f"cone of influence leaves out: {', '.join(result.left_out)}")
        if a.all:
            print_every(result)
        else:
            print_groups(result)
        print_environment(result.environment)
        if result.stats is not None:
            print_stats(result.stats)
    found = result.deadlocks or result.tau_cycles or result.violations or result.avoiding_cycles
    return 1 if found else 0


if __name__ == "__main__":
    sys.exit(main())
