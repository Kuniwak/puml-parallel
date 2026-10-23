"""The walk over the product of control states and valuations."""
from __future__ import annotations

import dataclasses
import inspect
import itertools
import sys
from dataclasses import dataclass, field

from csdfbmc.checks import Checks, apply
from csdfbmc.environment import Environment
from csdfbmc.formula import Evaluator, Missing, solve
from csdfbmc.independence import check_independent
from csdfbmc.meaning import AtomMeaning, apply_given
from csdfbmc.model import Builtin, Diagram, Edge, Free, free_names
from csdfbmc.protocol import ClientLike
from csdfbmc.values import encode, freeze, fresh, scalars, visible

# (state id, values of its variables in declaration order, values of the ghosts)
State = tuple[str, tuple, tuple]


@dataclass
class Result:
    states: list[State]
    # parents: state -> (previous state, event as taken, the edge) or None for
    # an initial state.
    parents: dict[State, tuple[State, str, "Edge"] | None]
    deadlocks: list[State]
    tau_cycles: list[list[State]]
    truncated: bool
    bound: int
    # environment: the seed and the given functions of the environment, and
    # the functions it decided an atom by (csdfbmc.environment); what a run
    # needs to be reproduced.
    environment: dict
    violations: list[tuple[str, State]] = field(default_factory=list)
    avoiding_cycles: list[list[State]] = field(default_factory=list)
    avoid: str | None = None
    ghost_names: tuple[str, ...] = ()
    # empty_posts: (state, event as taken) where the guard held but the post
    # admitted no successor: a clause that cannot be satisfied from this
    # state, or clauses that disagree.
    empty_posts: list[tuple[State, str]] = field(default_factory=list)
    # stopped_early: the walk stopped at the first finding (first=True).
    stopped_early: bool = False
    # bound_reached: states with no way out that a value of each sample type
    # occurring nowhere would give one: the bound, not the diagram, ends
    # them, so they are not deadlocks.
    bound_reached: list[State] = field(default_factory=list)

    def trace_to(self, state: State) -> list[str]:
        """The events from the start state to state, as the walk first reached it."""
        events: list[str] = []
        while self.parents[state] is not None:
            state, event, _ = self.parents[state]
            events.append(event)
        return list(reversed(events))

    def replay_to(self, state: State) -> dict:
        """What a REPL needs to walk to state: the initial values and, per
        step, the edge taken (its event text, source, destination and line)
        and the values of the state it leads to, as JSON."""
        steps = []
        while self.parents[state] is not None:
            prev, _, edge = self.parents[state]
            steps.append({"event": edge.event, "src": edge.src, "dst": edge.dst, "line": edge.line,
                          "values": [encode(v) for v in state[1]]})
            state = prev
        steps.reverse()
        return {"initial": [encode(v) for v in state[1]], "steps": steps}

    def to_json(self) -> dict:
        def st(s: State) -> dict:
            d = {"state": s[0], "values": [encode(v) for v in s[1]]}
            if self.ghost_names:
                d["ghosts"] = dict(zip(self.ghost_names, s[2]))
            return d

        def at(s: State) -> dict:
            return {"at": st(s), "trace": self.trace_to(s), "replay": self.replay_to(s)}
        return {
            "states": len(self.states),
            "truncated": self.truncated,
            "bound": self.bound,
            "deadlocks": [at(s) for s in self.deadlocks],
            "bound_reached": [at(s) for s in self.bound_reached],
            "tau_cycles": [{"cycle": [st(s) for s in cycle], **at(cycle[0])} for cycle in self.tau_cycles],
            "violations": [{"invariant": name, **at(s)} for name, s in self.violations],
            "avoid": self.avoid,
            "avoiding_cycles": [{"cycle": [st(s) for s in cycle], **at(cycle[0])} for cycle in self.avoiding_cycles],
            "empty_posts": [{"event": label, **at(s)} for s, label in self.empty_posts],
            "environment": self.environment,
            "groups": self.groups(),
            "stopped_early": self.stopped_early,
        }

    def groups(self) -> list[dict]:
        """The findings folded by shape, each with its count and the first
        (shortest) representative: deadlocks, and the states where the bound
        was reached, by (control state, the event that led there), broken invariants by (invariant, control state),
        empty posts by event, cycles by the set of control states they run
        through. In the order the first of each was met."""
        groups: dict[tuple, dict] = {}

        def add(key: tuple, make):
            g = groups.get(key)
            if g is None:
                g = make()
                g["count"] = 0
                groups[key] = g
            g["count"] += 1

        def last_event(s: State) -> str:
            p = self.parents[s]
            return p[2].name if p is not None else ""

        for s in self.deadlocks:
            add(("deadlock", s[0], last_event(s)), lambda s=s: {"kind": "deadlock", "at": s[0], "after": last_event(s), "trace": self.trace_to(s), "example": self.describe(s)})
        for s in self.bound_reached:
            add(("bound_reached", s[0], last_event(s)), lambda s=s: {"kind": "bound_reached", "at": s[0], "after": last_event(s), "trace": self.trace_to(s), "example": self.describe(s)})
        for name, s in self.violations:
            add(("violation", name, s[0]), lambda name=name, s=s: {"kind": "violation", "invariant": name, "at": s[0], "trace": self.trace_to(s), "example": self.describe(s)})
        for cycle in self.tau_cycles:
            states = tuple(sorted({c[0] for c in cycle}))
            add(("tau_cycle", states), lambda cycle=cycle, states=states: {"kind": "tau_cycle", "through": list(states), "trace": self.trace_to(cycle[0])})
        for cycle in self.avoiding_cycles:
            states = tuple(sorted({c[0] for c in cycle}))
            add(("avoiding_cycle", states), lambda cycle=cycle, states=states: {"kind": "avoiding_cycle", "avoid": self.avoid, "through": list(states), "trace": self.trace_to(cycle[0])})
        for s, label in self.empty_posts:
            event = label.split("(")[0]
            add(("empty_post", event, s[0]), lambda s=s, event=event: {"kind": "empty_post", "event": event, "at": s[0], "trace": self.trace_to(s)})
        return list(groups.values())

    def describe(self, s: State) -> str:
        text = f"{s[0]} {[encode(v) for v in s[1]]}"
        if self.ghost_names:
            text += " " + str(dict(zip(self.ghost_names, s[2])))
        return text


