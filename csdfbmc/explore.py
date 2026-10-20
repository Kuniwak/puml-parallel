"""The walk over the product of control states and valuations."""
from __future__ import annotations

import itertools
from dataclasses import dataclass, field

from csdfbmc.model import Diagram, Edge
from csdfbmc.twins import Twins

State = tuple[str, tuple]  # (state id, values of its variables in declaration order)


@dataclass
class Result:
    states: list[State]
    parents: dict[State, tuple[State, str] | None]
    deadlocks: list[State]
    tau_cycles: list[list[State]]
    opaque_clauses: list[str]
    truncated: bool
    bound: int

    def trace_to(self, state: State) -> list[str]:
        """The events from the start state to state, as the walk first reached it."""
        events: list[str] = []
        while self.parents[state] is not None:
            state, event = self.parents[state]
            events.append(event)
        return list(reversed(events))

    def to_json(self) -> dict:
        def st(s: State) -> dict:
            return {"state": s[0], "values": list(s[1])}
        return {
            "states": len(self.states),
            "truncated": self.truncated,
            "bound": self.bound,
            "deadlocks": [{"at": st(s), "trace": self.trace_to(s)} for s in self.deadlocks],
            "tau_cycles": [{"cycle": [st(s) for s in cycle], "trace": self.trace_to(cycle[0])} for cycle in self.tau_cycles],
            "opaque_clauses": self.opaque_clauses,
        }


class Explorer:
    """Enumerates the states a diagram can reach when its clauses mean what
    their twins say and its values stay inside the declared domains.

    opaque says what a clause without a twin means: "error" refuses it, "true"
    takes it as satisfied (an over-approximation, so a deadlock it reports may
    be spurious while a deadlock it misses cannot be hidden by it).
    """

    def __init__(self, diagram: Diagram, twins: Twins, *, opaque: str = "error", max_states: int = 100_000):
        if opaque not in ("error", "true"):
            raise ValueError(f"opaque must be 'error' or 'true', not {opaque!r}")
        self.d = diagram
        self.twins = twins
        self.opaque = opaque
        self.max_states = max_states
        self.opaque_seen: dict[str, None] = {}

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

    def _valuations(self, sid: str):
        names = self.d.vars[sid]
        return itertools.product(*(self.twins.domain(n) for n in names))

    def _env(self, sid: str, values: tuple, params: dict | None = None, after: tuple[str, tuple] | None = None) -> dict:
        env = dict(zip(self.d.vars[sid], values))
        if params:
            env.update(params)
        if after is not None:
            dst, dst_values = after
            env.update({n + "_": v for n, v in zip(self.d.vars[dst], dst_values)})
        return env

    # -- transitions ----------------------------------------------------------

    def initial(self) -> list[State]:
        return [(self.d.start, vals) for vals in self._valuations(self.d.start)
                if self._holds(self.d.start_post, self._env(self.d.start, vals))]

    def successors(self, state: State) -> list[tuple[str, State]]:
        """(event as taken, next state) for every enabled transition."""
        sid, values = state
        out: list[tuple[str, State]] = []
        for e in self.d.out_edges(sid):
            for args in itertools.product(*(self.twins.domain(p) for p in e.params)):
                params = dict(zip(e.params, args))
                if not self._holds(e.guard, self._env(sid, values, params)):
                    continue
                label = e.name if not e.params else e.name + "(" + ", ".join(f"{p}={v}" for p, v in params.items()) + ")"
                for dst_vals in self._valuations(e.dst):
                    if self._holds(e.post, self._env(sid, values, params, (e.dst, dst_vals))):
                        out.append((label, (e.dst, dst_vals)))
        return out

    def can_end(self, state: State) -> bool:
        end = self.d.end
        return end is not None and end.src == state[0] and self._holds(end.guard, self._env(state[0], state[1]))

    # -- the walk --------------------------------------------------------------

    def run(self) -> Result:
        parents: dict[State, tuple[State, str] | None] = {}
        order: list[State] = []
        queue: list[State] = []
        tau_next: dict[State, list[State]] = {}
        deadlocks: list[State] = []
        truncated = False

        for s in self.initial():
            if s not in parents:
                parents[s] = None
                order.append(s)
                queue.append(s)

        i = 0
        while i < len(queue):
            s = queue[i]
            i += 1
            succ = self.successors(s)
            tau_next[s] = [t for label, t in succ if label == "tau"]
            if not succ and not self.can_end(s):
                deadlocks.append(s)
            for label, t in succ:
                if t in parents:
                    continue
                if len(order) >= self.max_states:
                    truncated = True
                    continue
                parents[t] = (s, label)
                order.append(t)
                queue.append(t)

        return Result(
            states=order, parents=parents, deadlocks=deadlocks,
            tau_cycles=_tau_cycles(order, tau_next),
            opaque_clauses=list(self.opaque_seen), truncated=truncated, bound=self.max_states,
        )


def _tau_cycles(states: list[State], tau_next: dict[State, list[State]]) -> list[list[State]]:
    """One cycle per strongly connected component of the tau graph that has a
    cycle, in the order the states were met."""
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

    import sys
    sys.setrecursionlimit(max(sys.getrecursionlimit(), len(states) + 1000))
    for s in states:
        if s not in index:
            connect(s)
    return cycles
