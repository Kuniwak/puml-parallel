"""The walk over the product of control states and valuations."""
from __future__ import annotations

import dataclasses
import functools
import inspect
import itertools
import sys
from collections.abc import Callable
from dataclasses import dataclass, field

from csdfbmc.checks import ANY, Checks, apply
from csdfbmc.environment import Environment
from csdfbmc.formula import Evaluator, Missing, PostError, solve
from csdfbmc.meaning import AtomMeaning, apply_given
from csdfbmc.model import Bound, Builtin, Diagram, Edge, Free, Lit, conjuncts, free_names
from csdfbmc.protocol import ClientLike
from csdfbmc.values import LEFT_OUT, Strict, kind_key, beyond_samples, encode, freeze, same_value, visible

# (state id, values of its variables in declaration order, values of the ghosts),
# a Strict, so that states holding 1 and True are two states.
State = Strict

# The transitions of a state the walk takes, chosen from those enabled:
# (state, its transitions, whether a state was met) -> the transitions taken.
Ample = Callable[[State, list, Callable[[State], bool]], list]


# The state a state is stored by, the same for every state the walk takes
# as one.
Canonical = Callable[[State], State]


def as_met(state: State) -> State:
    """The state itself: the walk stores every state apart."""
    return state


def every_transition(state: State, transitions: list, known: Callable[[State], bool]) -> list:
    """Takes every transition: the walk unreduced."""
    return transitions


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
    # environment: the given functions of the environment, and
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
    # bound_reached: states with no way out that a value beyond a sample
    # (csdfbmc.values.sample) gives one, with the witness, the step it
    # enables or "the end edge": the bound, not the diagram, ends them, so
    # they are not deadlocks.
    bound_reached: list[tuple[State, str]] = field(default_factory=list)
    # left_out: the variables the cone of influence left out, each kept
    # at csdfbmc.values.LEFT_OUT.
    left_out: tuple[str, ...] = ()

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
            "bound_reached": [{"witness": w, **at(s)} for s, w in self.bound_reached],
            "tau_cycles": [{"cycle": [st(s) for s in cycle], **at(cycle[0])} for cycle in self.tau_cycles],
            "violations": [{"invariant": name, **at(s)} for name, s in self.violations],
            "avoid": self.avoid,
            "avoiding_cycles": [{"cycle": [st(s) for s in cycle], **at(cycle[0])} for cycle in self.avoiding_cycles],
            "empty_posts": [{"event": label, **at(s)} for s, label in self.empty_posts],
            "environment": self.environment,
            "groups": self.groups(),
            "stopped_early": self.stopped_early,
            "left_out": list(self.left_out),
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
        for s, w in self.bound_reached:
            add(("bound_reached", s[0], last_event(s)), lambda s=s, w=w: {"kind": "bound_reached", "at": s[0], "after": last_event(s), "witness": w,
                                                                          "trace": self.trace_to(s), "example": self.describe(s)})
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
    function nothing gives is refused, listing every
    one, and so is a declared name that names no function of the diagram.

    avoid names an event; the walk then also reports every cycle among the
    reachable states that performs no transition of that event. Under a
    fairness assumption written as a finite bound (a resend count in the
    types), such a cycle is a livelock: a run that never gets to the event.
    within narrows those cycles to the control states named.

    checks is the checks module (csdfbmc.checks): the types, ghosts and
    invariants. A ghost update and an invariant see each instance as the map
    of its fields.

    left_out are the variables to leave out, those outside the cone of
    influence (csdfbmc.cone.left_out) or none: the walk keeps each at
    csdfbmc.values.LEFT_OUT.

    ample chooses, from the transitions of a state, those the walk takes
    (csdfbmc.por.PartialOrder.ample), or every_transition takes them all;
    it is given the state, its transitions and whether a state was met.

    canonical gives the state a state is stored by: one representative of
    its renamings of the symmetric IDs (csdfbmc.symmetry.Symmetry.canonical),
    which may refuse it, or as_met, the state itself.
    """

    def __init__(self, diagram: Diagram, checks: Checks, client: ClientLike, *, environment: Environment,
                 max_states: int = 100_000, avoid: str | None = None, within: list[str] | None = None,
                 first: bool = False, left_out: tuple[str, ...],
                 ample: Ample, canonical: Canonical):
        # Every keyword option as given, so that with_types passes each on,
        # one added later too.
        self._options = {k: v for k, v in locals().items() if k not in ("self", "diagram", "checks", "client")}
        self.d = diagram
        self.checks = checks
        self.environment = environment
        self.meaning = AtomMeaning(client, environment, checks.carrier)
        self.ev = Evaluator(self.meaning, checks.carrier, apply_given(environment.given))
        self.max_states = max_states
        self.avoid = avoid
        self.within = set(within) if within else None
        # first stops the walk at the first deadlock or broken invariant;
        # cycles need the whole space and are not checked then.
        self.first = first
        self.ghost_names = tuple(checks.ghosts)
        self.empty_posts: list[tuple[State, str]] = []
        self.left_out = tuple(left_out)
        self.ample = ample
        self.canonical = canonical
        self._check_fields()
        self._check_exists()
        self._check_literals()
        self._check_environment()

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

    def _check_exists(self) -> None:
        """Refuses an ∃ over the type any, whose values the walk would have
        to enumerate (docs/CLAUSE_DESIGN.md, Types)."""
        clauses = [("the start edge", self.d.start_post)] + [(e.describe(), c) for e in self.d.edges for c in (e.guard, e.post)]
        clauses += [(f"the end edge from {x.src}", x.guard) for x in self.d.ends]
        for where, clause in clauses:
            for _, types in conjuncts(clause):
                if ANY in types:
                    raise PostError(f"{where}: an ∃ ranges over the type any, which constrains nothing and has no values "
                                    "to range over; bind it over a type TYPES gives")

    def _check_literals(self) -> None:
        """Refuses @eq or @ne of a literal and a variable whose type cannot
        hold it: a state variable or a field, of the type the diagram
        declares of it, or a variable an ∃ binds, of its type, when the type
        lists every value it has and the literal is none of them. Such a
        comparison is a slip - 1.0 for 1, "1" for 1 - that would make the
        clause constant unnoticed. A type that is a sample may hold the
        literal beyond it, any holds every value, and a type TYPES does not
        declare is refused where it is needed."""
        clauses = [("the start edge", self.d.start_post, {})]
        clauses += [(e.describe(), c, {n: self.d.field_type(e, n) for _, n in e.fields}) for e in self.d.edges for c in (e.guard, e.post)]
        clauses += [(f"the end edge from {x.src}", x.guard, {}) for x in self.d.ends]
        samples = self.checks.samples
        for where, clause, fields in clauses:
            for c, types in conjuncts(clause):
                if not isinstance(c, Builtin) or c.op not in ("eq", "ne") or len(c.args) != 2:
                    continue
                for lit, other in (c.args, c.args[::-1]):
                    if not isinstance(lit, Lit):
                        continue
                    if isinstance(other, Free):
                        typed = self.d.types if other.primed or other.name not in fields else fields
                        name, t = other.name + ("'" if other.primed else ""), typed.get(other.name)
                    elif isinstance(other, Bound):
                        name, t = "the variable an ∃ binds", types[len(types) - 1 - other.index]
                    else:
                        continue
                    if t not in self.checks.types or t in samples:
                        continue
                    value = lit.value
                    if not any(same_value(freeze(v), value) for v in self.checks.types[t]):
                        raise ValueError(f"{where}: @{c.op} compares {name} with the literal {lit.text}, which no value of its type "
                                         f"{t!r} in TYPES is; write a value of the type, or add the value to it")

    def _check_environment(self) -> None:
        """Refuses what the environment cannot decide: a declared function
        nothing gives or a term applies, listing every one; a declared name
        that names no function of the diagram; and a function given as Python
        that does not take the atom's arguments."""
        applied = self.d.applied()
        arity = {**applied, **{a.function: len(a.args) for a, _, _ in self.d.atoms()}}
        unknown = [n for n in self.environment.declared if n not in arity]
        if unknown:
            raise ValueError(f"the environment declares {', '.join(map(repr, sorted(unknown)))}, which names no function of the diagram")
        for name, g in self.environment.given.items():
            if callable(g) and not _takes(g, arity[name]):
                n = arity[name]
                raise ValueError(f"{name!r} has {n} argument{'' if n == 1 else 's'}, but the function given takes other arguments; "
                                 "give a function of the arguments, in order")
        self.environment.check_decided(applied)

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
                out.append(freeze(apply(updates[name], env, what=f"the update of ghost {ghost!r} on {name}")))
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
        out: dict = {}
        for a in solve(self.ev, post, names, before, self.d.types, where, self.d.families):
            v = tuple(LEFT_OUT if n in self.left_out else freeze(a[n]) for n in names)
            out.setdefault(kind_key(v), v)
        return list(out.values())

    def initial(self) -> list[State]:
        ghosts = tuple(freeze(init) for init, _ in self.checks.ghosts.values())
        vals = self._successor_values(self.d.start_post, {}, self.d.start, "the start edge")
        return [Strict((self.d.start, v, ghosts)) for v in vals]

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
                    grown.extend(dict(params, **{p: v}) for v in self._field_values(e, p))
            alive = grown
            known.add(p)
        return alive

    def _field_values(self, e: Edge, p: str) -> list:
        """The values the field p of e ranges over: those of the type its
        channel declares of it."""
        t = self.d.field_type(e, p)
        try:
            return self.checks.carrier(t)
        except KeyError as err:
            raise PostError(f"{e.describe()}: nothing determines the field {p}, so it ranges over its type {t}: {err.args[0]}") from None

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
            out.append((params, label, [Strict((e.dst, v, ghosts)) for v in vals]))
        return out

    def transitions(self, state: State) -> list[tuple[str, str, State, Edge, dict]]:
        """(event name, event as taken, next state, edge, the values of its
        received fields) for every enabled transition. A transition of an
        edge whose post admits no successor is kept among the empty posts."""
        out = []
        for e in self.d.out_edges(state[0]):
            for params, label, nexts in self._edge_successors(state, e):
                for t in nexts:
                    out.append((e.name, label, t, e, params))
        return out

    def successors(self, state: State) -> list[tuple[str, str, State, Edge]]:
        """(event name, event as taken, next state, edge) for every enabled
        transition (transitions, without the fields)."""
        return [tr[:4] for tr in self.transitions(state)]

    def _enabling(self, state: State, e: Edge) -> str | None:
        """The first transition of e from state, as taken, its ghosts left
        aside; None when it has none."""
        where = e.describe()
        for params in self._offers(state, e):
            before = self._before(state, params)
            if self.ev.holds(e.guard, before) and self._successor_values(e.post, before, e.dst, where):
                return self._label(e, before)
        return None

    def with_types(self, types: dict, **options) -> "Explorer":
        """This walk with the types ranging over other values: the same
        diagram, glue, environment and options, but those given."""
        return Explorer(self.d, dataclasses.replace(self.checks, types=types), self.meaning.client, **{**self._options, **options})

    @functools.cached_property
    def _wider(self) -> "Explorer":
        """This walk with each sample extended by the values beyond it, all
        at once; a type built of a sample takes them too
        (csdfbmc.values.beyond_samples). Built once, at the first state with
        no way out. It keeps every option: it only tries the state the walk
        met, a run of the diagram, and stores none, so it neither chooses
        ample sets nor asks for a representative."""
        return self.with_types(beyond_samples(self.checks.types))

    def bound_witness(self, state: State) -> str | None:
        """For a state with no way out, the step the values beyond the
        samples enable, as taken, or "the end edge" when they enable it:
        a witness that a value of the real type gives the state a way out,
        so the bound ends it, not the diagram. None when they enable
        nothing, and the state is a deadlock. The values are only tried; no
        state is added for them."""
        if not self.checks.samples:
            return None
        wider = self._wider
        if wider.can_end(state):
            return "the end edge"
        return next(filter(None, (wider._enabling(state, e) for e in self.d.out_edges(state[0]))), None)

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
        ghosts = tuple(freeze(init) for init, _ in self.checks.ghosts.values())
        state = Strict((state_id, tuple(freeze(v) for v in values), ghosts))
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
        bound_reached: list[tuple[State, str]] = []
        violations: list[tuple[str, State]] = []
        truncated = False
        # met: a state as stored (its representative under symmetry) -> the
        # state the walk met first and expands.
        met: dict[State, State] = {}

        key = self.canonical

        def known(s: State) -> bool:
            return key(s) in met

        def visit(s: State):
            met[key(s)] = s
            parents.setdefault(s, None)
            order.append(s)
            queue.append(s)
            for name in self._check_invariants(s):
                violations.append((name, s))

        for s in self.initial():
            if not known(s):
                visit(s)

        stopped = False
        i = 0
        while i < len(queue) and not stopped:
            s = queue[i]
            i += 1
            if self.first and violations:
                stopped = True
                break
            succ = self.transitions(s)
            succ = self.ample(s, succ, known)
            if not succ and not self.can_end(s):
                witness = self.bound_witness(s)
                if witness is not None:
                    bound_reached.append((s, witness))
                else:
                    deadlocks.append(s)
                    if self.first:
                        stopped = True
                        break
            for _, label, t, edge, _ in succ:
                if known(t):
                    continue
                if len(order) >= self.max_states:
                    truncated = True
                    continue
                parents[t] = (s, label, edge)
                visit(t)
            tau_next[s] = [met[key(t)] for _, _, t, edge, _ in succ if edge.is_tau and known(t)]
            if self.avoid is not None and (self.within is None or s[0] in self.within):
                avoid_next[s] = [met[key(t)] for name, _, t, _, _ in succ if name != self.avoid and known(t) and (self.within is None or t[0] in self.within)]

        return Result(
            states=order, parents=parents, deadlocks=deadlocks,
            tau_cycles=_cycles(order, tau_next) if not stopped else [],
            truncated=truncated, bound=self.max_states,
            environment={**self.environment.to_json(), "decided": list(self.meaning.decided)},
            violations=violations,
            avoiding_cycles=_cycles(order, avoid_next) if self.avoid is not None and not stopped else [],
            avoid=self.avoid, ghost_names=self.ghost_names,
            empty_posts=self.empty_posts, stopped_early=stopped, bound_reached=bound_reached,
            left_out=self.left_out,
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
