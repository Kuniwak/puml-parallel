"""The walk over the product of control states and valuations."""
from __future__ import annotations

import itertools
import sys
from dataclasses import dataclass, field

from csdfbmc.model import Clause, Diagram
from csdfbmc.twins import Twins, apply, check_pure
from csdfbmc.values import FrozenMap, freeze

# The key under which an instance's record holds its local state's name; the
# local variables are the other keys.
RECORD_STATE = "state"

# What a formal expression may call besides the names of the step.
FORMAL_GLOBALS = {
    "__builtins__": {},
    "len": len, "all": all, "any": any, "sum": sum, "min": min, "max": max, "abs": abs,
    "sorted": sorted, "set": set, "frozenset": frozenset, "tuple": tuple, "range": range,
    "FrozenMap": FrozenMap,
    "at": lambda m, i, state: i in m and m[i][RECORD_STATE] == state,
    "exists": lambda m, pred: any(pred(i, r) for i, r in m.items()),
    "count": lambda m, pred: sum(1 for i, r in m.items() if pred(i, r)),
}

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

    In an expansion of a promoted diagram, a map variable holds, per instance
    ID, a record of the local state's name (under "state") and the local
    variables. The generated clauses (in_dom, at_state, update, create, delete,
    frame) have their meaning built in; a local clause is evaluated with the
    family's twins over the instance's variables and the event's parameters;
    a constrain clause with the global twins, or with the formal expression
    written next to it.

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
        self.within = set(within) if within else None
        self.opaque_seen: dict[str, None] = {}
        self.ghost_names = tuple(twins.ghosts)
        self.empty_posts: list[tuple[State, str]] = []
        self.unconstrained: list[tuple[State, str, tuple[str, ...]]] = []
        self._formals: dict[str, object] = {}

    # -- environments -----------------------------------------------------------

    def _env(self, state: State, params: dict | None = None) -> dict:
        sid, values, ghosts = state
        env = dict(zip(self.ghost_names, ghosts))
        env.update(zip(self.d.vars[sid], values))
        if params:
            env.update(params)
        return env

    def _id_of(self, clause: Clause, env: dict):
        param = clause.id or self.d.promotes[clause.map].id_param
        if param not in env:
            raise KeyError(f"the clause {clause.text!r} needs the instance ID {param!r}, which the event does not carry")
        return env[param]

    def _instance_env(self, clause: Clause, env: dict) -> dict:
        """The names a local clause of a family may read: the event's
        parameters and ghosts, and the instance's variables when it exists."""
        local_env = dict(env)
        record = env[clause.map].get(self._id_of(clause, env)) if clause.map in env else None
        if record is not None:
            local_env.update((k, v) for k, v in record.items() if k != RECORD_STATE)
        return local_env

    # -- twins ------------------------------------------------------------------

    def _twin(self, clause: Clause):
        """The callable standing for a clause, or None when there is none."""
        if clause.kind == "local" and clause.map:
            return self.twins.local(self.d.local_of(clause).path).clauses.get(clause.text)
        fn = self.twins.clauses.get(clause.text)
        if fn is None and clause.kind == "constrain" and clause.formal:
            fn = self._formal(clause)
        return fn

    def _formal(self, clause: Clause):
        code = self._formals.get(clause.text)
        if code is None:
            check_pure(clause.formal, f"<formal line {clause.line}>")
            code = compile(clause.formal, f"<formal line {clause.line}>", "eval")
            self._formals[clause.text] = code

        def run(**env):
            return eval(code, FORMAL_GLOBALS, env)
        run.formal = True
        return run

    def _call(self, clause: Clause, env: dict):
        fn = self._twin(clause)
        if fn is None:
            return None
        if getattr(fn, "formal", False):
            return fn(**env)
        return apply(fn, env, what=f"the twin of {clause.text!r}")

    def _missing_twin(self, clause: Clause) -> bool:
        """Records an opaque clause, or refuses it; True when it is to be taken as true."""
        if self.opaque == "error":
            where = f"in the twins of {self.d.local_of(clause).path}" if clause.kind == "local" and clause.map else "in CLAUSES"
            raise KeyError(f"no twin for the clause {clause.text!r} {where}; add it, or run with opaque='true'")
        self.opaque_seen.setdefault(clause.text)
        return True

    # -- guards -----------------------------------------------------------------

    def _holds(self, clauses: tuple[Clause, ...], env: dict) -> bool:
        return all(self._guard_holds(c, env) for c in clauses)

    def _guard_holds(self, c: Clause, env: dict) -> bool:
        if c.kind == "in_dom":
            return self._id_of(c, env) in env[c.map]
        if c.kind == "not_in_dom":
            return self._id_of(c, env) not in env[c.map]
        if c.kind == "at_state":
            record = env[c.map].get(self._id_of(c, env))
            return record is not None and record[RECORD_STATE] == c.state
        if c.kind == "frame":
            return True
        if c.kind not in ("local", "constrain"):
            raise ValueError(f"unknown clause kind {c.kind!r} in a guard")
        if self._twin(c) is None:
            return self._missing_twin(c)
        scope = self._instance_env(c, env) if c.map and c.kind == "local" else env
        return bool(self._call(c, scope))

    # -- posts ------------------------------------------------------------------

    def _assignments(self, c: Clause, env: dict) -> list[dict] | None:
        """What a post twin returns, as frozen assignments; None when opaque."""
        if self._twin(c) is None:
            self._missing_twin(c)
            return None
        returned = self._call(c, env)
        if isinstance(returned, (bool, int, str)) or returned is None:
            raise TypeError(f"the twin of the post clause {c.text!r} returned {returned!r}; a post twin returns a collection of assignments such as [{{'x': x + 1}}]")
        out = []
        for a in returned:
            if not hasattr(a, "items"):
                raise TypeError(f"the twin of the post clause {c.text!r} returned {a!r}; each successor is a mapping from variable names to values")
            out.append({k: freeze(v) for k, v in a.items()})
        return out

    @staticmethod
    def _join(joined: list[dict], assignments: list[dict]) -> list[dict]:
        return [dict(j, **a) for j in joined for a in assignments
                if all(j[k] == v for k, v in a.items() if k in j)]

    def _successor_values(self, clauses: tuple[Clause, ...], env: dict, dst: str) -> tuple[list[tuple], tuple[str, ...]]:
        """The valuations of dst the post clauses admit, and the variables no
        clause gave a value (then the valuations are empty)."""
        joined: list[dict] = [{}]
        moves: dict[str, Clause] = {}
        local_posts: dict[str, list[Clause]] = {}
        for c in clauses:
            if c.kind == "frame":
                joined = self._join(joined, [{c.var: env[c.var]}])
            elif c.kind in ("update", "create", "delete"):
                moves[c.map] = c
            elif c.kind == "local" and c.map:
                local_posts.setdefault(c.map, []).append(c)
            elif c.kind in ("local", "constrain"):
                a = self._assignments(c, env)
                if a is not None:
                    joined = self._join(joined, a)
            else:
                raise ValueError(f"unknown clause kind {c.kind!r} in a post")
            if not joined:
                return [], ()

        for m, move in moves.items():
            idv = self._id_of(move, env)
            current = env[m]
            if move.kind == "delete":
                joined = self._join(joined, [{m: current.without(idv)}])
                continue
            local = self.d.local_of(move)
            names = local.vars_of(move.state)
            scope = self._instance_env(move, env)
            records: list[dict] = [{}]
            for c in local_posts.get(m, []):
                a = self._assignments(c, scope)
                if a is not None:
                    records = self._join(records, a)
                if not records:
                    return [], ()
            missing = tuple(f"{m}[{idv}].{n}" for n in names if n not in records[0])
            if missing:
                return [], missing
            joined = self._join(joined, [
                {m: current.set(idv, FrozenMap({RECORD_STATE: move.state, **{n: r[n] for n in names}}))} for r in records])
            if not joined:
                return [], ()
        for m in local_posts:
            if m not in moves:
                raise ValueError(f"the post has local clauses for map {m!r} but no update, create or delete of it")

        names = self.d.vars[dst]
        if joined:
            missing = tuple(n for n in names if n not in joined[0])
            if missing:
                return [], missing
        return [tuple(j[n] for n in names) for j in joined], ()

    # -- ghosts and invariants --------------------------------------------------

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

    # -- transitions ------------------------------------------------------------

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
            names = e.params + e.implicit_ids()
            for args in itertools.product(*(self.twins.domain(p) for p in names)):
                params = dict(zip(names, args))
                if not self._holds(e.guard, self._env(state, params)):
                    continue
                label = e.name if not names else e.name + "(" + ", ".join(f"{p}={v}" for p, v in params.items()) + ")"
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

    # -- the walk ----------------------------------------------------------------

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
