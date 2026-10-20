"""The walk over the product of control states and valuations."""
from __future__ import annotations

import itertools
import sys
from dataclasses import dataclass, field

from csdfbmc.model import Diagram
from csdfbmc.twins import Twins, apply
from csdfbmc.values import freeze

# (state id, values of its variables in declaration order, values of the ghosts)
State = tuple[str, tuple, tuple]


@dataclass
class Result:
    states: list[State]
    parents: dict[State, tuple[State, str] | None]
    deadlocks: list[State]
    tau_cycles: list[list[State]]
    opaque_clauses: list[str]
    truncated: bool
    bound: int
    violations: list[tuple[str, State]] = field(default_factory=list)
    avoiding_cycles: list[list[State]] = field(default_factory=list)
    avoid: str | None = None
    ghost_names: tuple[str, ...] = ()
    # empty_posts: (state, event as taken) where the guard held but the post
    # twins returned no successor. A real post always admits some values, so
    # this is a twin that says less than the clause, or a clause that cannot
    # be satisfied from this state.
    empty_posts: list[tuple[State, str]] = field(default_factory=list)
    # unconstrained: (state, event as taken, variables) where the post twins
    # left variables of the next state without a value: a post clause is
    # missing, or its twin constrains fewer variables than it should.
    unconstrained: list[tuple[State, str, tuple[str, ...]]] = field(default_factory=list)

    def trace_to(self, state: State) -> list[str]:
        """The events from the start state to state, as the walk first reached it."""
        events: list[str] = []
        while self.parents[state] is not None:
            state, event = self.parents[state]
            events.append(event)
        return list(reversed(events))

    def to_json(self) -> dict:
        def st(s: State) -> dict:
            d = {"state": s[0], "values": list(s[1])}
            if self.ghost_names:
                d["ghosts"] = dict(zip(self.ghost_names, s[2]))
            return d
        return {
            "states": len(self.states),
            "truncated": self.truncated,
            "bound": self.bound,
            "deadlocks": [{"at": st(s), "trace": self.trace_to(s)} for s in self.deadlocks],
            "tau_cycles": [{"cycle": [st(s) for s in cycle], "trace": self.trace_to(cycle[0])} for cycle in self.tau_cycles],
            "violations": [{"invariant": name, "at": st(s), "trace": self.trace_to(s)} for name, s in self.violations],
            "avoid": self.avoid,
            "avoiding_cycles": [{"cycle": [st(s) for s in cycle], "trace": self.trace_to(cycle[0])} for cycle in self.avoiding_cycles],
            "empty_posts": [{"at": st(s), "event": label, "trace": self.trace_to(s)} for s, label in self.empty_posts],
            "unconstrained": [{"at": st(s), "event": label, "variables": list(names), "trace": self.trace_to(s)} for s, label, names in self.unconstrained],
            "opaque_clauses": self.opaque_clauses,
        }

    def describe(self, s: State) -> str:
        text = f"{s[0]} {list(s[1])}"
        if self.ghost_names:
            text += " " + str(dict(zip(self.ghost_names, s[2])))
        return text


