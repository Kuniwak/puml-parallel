"""The walk over the product of control states and valuations."""
from __future__ import annotations

import itertools
import sys
from dataclasses import dataclass, field

from csdfbmc.cone import Cone
from csdfbmc.formal import RECORD_STATE
from csdfbmc.model import Clause, Diagram, Edge
from csdfbmc.meaning import OPAQUE, Meaning, TwinsMeaning, standard
from csdfbmc.reduce import PartialOrder, Symmetry
from csdfbmc.twins import Twins, apply
from csdfbmc.values import FrozenMap, encode, freeze

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
    # stopped_early: the walk stopped at the first finding (first=True).
    stopped_early: bool = False
    # dropped_clauses: the guard clauses csdfpromote -only listed as reading a
    # left-out map, which the walk took as true, as (text, maps).
    dropped_clauses: list[tuple[str, tuple[str, ...]]] = field(default_factory=list)
    # relevant: the variables the cone kept, in the order the diagram
    # declares them; None when every variable was kept. dropped: each one
    # left out, with the edges whose posts set it. cone_blocked_by: what
    # kept the cone from being computed.
    relevant: tuple[str, ...] | None = None
    dropped: dict[str, list[str]] = field(default_factory=dict)
    cone_blocked_by: tuple[str, ...] = ()

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
            "tau_cycles": [{"cycle": [st(s) for s in cycle], **at(cycle[0])} for cycle in self.tau_cycles],
            "violations": [{"invariant": name, **at(s)} for name, s in self.violations],
            "avoid": self.avoid,
            "avoiding_cycles": [{"cycle": [st(s) for s in cycle], **at(cycle[0])} for cycle in self.avoiding_cycles],
            "empty_posts": [{"event": label, **at(s)} for s, label in self.empty_posts],
            "unconstrained": [{"event": label, "variables": list(names), **at(s)} for s, label, names in self.unconstrained],
            "opaque_clauses": self.opaque_clauses,
            "dropped_clauses": [{"text": text, "maps": list(maps)} for text, maps in self.dropped_clauses],
            "relevant": list(self.relevant) if self.relevant is not None else None,
            "dropped_variables": self.dropped,
            "cone_blocked_by": list(self.cone_blocked_by),
            "groups": self.groups(),
            "stopped_early": self.stopped_early,
        }

    def groups(self) -> list[dict]:
        """The findings folded by shape, each with its count and the first
        (shortest) representative: deadlocks by (control state, the event
        that led there), broken invariants by (invariant, control state),
        empty and unconstrained posts by event, cycles by the set of control
        states they run through. In the order the first of each was met."""
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
        for name, s in self.violations:
            add(("violation", name, s[0]), lambda name=name, s=s: {"kind": "violation", "invariant": name, "at": s[0], "trace": self.trace_to(s), "example": self.describe(s)})
        for cycle in self.tau_cycles:
            states = tuple(sorted({c[0] for c in cycle}))
            add(("tau_cycle", states), lambda cycle=cycle, states=states: {"kind": "tau_cycle", "through": list(states), "trace": self.trace_to(cycle[0])})
        for cycle in self.avoiding_cycles:
            states = tuple(sorted({c[0] for c in cycle}))
            add(("avoiding_cycle", states), lambda cycle=cycle, states=states: {"kind": "avoiding_cycle", "avoid": self.avoid, "through": list(states), "trace": self.trace_to(cycle[0])})
        for s, label, names in self.unconstrained:
            event = label.split("(")[0]
            add(("unconstrained", event, names), lambda s=s, event=event, names=names: {"kind": "unconstrained", "event": event, "variables": list(names), "at": s[0], "trace": self.trace_to(s)})
        for s, label in self.empty_posts:
            event = label.split("(")[0]
            add(("empty_post", event, s[0]), lambda s=s, event=event: {"kind": "empty_post", "event": event, "at": s[0], "trace": self.trace_to(s)})
        return list(groups.values())

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

    meaning says what the hand-written and constrain clauses mean
    (csdfbmc.meaning): the twins (TwinsMeaning, which of_twins gives), or a
    glue of the evaluator protocol (GlueMeaning), which reads every clause
    by its text, each as meaning.standard makes it, with the tool word
    @opaque. A clause
    the meaning has nothing for follows the opaque rule. symmetry is refused
    when the meaning cannot be read to check SYMMETRIC.

    por and symmetry turn on the reductions of csdfbmc.reduce: the walk then
    meets fewer states and reports the same deadlocks, broken invariants and
    cycles. An empty or unconstrained post is reported only at the states
    the walk still meets.
    """

    @classmethod
    def of_twins(cls, diagram: Diagram, twins: Twins, **options) -> "Explorer":
        """An Explorer whose clauses mean what the twins say."""
        return cls(diagram, twins, standard(TwinsMeaning(diagram, twins)), cone=Cone.keep_all(), **options)

    def __init__(self, diagram: Diagram, twins: Twins, meaning: Meaning, *, opaque: str = "error", max_states: int = 100_000,
                 avoid: str | None = None, within: list[str] | None = None, first: bool = False,
                 por: bool = False, symmetry: bool = False, cone: Cone):
        if opaque not in ("error", "true"):
            raise ValueError(f"opaque must be 'error' or 'true', not {opaque!r}")
        self.d = diagram
        self.twins = twins
        self.opaque = opaque
        self._tau_choices: dict[tuple, tuple[str, ...]] = {}
        self.max_states = max_states
        self.avoid = avoid
        self.within = set(within) if within else None
        # first stops the walk at the first deadlock, broken invariant or
        # unconstrained post; cycles need the whole space and are not checked then.
        self.first = first
        self.opaque_seen: dict[str, None] = {}
        self.dropped_seen: dict[tuple[str, tuple[str, ...]], None] = {}
        self.ghost_names = tuple(twins.ghosts)
        self.empty_posts: list[tuple[State, str]] = []
        self.unconstrained: list[tuple[State, str, tuple[str, ...]]] = []
        # meaning: what the hand-written and constrain clauses mean.
        self.meaning = meaning
        # cone says which variables the walk keeps (csdfbmc.cone).
        self.cone = cone
        if not self.meaning.by_prose and (twins.clauses or twins.locals):
            raise ValueError("the clauses are read through a glue, so the twins' CLAUSES and their locals would not be used; leave them out")
        self.por = PartialOrder(self) if por else None
        if symmetry and not self.meaning.readable:
            raise ValueError("symmetry is checked by reading the clauses for SYMMETRIC, which cannot be done for clauses read through a glue")
        self.symmetry = Symmetry(diagram, twins) if symmetry else None

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

    def _missing_twin(self, clause: Clause) -> bool:
        """Records an opaque clause, or refuses it; True when it is to be taken as true."""
        if self.opaque == "error":
            raise KeyError(self.meaning.opaque_message(clause))
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
        if c.kind == "dropped":
            self.dropped_seen.setdefault((c.text, c.maps))
            return True
        if c.kind not in ("local", "constrain"):
            raise ValueError(f"unknown clause kind {c.kind!r} in a guard")
        scope = self._instance_env(c, env) if c.map and c.kind == "local" else env
        value = self.meaning.evaluate(c, "guard", scope)
        if value is OPAQUE:
            return self._missing_twin(c)
        return bool(value)

    # -- posts ------------------------------------------------------------------

    def _assignments(self, c: Clause, env: dict) -> list[dict] | None:
        """What a post twin returns, as frozen assignments; None when opaque."""
        returned = self.meaning.evaluate(c, "post", env)
        if returned is OPAQUE:
            self._missing_twin(c)
            return None
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
            if c.kind == "empty":
                joined = self._join(joined, [{c.map: FrozenMap() if self.cone.keeps(c.map) else None}])
            elif c.kind == "frame":
                # A left-out variable has no value to carry across.
                joined = self._join(joined, [{c.var: env[c.var] if self.cone.keeps(c.var) else None}])
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
            if not self.cone.keeps(m):
                # A left-out map has no local post clauses, since they read
                # it; a move into a state with variables then leaves them
                # without a value, as in the full walk. Otherwise the move
                # admits one successor, and the map has no value.
                names = self.d.local_of(move).vars_of(move.state) if move.kind != "delete" else ()
                if names:
                    return [], tuple(f"{m}[{idv}].{n}" for n in names)
                joined = self._join(joined, [{m: None}])
                continue
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
        # A variable left out of the cone has no value in the state; the
        # successors that differ only in it are one.
        projected = dict.fromkeys(tuple(j[n] if self.cone.keeps(n) else None for n in names) for j in joined)
        return list(projected), ()

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

    def _edge_successors(self, state: State, e: Edge, record: bool = True):
        """(params, event as taken, next states) for every parameter valuation
        of e that is enabled from state. With record, what the walk reports
        (unconstrained and empty posts) is kept."""
        names = e.params + e.implicit_ids() + self._chosen_by_tau(e)
        out = []
        for params in self._enabled(state, e, names):
            label = e.name if not names else e.name + "(" + ", ".join(f"{p}={v}" for p, v in params.items()) + ")"
            ghosts = self._ghosts_after(state, e.name, params)
            vals, missing = self._successor_values(e.post, self._env(state, params), e.dst)
            if missing:
                if record:
                    self.unconstrained.append((state, label, missing))
                continue
            if not vals:
                if record:
                    self.empty_posts.append((state, label))
                continue
            out.append((params, label, [(e.dst, v, ghosts) for v in vals]))
        return out

    def _chosen_by_tau(self, e: Edge) -> tuple[str, ...]:
        """The values a tau chooses: the names its clauses read that are not
        state variables (an instance's own among them), ghosts or fields but
        have a domain. Hiding c?x leaves such a tau, which is the internal
        choice of x over its domain; a name with no domain is not chosen, and
        is refused when read. A tau names what it chose in its label, as
        tau(x=1), as a promoted tau names the instance it chose.

        A tau the author wrote is read by the same rule, so a name that has a
        domain is chosen whether hiding left it or not."""
        if not e.is_tau:
            return ()
        key = (e.src, e.dst, e.line, e.guard, e.post)
        if key not in self._tau_choices:
            bound = set(self.d.vars.get(e.src, ())) | set(self.d.vars.get(e.dst, ())) | set(self.ghost_names) | set(e.params) | set(e.implicit_ids())
            chosen: dict[str, None] = {}
            for role, clauses in (("guard", e.guard), ("post", e.post)):
                for c in clauses:
                    if c.kind not in ("local", "constrain"):
                        continue
                    own = set()
                    if c.kind == "local" and c.map:
                        # A family's clause reads the instance's own variables.
                        own = {v for vs in self.d.local_of(c).vars.values() for v in vs}
                    for n in sorted(self.meaning.names(c, role) or ()):
                        if n not in bound and n not in own and n in self.twins.domains:
                            chosen.setdefault(n)
            self._tau_choices[key] = tuple(chosen)
        return self._tau_choices[key]

    def _values(self, e: Edge, name: str) -> list:
        """The values e's field name ranges over: its domain, and of those
        only the ones every name renamed onto it can also take. A composition
        renames a side's field onto the other's, and the two synchronise only
        on a value both sides can take."""
        values = self.twins.domain(name)
        for c in e.guard + e.post:
            for f, t in c.rename:
                if t != name:
                    continue
                try:
                    held = self.twins.domain(f)
                except KeyError:
                    continue  # a name with no domain of its own takes any value
                values = [v for v in values if v in held]
        return values

    def _enabled(self, state: State, e: Edge, names: tuple[str, ...]) -> list[dict]:
        """The parameter valuations of e whose guard holds at state. The
        guard is taken clause by clause over every valuation still enabled,
        so a clause read through a glue costs one request per edge."""
        alive = [(params, self._env(state, params))
                 for params in (dict(zip(names, args)) for args in itertools.product(*(self._values(e, p) for p in names)))]
        for c in e.guard:
            if c.kind in ("local", "constrain"):
                self.meaning.prefetch(c, "guard", [self._instance_env(c, env) if c.map and c.kind == "local" else env for _, env in alive])
            alive = [(params, env) for params, env in alive if self._guard_holds(c, env)]
        return [params for params, _ in alive]

    def successors(self, state: State) -> list[tuple[str, str, State, Edge]]:
        """(event name, event as taken, next state, edge) for every enabled transition."""
        return [tr[:4] for tr in self._transitions(state)]

    def _transitions(self, state: State) -> list[tuple[str, str, State, Edge, dict]]:
        """successors, each with the parameter valuation it was taken with."""
        out = []
        for e in self.d.out_edges(state[0]):
            for params, label, nexts in self._edge_successors(state, e):
                for t in nexts:
                    out.append((e.name, label, t, e, params))
        return out

    # -- what a REPL asks at a value prompt ------------------------------------

    def initial_candidates(self) -> list[list]:
        """The values of the start state the start post admits, as JSON."""
        return [[encode(v) for v in s[1]] for s in self.initial()]

    def candidates(self, state_id: str, values: list, edge_index: int) -> list[dict]:
        """The values the twins admit after the edge_index-th outgoing edge of
        state_id (in the diagram's order) from the given values, per parameter
        valuation: [{"params": {...}, "values": [...]}, ...]. The ghosts are
        taken at their initial values, since a REPL does not keep them."""
        edges = self.d.out_edges(state_id)
        if edge_index < 0 or edge_index >= len(edges):
            raise IndexError(f"state {state_id!r} has {len(edges)} outgoing edge(s); no edge {edge_index}")
        if len(values) != len(self.d.vars[state_id]):
            raise ValueError(f"state {state_id!r} has {len(self.d.vars[state_id])} variable(s), got {len(values)} value(s)")
        ghosts = tuple(init for init, _ in self.twins.ghosts.values())
        state: State = (state_id, tuple(freeze(v) for v in values), ghosts)
        out = []
        for params, _, nexts in self._edge_successors(state, edges[edge_index], record=False):
            for t in nexts:
                out.append({"params": {k: encode(v) for k, v in params.items()}, "values": [encode(v) for v in t[1]]})
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
        # seen: the states met, each by its representative under symmetry, to
        # the state the walk met first and expands.
        seen: dict[State, State] = {}

        def key(s: State) -> State:
            return self.symmetry.canonical(s) if self.symmetry else s

        def met(s: State) -> State | None:
            return seen.get(key(s))

        def visit(s: State):
            seen[key(s)] = s
            parents.setdefault(s, None)
            order.append(s)
            queue.append(s)
            for name in self._check_invariants(s):
                violations.append((name, s))

        for s in self.initial():
            if met(s) is None:
                visit(s)

        stopped = False
        i = 0
        while i < len(queue) and not stopped:
            s = queue[i]
            i += 1
            if self.first and (violations or self.unconstrained):
                stopped = True
                break
            succ = self._transitions(s)
            if not succ and not self.can_end(s):
                deadlocks.append(s)
                if self.first:
                    stopped = True
                    break
            if self.por:
                succ = self.por.ample(s, succ, lambda t: met(t) is not None)
            for _, label, t, edge, _ in succ:
                if met(t) is not None:
                    continue
                if len(order) >= self.max_states:
                    truncated = True
                    continue
                parents[t] = (s, label, edge)
                visit(t)
            nexts = [(name, met(t) or t) for name, _, t, _, _ in succ]
            tau_next[s] = [t for name, t in nexts if name == "tau"]
            if self.avoid is not None and (self.within is None or s[0] in self.within):
                avoid_next[s] = [t for name, t in nexts if name != self.avoid and (self.within is None or t[0] in self.within)]

        return Result(
            states=order, parents=parents, deadlocks=deadlocks,
            tau_cycles=_cycles(order, tau_next) if not stopped else [],
            opaque_clauses=list(self.opaque_seen), truncated=truncated, bound=self.max_states,
            violations=violations,
            avoiding_cycles=_cycles(order, avoid_next) if self.avoid is not None and not stopped else [],
            avoid=self.avoid, ghost_names=self.ghost_names,
            empty_posts=self.empty_posts, unconstrained=self.unconstrained,
            stopped_early=stopped, dropped_clauses=list(self.dropped_seen),
            relevant=self._relevant_in_order(), dropped=self.cone.dropped, cone_blocked_by=self.cone.blocked_by,
        )

    def _relevant_in_order(self) -> tuple[str, ...] | None:
        if self.cone.relevant is None:
            return None
        declared = dict.fromkeys(v for vs in self.d.vars.values() for v in vs)
        return tuple(v for v in declared if v in self.cone.relevant)


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
