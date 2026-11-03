"""python -m csdfbmc <diagram.json> <checks.py> (--evaluator CMD | --evaluator-module MODULE) [--max-states N] [--env NAME=true|false] [--avoid EVENT] [--first] [--all] [--json] [--cone] [--por] [--symmetry] [--stats] [--jobs N] [--progress SECONDS] [--checkpoint FILE [--checkpoint-every SECONDS]] [--resume FILE] [--random-walk [--walks N] [--max-depth D] [--seed S] [--weight-tsv FILE]]

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
SIGINT or SIGTERM stops the walk where it is, with the report of what it
found until then (csdfbmc.stopping).
--random-walk takes walks at random instead (csdfbmc.randomwalk), for
findings deeper than the breadth-first walk reaches.
Exits 1 when it finds any, 2 on a usage or
loading error.
"""
import argparse
import json
import random
import sys

from csdfbmc import checkpoint, parallel, progress
from csdfbmc.cli import LOADING_ERRORS, add_inputs, explorer, opened, random_walks
from csdfbmc.randomwalk import RandomWalkReport
from csdfbmc.stopping import on_signals

# The bound on the states of a breadth-first walk unless given.
MAX_STATES = 100_000

# The options of a breadth-first walk alone, by their destinations: the
# random walks keep no graph of the states, nor the states.
BREADTH_FIRST_ONLY = ("avoid", "within", "symmetry", "max_states", "stats", "checkpoint", "resume")

# The options of the random walks alone.
RANDOM_WALK_ONLY = ("walks", "max_depth", "seed", "weight_tsv")


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
            print_stuck(g["stuck"])
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


def print_stuck(stuck: list[dict]) -> None:
    """Why each edge out of a deadlock takes no step (Explorer.why_stuck)."""
    for why in stuck:
        if why["reason"] == "state":
            text = f"guard false on the state: {why['conjunct']}"
        elif why["reason"] == "post":
            text = "the guard holds, but the post admits no successor"
        elif why["conjunct"] is None:
            text = f"no value of {', '.join('?' + f for f in why['fields'])} to try"
        else:
            text = f"no value of {', '.join('?' + f for f in why['fields'])} satisfies {why['conjunct']} (tried {why['tried']})"
        if "hint" in why:
            text += f"; {why['hint']}"
        print(f"  stuck: {why['edge']}: {text}")


def print_every(result) -> None:
    for s in result.deadlocks:
        print(f"deadlock at {result.describe(s)} after {result.trace_to(s)}")
        print_stuck(result.stuck.get(s, []))
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


def print_fired(walks) -> None:
    """How often the random walks fired each event, and which never."""
    print(f"fired: {', '.join(f'{name} {n}' for name, n in walks.fired.items()) or 'nothing'}")
    if walks.never_fired:
        print(f"never fired: {', '.join(walks.never_fired)}")


def print_environment(env: dict) -> None:
    """What reproduces the run: the functions given, then the
    functions the environment decided a clause by."""
    if env["given"]:
        print(f"environment: given {', '.join(env['given'])}")
    for name in env["decided"]:
        print(f"decided by the environment: {name}")


