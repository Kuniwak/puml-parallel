"""Random walks over the states of a diagram (csdfbmc --random-walk).

A walk starts at a start state and takes, at each state, one of the
transitions the walk would take (Explorer.ample) at random, each as likely
as the weight of its event (1 unless given), until it reaches the depth
given, a state with no way out, or a broken invariant. A walk keeps its own
path alone, so walks reach deep with little memory; they find deadlocks,
broken invariants and empty posts, never that there are none. Cycles need
the graph of the states, which no walk keeps.

Walk i draws from a generator seeded by (seed, i) alone, so that a seed
gives the same walks whatever order, or process, they are taken in.
"""
from __future__ import annotations

import collections
import csv
import random
from collections.abc import Callable, Iterable
from dataclasses import dataclass, field

from csdfbmc.explore import Explorer, Result, State
from csdfbmc.model import Edge
from csdfbmc.stopping import never


@dataclass
class Outcome:
    """A walk: its start, its steps (event as taken, edge, state reached),
    what ended it, and the events as taken whose post admitted no successor
    on the way."""
    start: State
    steps: list[tuple[str, Edge, State]] = field(default_factory=list)
    # ("deadlock", None), ("bound_reached", witness), ("violation", name), or
    # None when it reached the depth or a state that can end.
    finding: tuple[str, str | None] | None = None
    fired: collections.Counter = field(default_factory=collections.Counter)
    empty_posts: list[tuple[State, str]] = field(default_factory=list)
    decided: list[str] = field(default_factory=list)

    def events(self) -> list[str]:
        return [label for label, _, _ in self.steps]

    def last(self) -> State:
        return self.steps[-1][2] if self.steps else self.start


@dataclass
class RandomWalkReport:
    """What the walks found (result, in the form of a breadth-first walk's)
    and how far they went."""
    result: Result
    seed: int
    walks: int
    steps: int
    deepest: int
    fired: dict[str, int]
    never_fired: list[str]

    def to_json(self) -> dict:
        return {"walks": self.walks, "steps": self.steps, "deepest": self.deepest, "seed": self.seed,
                "fired": self.fired, "never_fired": self.never_fired}


def load_weights(path: str, diagram) -> dict[str, float]:
    """The weights of the events a TSV at path gives, a line per event: its
    name and a weight of 0 or more. An event the diagram has no edge of is
    refused, as a name mistyped would be."""
    names = {e.name for e in diagram.edges}
    weights: dict[str, float] = {}
    with open(path, encoding="utf-8", newline="") as f:
        rows = csv.reader(f, delimiter="\t")
        for cols in rows:
            n = rows.line_num
            if not cols:
                continue
            try:
                weight = float(cols[1]) if len(cols) == 2 else None
            except ValueError:
                weight = None
            if weight is None or weight < 0:
                raise ValueError(f"{path}:{n}: a weight line is an event and a weight of 0 or more, separated by a tab, not the columns {cols!r}")
            if cols[0] not in names:
                raise ValueError(f"{path}:{n}: no edge of the diagram has the event {cols[0]!r} this weight is for")
            weights[cols[0]] = weight
    return weights


class RandomWalks:
    def __init__(self, explorer: Explorer, max_depth: int, weights: dict[str, float], seed: int):
        self.explorer, self.max_depth, self.weights, self.seed = explorer, max_depth, weights, seed

    def walk(self, i: int) -> Outcome:
        """The i-th walk."""
        rng = random.Random(f"{self.seed}/{i}")
        x = self.explorer
        starts, decided = x.initial_deciding()
        s = rng.choice(starts)
        out = Outcome(start=s)
        out.decided = decided
        on_path = {s}
        broken = x.check_invariants(s)
        while not broken and len(out.steps) < self.max_depth:
            e = x.expand(s)
            out.decided.extend(name for name in e.decided if name not in out.decided)
            out.empty_posts.extend((s, label) for label in e.empty_posts)
            succ = x.ample(s, e.transitions, on_path.__contains__)
            weighted = [(t, self.weights.get(t[0], 1)) for t in succ]
            weighted = [(t, w) for t, w in weighted if w > 0]
            if not weighted:
                if not succ and not e.can_end:
                    out.finding = ("bound_reached", e.witness) if e.witness is not None else ("deadlock", None)
                break
            name, label, s, edge, _ = rng.choices([t for t, _ in weighted], [w for _, w in weighted])[0]
            out.steps.append((label, edge, s))
            out.fired[name] += 1
            on_path.add(s)
            broken = x.check_invariants(s)
        if broken:
            out.finding = ("violation", broken[0])
        return out

    def run(self, walks: int, take: Callable[[range], Iterable[Outcome]] | None = None, first: bool = False,
            stop: Callable[[], str | None] = never) -> RandomWalkReport:
        """walks walks, taken by take (in order; one after another here by
        default), stopping at the first finding with first. stop is asked
        before each walk is taken in, as Explorer.run asks it."""
        x = self.explorer
        parents: dict[State, tuple[State, str, Edge] | None] = {}
        deadlocks: list[State] = []
        bound_reached: list[tuple[State, str]] = []
        violations: list[tuple[str, State]] = []
        empty_posts: list[tuple[State, str]] = []
        seen_posts: set[tuple[State, str]] = set()
        seen_findings: set[tuple[str, str | None, State]] = set()
        fired: collections.Counter = collections.Counter()
        done = steps = deepest = 0
        stopped = False
        stopped_by = None
        for out in (take or (lambda r: map(self.walk, r)))(range(walks)):
            stopped_by = stop()
            if stopped_by is not None:
                break
            done += 1
            steps += len(out.steps)
            deepest = max(deepest, len(out.steps))
            fired.update(out.fired)
            x.note_decided(out.decided)
            kept = []
            for s, label in out.empty_posts:
                if (s, label) not in seen_posts:
                    seen_posts.add((s, label))
                    kept.append((s, label))
            if out.finding is not None or kept:
                _trace(parents, out)
                empty_posts.extend(kept)
            if out.finding is not None:
                kind, what = out.finding
                s = out.last()
                if (kind, what, s) in seen_findings:
                    continue
                seen_findings.add((kind, what, s))
                if kind == "deadlock":
                    deadlocks.append(s)
                elif kind == "bound_reached":
                    bound_reached.append((s, what))
                else:
                    violations.append((what, s))
                if first and kind != "bound_reached":
                    stopped = True
                    break
        if stopped_by is None:
            # take ends early when it is stopped (csdfbmc.parallel).
            stopped_by = stop()
        result = Result(
            stuck=x.stuck(deadlocks, parents), states=[], parents=parents, deadlocks=deadlocks, tau_cycles=[],
            truncated=False, bound=0, environment=x.environment_report(),
            violations=violations, ghost_names=x.ghost_names, empty_posts=empty_posts, stopped_early=stopped, stopped_by=stopped_by,
            bound_reached=bound_reached, left_out=x.left_out)
        events = sorted({e.name for e in x.d.edges if not e.is_tau})
        return RandomWalkReport(result=result, seed=self.seed, walks=done, steps=steps, deepest=deepest,
                     fired={name: fired[name] for name in sorted(fired, key=lambda n: (-fired[n], n))},
                     never_fired=[name for name in events if name not in fired])


def _trace(parents: dict, out: Outcome) -> None:
    """Records out's path in parents, a state met before keeping the way
    it was first reached."""
    parents.setdefault(out.start, None)
    prev = out.start
    for label, edge, s in out.steps:
        parents.setdefault(s, (prev, label, edge))
        prev = s