class Explorer:
    """Enumerates the states a diagram can reach (docs/CLAUSE_DESIGN.md, O7).

    A step from a state takes an edge out of its control state: the fields
    the event receives range over their types in TYPES, except a field an
    equality builtin of the guard determines from the state and the fields
    determined before it, which is computed (Q5); the guard must hold; and
    the post gives the values after the step (csdfbmc.formula.solve). A sent
    field sends the value the step has bound.

    client is the glue every atom the environment does not declare is read
    through (csdfbmc.protocol.checked gives it); environment declares and
    decides the others (csdfbmc.environment). Before the walk, a declared
    function nothing gives and no seed chooses is refused, listing every
    one, and so is a declared name that names no atom of the diagram.

    avoid names an event; the walk then also reports every cycle among the
    reachable states that performs no transition of that event. Under a
    fairness assumption written as a finite bound (a resend count in the
    types), such a cycle is a livelock: a run that never gets to the event.
    within narrows those cycles to the control states named.

    checks is the checks module (csdfbmc.checks): the types, ghosts and
    invariants. A ghost update and an invariant see each instance as the map
    of its fields.
    """

    def __init__(self, diagram: Diagram, checks: Checks, client: ClientLike, *, environment: Environment,
                 max_states: int = 100_000, avoid: str | None = None, within: list[str] | None = None,
                 first: bool = False):
        self.d = diagram
        self.checks = checks
        self.environment = environment
        self.meaning = AtomMeaning(client, environment, checks.domain)
        self.ev = Evaluator(self.meaning, checks.carrier, apply_given(checks.environment))
        self.max_states = max_states
        self.avoid = avoid
        self.within = set(within) if within else None
        # first stops the walk at the first deadlock or broken invariant;
        # cycles need the whole space and are not checked then.
        self.first = first
        self.ghost_names = tuple(checks.ghosts)
        self.empty_posts: list[tuple[State, str]] = []
        self._check_fields()
        self._check_environment()
        check_independent(diagram, checks, client, environment)

    # -- before the walk ----------------------------------------------------------

    def _check_fields(self) -> None:
        """Refuses an edge that receives a field named like a variable of its
        states, over which it would bind, and one that sends a name its step
        does not bind: c!y sends a state variable before the step or a field
        the event receives."""
        for e in self.d.edges:
            for n in e.params:
                if n in self.d.vars[e.src] or n in self.d.vars[e.dst]:
                    raise KeyError(f"{e.event} at line {e.line} receives {n!r}, which names a state variable of {e.src} or {e.dst}")
            bound = set(self.d.vars[e.src]) | set(e.params)
            for n in e.sent:
                if n not in bound:
                    raise KeyError(f"{e.event} at line {e.line} sends {n!r}, which the step does not bind; it may send {sorted(bound)}")

    def _check_environment(self) -> None:
        """Refuses what the environment cannot decide: a declared function
        nothing gives and no seed chooses, listing every one; a declared name
        that names no atom of the diagram; and a function given as Python
        that does not take the atom's arguments."""
        arity = {a.function: len(a.args) for a, _, _ in self.d.atoms()}
        unknown = [n for n in self.environment.declared if n not in arity]
        if unknown:
            raise ValueError(f"the environment declares {', '.join(map(repr, sorted(unknown)))}, which names no atom of the diagram")
        for name, g in self.environment.given.items():
            if callable(g) and not _takes(g, arity[name]):
                n = arity[name]
                raise ValueError(f"{name!r} has {n} argument{'' if n == 1 else 's'}, but the function given takes other arguments; "
                                 "give a function of the arguments, in order")
        undecided = sorted(n for n in self.environment.declared if n not in self.environment.given)
        if undecided and self.environment.seed is None:
            raise ValueError(f"nothing decides the environment's functions {', '.join(map(repr, undecided))}; "
                             "give each in ENVIRONMENT or with --env NAME=true|false, or choose them with --seed N")

    # -- values ----------------------------------------------------------------

    def _before(self, state: State, params: dict | None = None) -> dict:
        sid, values, _ = state
        return {**dict(zip(self.d.vars[sid], values)), **(params or {})}

    def _ghost_env(self, state: State, params: dict | None = None) -> dict:
        env = dict(zip(self.ghost_names, state[2]))
        env.update((k, visible(v)) for k, v in self._before(state, params).items())
        return env

    def _ghosts_after(self, state: State, name: str, params: dict) -> tuple:
        env = self._ghost_env(state, params)
        out = []
        for ghost, (_, updates) in self.checks.ghosts.items():
            if name in updates:
                out.append(apply(updates[name], env, what=f"the update of ghost {ghost!r} on {name}"))
            else:
                out.append(env[ghost])
        return tuple(out)

    def _check_invariants(self, state: State) -> list[str]:
        env = self._ghost_env(state)
        return [name for name, fn in self.checks.invariants.items()
                if self.checks.binds(fn, env) and not apply(fn, env, what=f"the invariant {name!r}")]

    # -- transitions ------------------------------------------------------------

    def _successor_values(self, post: tuple, before: dict, dst: str, where: str) -> list[tuple]:
        names = self.d.vars[dst]
        return [tuple(freeze(a[n]) for n in names) for a in solve(self.ev, post, names, before, self.checks.domain, where, self.d.families)]

    def initial(self) -> list[State]:
        ghosts = tuple(init for init, _ in self.checks.ghosts.values())
        vals = self._successor_values(self.d.start_post, {}, self.d.start, "the start edge")
        return [(self.d.start, v, ghosts) for v in vals]

    def _determined(self, e: Edge, p: str, known: set[str]):
        """The term an equality builtin of e's guard determines the field p
        by, from the names known (Q5); None when none does."""
        for c in e.guard:
            if isinstance(c, Builtin) and c.op == "eq":
                for x, y in ((c.args[0], c.args[1]), (c.args[1], c.args[0])):
                    if x == Free(p) and free_names(y, primed=False) <= known and not free_names(y, primed=True):
                        return y
        return None

    def _offers(self, state: State, e: Edge) -> list[dict]:
        """The values of the fields e receives that the walk tries: each
        ranges over its type, unless the guard determines it (Q5)."""
        alive: list[dict] = [{}]
        known = set(self.d.vars[state[0]])
        for p in e.params:
            term = self._determined(e, p, known)
            grown = []
            for params in alive:
                if term is not None:
                    try:
                        grown.append(dict(params, **{p: self.ev.value(term, self._before(state, params), {}, ())}))
                    except Missing:
                        pass  # the guard reads an entry its map does not have: false
                else:
                    grown.extend(dict(params, **{p: v}) for v in self.checks.domain(p))
            alive = grown
            known.add(p)
        return alive

    def _label(self, e: Edge, before: dict) -> str:
        """The event as taken: each field, in the event's order, with the
        value received or sent."""
        if not e.fields:
            return e.name
        return e.name + "(" + ", ".join(f"{n}={encode(before[n])}" for _, n in e.fields) + ")"

    def _edge_successors(self, state: State, e: Edge, record: bool = True):
        """(params, event as taken, next states) for every value of the fields
        of e enabled from state. With record, an empty post is kept."""
        out = []
        where = e.describe()
        for params in self._offers(state, e):
            before = self._before(state, params)
            if not self.ev.holds(e.guard, before):
                continue
            label = self._label(e, before)
            vals = self._successor_values(e.post, before, e.dst, where)
            if not vals:
                if record:
                    self.empty_posts.append((state, label))
                continue
            ghosts = self._ghosts_after(state, e.name, params)
            out.append((params, label, [(e.dst, v, ghosts) for v in vals]))
        return out

    def successors(self, state: State) -> list[tuple[str, str, State, Edge]]:
        """(event name, event as taken, next state, edge) for every enabled
        transition. A transition of an edge whose post admits no successor is
        kept among the empty posts."""
        out = []
        for e in self.d.out_edges(state[0]):
            for _, label, nexts in self._edge_successors(state, e):
                for t in nexts:
                    out.append((e.name, label, t, e))
        return out

    def _enabled(self, state: State, e: Edge) -> bool:
        """Whether e has a transition from state, its ghosts left aside."""
        where = e.describe()
        return any(self.ev.holds(e.guard, before) and self._successor_values(e.post, before, e.dst, where)
                   for before in (self._before(state, params) for params in self._offers(state, e)))

    def with_types(self, types: dict) -> "Explorer":
        """This walk with the types ranging over other values: the same
        diagram, glue, environment and options."""
        return Explorer(self.d, dataclasses.replace(self.checks, types=types), self.meaning.client,
                        environment=self.environment, max_states=self.max_states, avoid=self.avoid,
                        within=sorted(self.within) if self.within is not None else None, first=self.first)

    def _wider(self, state: State) -> "Explorer":
        """This walk with one more value in each sample type, a value that
        occurs nowhere in state or the type."""
        avoid = set().union(*(scalars(v) for v in state[1])) if state[1] else set()
        return self.with_types({t: list(vs) + ([fresh(vs, avoid)] if t in self.checks.samples else [])
                                for t, vs in self.checks.types.items()})

    def bound_ends(self, state: State) -> bool:
        """Whether a state with no way out has one once each sample type has
        one more value, occurring nowhere: the bound ends it, not the
        diagram. The value is only tried; no state is added for it."""
        if not self.checks.samples:
            return False
        wider = self._wider(state)
        return wider.can_end(state) or any(wider._enabled(state, e) for e in self.d.out_edges(state[0]))

    def can_end(self, state: State) -> bool:
        return any(end.src == state[0] and self.ev.holds(end.guard, self._before(state)) for end in self.d.ends)

    # -- what a REPL asks at a value prompt ------------------------------------

    def initial_candidates(self) -> list[list]:
        """The values of the start state the start post admits, as JSON."""
        return [[encode(v) for v in s[1]] for s in self.initial()]

    def candidates(self, state_id: str, values: list, edge_index: int) -> list[dict]:
        """The values the clauses admit after the edge_index-th outgoing edge of
        state_id (in the diagram's order) from the given values, per value of
        the fields: [{"params": {...}, "values": [...]}, ...]. The ghosts are
        taken at their initial values, since a REPL does not keep them."""
        edges = self.d.out_edges(state_id)
        if edge_index < 0 or edge_index >= len(edges):
            raise IndexError(f"state {state_id!r} has {len(edges)} outgoing edge(s); no edge {edge_index}")
        if len(values) != len(self.d.vars[state_id]):
            raise ValueError(f"state {state_id!r} has {len(self.d.vars[state_id])} variable(s), got {len(values)} value(s)")
        ghosts = tuple(init for init, _ in self.checks.ghosts.values())
        state: State = (state_id, tuple(freeze(v) for v in values), ghosts)
        out = []
        for params, _, nexts in self._edge_successors(state, edges[edge_index], record=False):
            for t in nexts:
                out.append({"params": {k: encode(v) for k, v in params.items()}, "values": [encode(v) for v in t[1]]})
        return out

    # -- the walk ----------------------------------------------------------------

    def run(self) -> Result:
        self.empty_posts = []
        self.meaning.decided.clear()
        parents: dict[State, tuple[State, str, Edge] | None] = {}
        order: list[State] = []
        queue: list[State] = []
        tau_next: dict[State, list[State]] = {}
        avoid_next: dict[State, list[State]] = {}
        deadlocks: list[State] = []
        bound_reached: list[State] = []
        violations: list[tuple[str, State]] = []
        truncated = False
        seen: set[State] = set()

        def visit(s: State):
            seen.add(s)
            parents.setdefault(s, None)
            order.append(s)
            queue.append(s)
            for name in self._check_invariants(s):
                violations.append((name, s))

        for s in self.initial():
            if s not in seen:
                visit(s)

        stopped = False
        i = 0
        while i < len(queue) and not stopped:
            s = queue[i]
            i += 1
            if self.first and violations:
                stopped = True
                break
            succ = self.successors(s)
            if not succ and not self.can_end(s):
                if self.bound_ends(s):
                    bound_reached.append(s)
                else:
                    deadlocks.append(s)
                    if self.first:
                        stopped = True
                        break
            for _, label, t, edge in succ:
                if t in seen:
                    continue
                if len(order) >= self.max_states:
                    truncated = True
                    continue
                parents[t] = (s, label, edge)
                visit(t)
            tau_next[s] = [t for _, _, t, edge in succ if edge.is_tau]
            if self.avoid is not None and (self.within is None or s[0] in self.within):
                avoid_next[s] = [t for name, _, t, _ in succ if name != self.avoid and (self.within is None or t[0] in self.within)]

        return Result(
            states=order, parents=parents, deadlocks=deadlocks,
            tau_cycles=_cycles(order, tau_next) if not stopped else [],
            truncated=truncated, bound=self.max_states,
            environment={**self.environment.to_json(), "decided": list(self.meaning.decided)},
            violations=violations,
            avoiding_cycles=_cycles(order, avoid_next) if self.avoid is not None and not stopped else [],
            avoid=self.avoid, ghost_names=self.ghost_names,
            empty_posts=self.empty_posts, stopped_early=stopped, bound_reached=bound_reached,
        )


def _cycles(states: list[State], next_: dict[State, list[State]]) -> list[list[State]]:
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
        for w in next_.get(v, ()):
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
            if len(comp) > 1 or v in next_.get(v, ()):
                cycles.append(comp)

    sys.setrecursionlimit(max(sys.getrecursionlimit(), len(states) + 1000))
    for s in states:
        if s not in index:
            connect(s)
    return cycles


def _takes(g, n: int) -> bool:
    """Whether g can be called with n arguments in order."""
    try:
        inspect.signature(g).bind(*range(n))
    except TypeError:
        return False
    except ValueError:
        return True
    return True