def main(argv=None, stopping=on_signals, reporting=progress.reporting) -> int:
    """csdfbmc on argv; stopping is what a signal stops the walk by
    (csdfbmc.stopping.on_signals), and reporting what says its progress
    (csdfbmc.progress.reporting)."""
    ap = argparse.ArgumentParser(prog="csdfbmc", description=__doc__.split("\n\n")[1])
    add_inputs(ap)
    ap.add_argument("--max-states", type=int, help=f"stop after this many states (default {MAX_STATES})")
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
                         "the states per second, the field values tried and the valuations checked since the last line, the time spent and, on macOS and Linux, the memory held in all and per state (default 30; 0 says nothing)")
    ap.add_argument("--checkpoint", metavar="FILE",
                    help="write the walk to FILE as it goes, every --checkpoint-every seconds and at its end, for --resume to take up")
    ap.add_argument("--checkpoint-every", type=float, default=60, metavar="SECONDS",
                    help="with --checkpoint, write it at most every SECONDS (default 60)")
    ap.add_argument("--resume", metavar="FILE",
                    help="take up the walk --checkpoint wrote to FILE, of the same inputs: it says what the whole walk says. "
                         "FILE is a pickle, run as it is read: take up only those you wrote")
    ap.add_argument("--random-walk", action="store_true",
                    help="instead of the breadth-first walk, take --walks walks from the start, each step one of the ways out at random, "
                         "as likely as its event's weight, up to --max-depth steps: findings deep, with the memory of one path; "
                         "never that there are none, nor cycles")
    ap.add_argument("--walks", type=int, metavar="N", help="with --random-walk, take N walks (default 1000)")
    ap.add_argument("--max-depth", type=int, metavar="D", help="with --random-walk, end a walk after D steps (default 1000)")
    ap.add_argument("--seed", type=int, metavar="S",
                    help="with --random-walk, the seed the walks are drawn by, the same walks with any --jobs (default: one drawn and said)")
    ap.add_argument("--weight-tsv", metavar="FILE",
                    help="with --random-walk, the weights of the events, a line each: the event and a weight of 0 or more, "
                         "separated by a tab (default 1)")
    a = ap.parse_args(argv)
    if a.random_walk:
        given = [name for name in BREADTH_FIRST_ONLY if getattr(a, name)]
        if given:
            ap.error(f"--random-walk takes no {', '.join('--' + n.replace('_', '-') for n in given)}: "
                     "the random walks keep no graph of the states")
        a.walks = 1000 if a.walks is None else a.walks
        a.max_depth = 1000 if a.max_depth is None else a.max_depth
        a.seed = random.randrange(2 ** 32) if a.seed is None else a.seed
        if a.walks < 1 or a.max_depth < 1:
            ap.error("--walks and --max-depth expect a positive number")
    else:
        given = [name for name in RANDOM_WALK_ONLY if getattr(a, name) is not None]
        if given:
            ap.error(f"{', '.join('--' + n.replace('_', '-') for n in given)} need --random-walk")
        a.max_states = MAX_STATES if a.max_states is None else a.max_states
    if a.jobs < 1:
        ap.error(f"--jobs expects a positive number, not {a.jobs}")
    if a.progress < 0:
        ap.error(f"--progress expects 0 or more seconds, not {a.progress:g}")
    if a.checkpoint_every < 0:
        ap.error(f"--checkpoint-every expects 0 or more seconds, not {a.checkpoint_every:g}")

    # Inside, a signal stops the walk where it is, and one after it is
    # ignored while the report and the checkpoint are written.
    with stopping() as stop:
        if a.random_walk:
            return main_random_walks(a, stop, reporting)
        return main_walk(a, stop, reporting)


def main_walk(a: argparse.Namespace, stop, reporting) -> int:
    try:
        with opened(a) as (diagram, checks, client, environment):
            walk = explorer(a, diagram, checks, client, environment)
            digest = checkpoint.fingerprint(a) if a.checkpoint or a.resume else None
            resume = checkpoint.loads(checkpoint.load(a.resume, digest), diagram.edges) if a.resume else None
            saving = None
            if a.checkpoint:
                saving = checkpoint.Every(a.checkpoint_every,
                                          lambda w: checkpoint.save(a.checkpoint, digest, checkpoint.dumps(w, diagram.edges)))
            with parallel.expanding(a, diagram, a.jobs, stop) as expand_all, reporting(sys.stderr, a.progress, progress.Progress) as said:
                result = walk.run(expand_all, said.update, resume=resume, checkpoint=saving, stop=stop)
            if saving is not None:
                saving.flush()
    except LOADING_ERRORS as e:
        print(f"csdfbmc: {e}", file=sys.stderr)
        return 2

    head = f"states: {len(result.states)}"
    if result.truncated:
        head += " (truncated at the bound)"
    return report(a, result, head)


def main_random_walks(a: argparse.Namespace, stop, reporting) -> int:
    try:
        with opened(a) as (diagram, checks, client, environment):
            walker = random_walks(a, explorer(a, diagram, checks, client, environment))
            with parallel.walking(a, diagram, a.jobs, stop) as take, \
                    reporting(sys.stderr, a.progress, progress.WalkProgress) as said:
                got = walker.run(a.walks, take, first=a.first, stop=stop, progress=said.update)
    except LOADING_ERRORS as e:
        print(f"csdfbmc: {e}", file=sys.stderr)
        return 2
    head = f"random walks: {got.walks}, steps: {got.steps}, deepest: {got.deepest}, seed: {got.seed}"
    return report(a, got.result, head, got)


def report(a: argparse.Namespace, result, head: str, walks: RandomWalkReport | None = None) -> int:
    """Prints result, a breadth-first walk's or the random walks' (walks),
    as the arguments a ask; the exit status."""
    used = [name for name in REDUCTIONS if getattr(a, name)]
    if a.json:
        said = result.to_json()
        if walks is not None:
            # The walks keep no states to count; they say how far they went.
            del said["states"]
            said["random_walk"] = walks.to_json()
        json.dump({**said, "reductions": used}, sys.stdout, ensure_ascii=False)
        print()
    else:
        if result.stopped_early:
            head += " (stopped at the first finding)"
        if result.stopped_by is not None:
            head += f" (stopped by {result.stopped_by})"
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
        if walks is not None:
            print_fired(walks)
    found = result.deadlocks or result.tau_cycles or result.violations or result.avoiding_cycles
    return 1 if found else 0


if __name__ == "__main__":
    sys.exit(main())