class Explorer:
    """Enumerates the states a diagram can reach when its clauses mean what
    their twins say. A guard twin decides; a post twin returns the successor
    values of the variables it constrains, as a collection of partial
    assignments, and the successors of an edge are the natural join of what
    its clauses return. Event parameters range over the declared domains.

    opaque says what a clause without a twin means: "error" refuses it, "true"
    takes it as satisfied (an over-approximation, so a deadlock it reports may
    be spurious while a deadlock it misses cannot be hidden by it).

    avoid names an event; the walk then also reports every cycle among the
    reachable states that performs no transition of that event. Under a
    fairness assumption written as a finite bound (a resend count in the
    domains), such a cycle is a livelock: a run that never gets to the event.
    within narrows those cycles to the control states named.
    """

    def __init__(self, diagram: Diagram, twins: Twins, *, opaque: str = "error", max_states: int = 100_000,
                 avoid: str | None = None, within: list[str] | None = None):
        if opaque not in ("error", "true"):
            raise ValueError(f"opaque must be 'error' or 'true', not {opaque!r}")
        self.d = diagram
        self.twins = twins
        self.opaque = opaque
        self.max_states = max_states
        self.avoid = avoid
        # within narrows the avoiding cycles to the control states named: a
        # cycle that matters is one a run can stay in while something is in
        # flight, not the idle loop of the whole system.
        self.within = set(within) if within else None
        self.opaque_seen: dict[str, None] = {}
        self.ghost_names = tuple(twins.ghosts)
        self.empty_posts: list[tuple[State, str]] = []
        self.unconstrained: list[tuple[State, str, tuple[str, ...]]] = []

    # -- clauses --------------------------------------------------------------

    def _holds(self, clauses: tuple[str, ...], env: dict) -> bool:
        for c in clauses:
            if c not in self.twins.clauses:
                if self.opaque == "error":
                    raise KeyError(f"no twin for the clause {c!r}; add it to CLAUSES or run with opaque='true'")
                self.opaque_seen.setdefault(c)
                continue
            if not self.twins.evaluate(c, env):
                return False
        return True

    def _env(self, state: State, params: dict | None = None) -> dict:
        sid, values, ghosts = state
        env = dict(zip(self.ghost_names, ghosts))
        env.update(zip(self.d.vars[sid], values))
        if params:
            env.update(params)
        return env

    def _successor_values(self, clauses: tuple[str, ...], env: dict, dst: str) -> tuple[list[tuple], tuple[str, ...]]:
        """The valuations of dst the post clauses admit, as the natural join of
        the assignments each clause returns, and the variables of dst that no
        clause gave a value (then the valuations are empty)."""
        joined: list[dict] = [{}]
        for c in clauses:
            if c not in self.twins.clauses:
                if self.opaque == "error":
                    raise KeyError(f"no twin for the clause {c!r}; add it to CLAUSES or run with opaque='true'")
                self.opaque_seen.setdefault(c)
                continue
            returned = apply(self.twins.clauses[c], env, what=f"the twin of {c!r}")
            if isinstance(returned, (bool, int, str)) or returned is None:
                raise TypeError(f"the twin of the post clause {c!r} returned {returned!r}; a post twin returns a collection of assignments such as [{{'x': x + 1}}]")
            assignments = []
            for a in returned:
                if not hasattr(a, "items"):
                    raise TypeError(f"the twin of the post clause {c!r} returned {a!r}; each successor is a mapping from variable names to values")
                assignments.append({k: freeze(v) for k, v in a.items()})
            joined = [dict(j, **a) for j in joined for a in assignments
                      if all(j[k] == v for k, v in a.items() if k in j)]
            if not joined:
                break
        names = self.d.vars[dst]
        if joined:
            missing = tuple(n for n in names if n not in joined[0])
            if missing:
                return [], missing
        return [tuple(j[n] for n in names) for j in joined], ()

    def _ghosts_after(self, state: State, name: str, params: dict) -> tuple:
        env = self._env(state, params)
        out = []
        for ghost, (_, updates) in self.twins.ghosts.items():
            if name in updates:
                out.append(apply(updates[name], env, what=f"the update of ghost {ghost!r} on {name}"))
            else:
                out.append(env[ghost])
        return tuple(out)

    def _check_invariants(self, state: State) -> list[str]:
        env = self._env(state)
        broken = []
        for name, fn in self.twins.invariants.items():
            if self.twins.binds(fn, env) and not apply(fn, env, what=f"the invariant {name!r}"):
                broken.append(name)
        return broken

    # -- transitions ----------------------------------------------------------

    def initial(self) -> list[State]:
        ghosts = tuple(init for init, _ in self.twins.ghosts.values())
        env = dict(zip(self.ghost_names, ghosts))
        vals, missing = self._successor_values(self.d.start_post, env, self.d.start)
        if missing:
            raise KeyError(f"the start edge leaves {list(missing)} of state {self.d.start!r} without a value; its post twins must assign every variable")
        return [(self.d.start, v, ghosts) for v in vals]

    def successors(self, state: State) -> list[tuple[str, str, State]]:
        """(event name, event as taken, next state) for every enabled transition."""
        sid, values, _ = state
        out: list[tuple[str, str, State]] = []
        for e in self.d.out_edges(sid):
            for args in itertools.product(*(self.twins.domain(p) for p in e.params)):
                params = dict(zip(e.params, args))
                if not self._holds(e.guard, self._env(state, params)):
                    continue
                label = e.name if not e.params else e.name + "(" + ", ".join(f"{p}={v}" for p, v in params.items()) + ")"
                ghosts = self._ghosts_after(state, e.name, params)
                vals, missing = self._successor_values(e.post, self._env(state, params), e.dst)
                if missing:
                    self.unconstrained.append((state, label, missing))
                    continue
                if not vals:
                    self.empty_posts.append((state, label))
                    continue
                for dst_vals in vals:
                    out.append((e.name, label, (e.dst, dst_vals, ghosts)))
        return out

    def can_end(self, state: State) -> bool:
        end = self.d.end
        return end is not None and end.src == state[0] and self._holds(end.guard, self._env(state))

    # -- the walk --------------------------------------------------------------

    def run(self) -> Result:
        parents: dict[State, tuple[State, str] | None] = {}
        order: list[State] = []
        queue: list[State] = []
        tau_next: dict[State, list[State]] = {}
        avoid_next: dict[State, list[State]] = {}
        deadlocks: list[State] = []
        violations: list[tuple[str, State]] = []
        truncated = False

        def visit(s: State):
            parents.setdefault(s, None)
            order.append(s)
            queue.append(s)
            for name in self._check_invariants(s):
                violations.append((name, s))

        for s in self.initial():
            if s not in parents:
                visit(s)

        i = 0
        while i < len(queue):
            s = queue[i]
            i += 1
            succ = self.successors(s)
            tau_next[s] = [t for name, _, t in succ if name == "tau"]
            if self.avoid is not None and (self.within is None or s[0] in self.within):
                avoid_next[s] = [t for name, _, t in succ if name != self.avoid and (self.within is None or t[0] in self.within)]
            if not succ and not self.can_end(s):
                deadlocks.append(s)
            for _, label, t in succ:
                if t in parents:
                    continue
                if len(order) >= self.max_states:
                    truncated = True
                    continue
                parents[t] = (s, label)
                visit(t)

        return Result(
            states=order, parents=parents, deadlocks=deadlocks,
            tau_cycles=_cycles(order, tau_next),
            opaque_clauses=list(self.opaque_seen), truncated=truncated, bound=self.max_states,
            violations=violations,
            avoiding_cycles=_cycles(order, avoid_next) if self.avoid is not None else [],
            avoid=self.avoid, ghost_names=self.ghost_names,
            empty_posts=self.empty_posts, unconstrained=self.unconstrained,
        )


def _cycles(states: list[State], tau_next: dict[State, list[State]]) -> list[list[State]]:
    """One cycle per strongly connected component of the given graph that has
    a cycle, in the order the states were met."""
    index: dict[State, int] = {}
    low: dict[State, int] = {}
    on_stack: set[State] = set()
    stack: list[State] = []
    cycles: list[list[State]] = []
    counter = itertools.count()

    def connect(v: State):
        index[v] = low[v] = next(counter)
        stack.append(v)
        on_stack.add(v)
        for w in tau_next.get(v, ()):
            if w not in index:
                connect(w)
                low[v] = min(low[v], low[w])
            elif w in on_stack:
                low[v] = min(low[v], index[w])
        if low[v] == index[v]:
            comp = []
            while True:
                w = stack.pop()
                on_stack.discard(w)
                comp.append(w)
                if w == v:
                    break
            comp.reverse()
            if len(comp) > 1 or v in tau_next.get(v, ()):
                cycles.append(comp)

    sys.setrecursionlimit(max(sys.getrecursionlimit(), len(states) + 1000))
    for s in states:
        if s not in index:
            connect(s)
    return cycles
