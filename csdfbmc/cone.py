"""The cone of influence: the state variables whose values can decide, along
some path, whether a step can be taken, what it leads to in them, or what a
property says (docs/CLAUSE_DESIGN.md, Q1 and O9).

What a clause reads is its free names: the holes of its atoms and the terms
of its builtins, which the tools know without asking a glue. The variables
kept, R, are the least set holding

- every name a guard reads, of an edge or an end edge;
- every name an invariant or a ghost update reads, by its signature;
- every state variable an event sends, whose value is part of the event;
- the map of every family;
- every name a conjunct of a post reads before the step, except a copy
  @eq(x', y) of a variable y into a variable x' no other conjunct of the
  post gives a value: y is in R only when x is.

A variable outside R is then never read by anything that decides a step or
a property, and only a copy carries its value into another variable outside
R. A copy always admits its one value, so whether a post admits a successor
and what its successors hold in R never depend on a variable outside R. The
walk keeps such a variable at csdfbmc.values.LEFT_OUT, and the walk projected so is
bisimilar to the full one in the control states and R: it finds the same
deadlocks, cycles, broken invariants, empty posts and bound reached.

A property whose reads are unknown, an invariant or a ghost update taking
any keyword, is refused: what it reads cannot be told.
"""
from __future__ import annotations

from csdfbmc.checks import Checks, reads
from csdfbmc.model import Builtin, Diagram, Free, free_names, root


def property_reads(checks: Checks, why: str) -> set[str]:
    """The names the invariants and the ghost updates read; refused when one
    takes any keyword, since what it reads cannot be told."""
    out: set[str] = set()
    what = [(f"the invariant {name!r}", fn) for name, fn in checks.invariants.items()]
    what += [(f"the update of ghost {g!r} on {event}", fn) for g, (_, updates) in checks.ghosts.items() for event, fn in updates.items()]
    for name, fn in what:
        names = reads(fn)
        if names is None:
            raise ValueError(f"{why} needs to know what {name} reads, but it takes any keyword; name its parameters")
        out |= names
    return out


def _copy(c) -> tuple[str, str] | None:
    """(x, y) when c is the copy @eq(x', y) of a variable into a variable."""
    if isinstance(c, Builtin) and c.op == "eq":
        for x, y in ((c.args[0], c.args[1]), (c.args[1], c.args[0])):
            if isinstance(x, Free) and x.primed and isinstance(y, Free) and not y.primed:
                return x.name, y.name
    return None


def _written(c) -> set[str]:
    """The variables a conjunct gives a value after the step, by their roots."""
    out: set[str] = set()

    def walk(x):
        if hasattr(x, "args"):
            for a in x.args:
                r = root(a)
                if isinstance(r, Free) and r.primed:
                    out.add(r.name)
                walk(a)
        elif hasattr(x, "body"):
            for b in x.body:
                walk(b)
    walk(c)
    return out


def left_out(d: Diagram, checks: Checks) -> tuple[str, ...]:
    """The state variables outside the cone of influence, in order."""
    variables = list(dict.fromkeys(v for vs in d.vars.values() for v in vs))
    kept: set[str] = set(property_reads(checks, "--cone")) | set(d.families)
    for e in d.edges:
        kept |= free_names(e.guard)
        kept |= {n for n in e.sent if n in variables}
    for x in d.ends:
        kept |= free_names(x.guard)
    copies: list[tuple[str, str]] = []
    for post in [d.start_post] + [e.post for e in d.edges]:
        writers: dict[str, int] = {}
        for c in post:
            for x in _written(c):
                writers[x] = writers.get(x, 0) + 1
        for c in post:
            copy = _copy(c)
            if copy is not None and writers[copy[0]] == 1:
                copies.append(copy)
            else:
                kept |= free_names(c, primed=False)
    grown = True
    while grown:
        grown = False
        for x, y in copies:
            if x in kept and y not in kept:
                kept.add(y)
                grown = True
    return tuple(v for v in variables if v not in kept)
