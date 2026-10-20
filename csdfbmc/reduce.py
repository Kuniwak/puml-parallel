"""Partial-order and symmetry reduction of the walk.

Both are opt-in, so a finding can always be confirmed without them.

PartialOrder picks, at a state, the transitions of one instance in place of
all the enabled ones (an ample set), when that instance moves independently
of everything else. A map m is independent in a control state g when:

- every edge out of g is a self-loop, so the walk never leaves g, and no edge
  out of g is a tau edge, so there is no tau cycle there to lose;
- every edge out of g that moves m moves m alone, updates no ghost, reads no
  state variable or ghost but m, and reads m only through clauses about its
  own instance (a constrain such as len(m) < 2 ties the instances together);
- no other edge out of g, and no ghost update of its event, reads m, and no
  invariant reads m (the visibility proviso).

A transition of an instance then commutes with every transition of another,
and none can enable or disable it. The transitions of one instance are taken
alone when none of them is the --avoid event and none leads to a state
already met; otherwise the state is expanded in full (the cycle proviso,
which in a breadth-first walk puts a fully expanded state on every cycle).
What a clause reads is what its twin's signature asks for, the names of its
formal expression, or what the glue says it reads; an opaque clause, taken as true, reads nothing. A twin
is taken to read only what it names: one that reaches a map through a helper
defeats the analysis.

Symmetry stores a state by one representative of its renamings of the
instance IDs the twins module declares SYMMETRIC. The walk still expands the
representative it first met, not a renamed copy, so every trace it reports
is a run of the diagram. The declaration is refused when the twins could tell
two IDs apart: an ID written as a constant (a literal listing every ID of its
group, as a domain does, names none), an ID ordered by <, <=, >, >=, sorted,
min or max, or a domain holding IDs that is not closed under renaming them.
The check is syntactic: it reads the twins modules and the formal
expressions, and it does not follow a value through a helper.
"""
from __future__ import annotations

import ast
import inspect
import itertools
from collections.abc import Mapping
from typing import Callable

from csdfbmc.model import Clause, Diagram, Edge
from csdfbmc.twins import Twins
from csdfbmc.values import FrozenMap

_MOVES = ("update", "create", "delete")
_INSTANCE_KINDS = ("in_dom", "not_in_dom", "at_state") + _MOVES


def _signature(fn: Callable) -> set[str]:
    return set(inspect.signature(fn).parameters)


def formal_names(expr: str) -> set[str]:
    return {n.id for n in ast.walk(ast.parse(expr, mode="eval")) if isinstance(n, ast.Name)}


def names_beyond(expr: str, own_id: str | None) -> set[str]:
    """The names expr reads, but a map read only as m[own_id]."""
    tree = ast.parse(expr, mode="eval")
    keyed = {id(n.value) for n in ast.walk(tree)
             if isinstance(n, ast.Subscript) and isinstance(n.value, ast.Name)
             and isinstance(n.slice, ast.Name) and n.slice.id == own_id}
    return {n.id for n in ast.walk(tree) if isinstance(n, ast.Name) and id(n) not in keyed}


def _with_roles(e: Edge) -> list[tuple[Clause, str]]:
    return [(c, "guard") for c in e.guard] + [(c, "post") for c in e.post]


def moved_maps(e: Edge) -> set[str]:
    return {c.map for c in e.post if c.kind in _MOVES}


class PartialOrder:
    def __init__(self, explorer):
        self.ex = explorer
        d: Diagram = explorer.d
        twins: Twins = explorer.twins
        self.names = {v for vs in d.vars.values() for v in vs} | set(twins.ghosts)
        self.invariant_reads = set().union(*(_signature(fn) for fn in twins.invariants.values()))
        self.ghost_events = {event for _, updates in twins.ghosts.values() for event in updates}
        self._independent: dict[str, set[str]] = {}

    def _clause_reads(self, c: Clause, role: str) -> set[str]:
        if c.kind in _INSTANCE_KINDS:
            return {c.map}
        if c.kind in ("frame", "dropped"):
            return set()  # a frame copies a variable; it neither enables nor changes anything
        names = self.ex.clause_names(c, role)
        if names is None:
            return {c.map} if c.map else set()
        if c.kind == "local" and c.map:
            # A local twin reads the instance's own variables, by their local names.
            return {c.map} | (names & set(self.ex.twins.ghosts))
        return names & self.names

    def _edge_reads(self, e: Edge) -> set[str]:
        reads = set().union(*(self._clause_reads(c, role) for c, role in _with_roles(e)))
        for _, updates in self.ex.twins.ghosts.values():
            if e.name in updates:
                reads |= _signature(updates[e.name]) & self.names
        return reads

    def _shared_reads(self, e: Edge) -> set[str]:
        """What e's clauses read beyond the one instance they are about: a
        constrain or hand-written clause reading a map reads every instance
        of it, so it ties the instances together - unless it is a formal
        expression that reads the map only as m[id], at the edge's own ID."""
        own_id = next((c.id or self.ex.d.promotes[c.map].id_param for c in e.post if c.kind in _MOVES), None)
        reads: set[str] = set()
        for c, role in _with_roles(e):
            if c.kind in _INSTANCE_KINDS or (c.kind == "local" and c.map):
                continue
            beyond = self.ex.clause_names_beyond(c, own_id)
            if beyond is not None:
                reads |= beyond & self.names
            else:
                reads |= self._clause_reads(c, role)
        return reads

    def independent(self, g: str) -> set[str]:
        """The maps whose instances move independently in control state g."""
        if g in self._independent:
            return self._independent[g]
        edges = self.ex.d.out_edges(g)
        out: set[str] = set()
        if all(e.dst == g and not e.is_tau for e in edges):
            for m in set().union(*(moved_maps(e) for e in edges)):
                own = [e for e in edges if m in moved_maps(e)]
                others = [e for e in edges if m not in moved_maps(e)]
                if (m not in self.invariant_reads
                        and all(moved_maps(e) == {m} and e.name not in self.ghost_events and self._edge_reads(e) <= {m}
                                and not self._shared_reads(e) for e in own)
                        and all(m not in self._edge_reads(e) for e in others)):
                    out.add(m)
        self._independent[g] = out
        return out

    def ample(self, state, transitions: list, known: Callable[[object], bool]) -> list:
        """The transitions to take from state: those of one independent
        instance, or all of them. transitions are (name, label, next, edge,
        params); known says whether a state was met already."""
        independent = self.independent(state[0])
        if not independent:
            return transitions
        groups: dict[tuple, list] = {}
        for tr in transitions:
            edge, params = tr[3], tr[4]
            moved = moved_maps(edge)
            if len(moved) != 1 or not moved <= independent:
                continue
            (m,) = moved
            move = next(c for c in edge.post if c.kind in _MOVES)
            groups.setdefault((m, params[move.id or self.ex.d.promotes[m].id_param]), []).append(tr)
        for group in sorted(groups.values(), key=len):
            if len(group) == len(transitions):
                break
            if any(tr[0] == self.ex.avoid for tr in group):
                continue
            if any(known(tr[2]) for tr in group):
                continue
            return group
        return transitions


class Symmetry:
    def __init__(self, diagram: Diagram, twins: Twins):
        if not twins.symmetric:
            raise ValueError("--symmetry needs the twins module to declare SYMMETRIC, the instance-ID parameters whose values are interchangeable")
        ids = {p.id_param for p in diagram.promotes.values()}
        self.params = set(twins.symmetric)
        self.maps = {p.map for p in diagram.promotes.values() if p.id_param in self.params}
        groups: list[frozenset] = []
        for p in twins.symmetric:
            if p not in ids:
                raise ValueError(f"SYMMETRIC names {p!r}, which is not the instance-ID parameter of any promoted map")
            values = frozenset(twins.domain(p))
            if not all(isinstance(v, str) for v in values):
                raise ValueError(f"SYMMETRIC names {p!r}, whose values are not all strings; an ID that is a number cannot be told from a count")
            for g in groups:
                if g & values and g != values:
                    raise ValueError(f"the domain of {p!r} overlaps another symmetric domain without being the same")
            if values not in groups:
                groups.append(values)
        self.groups = groups
        self.values = frozenset().union(*groups)
        self.perms = [dict(itertools.chain.from_iterable(zip(sorted(g), perm) for g, perm in zip(groups, combo)))
                      for combo in itertools.product(*(itertools.permutations(sorted(g)) for g in groups))]
        self._check_domains(diagram, twins)
        self._check_sources(diagram, twins)

    def _check_domains(self, diagram: Diagram, twins: Twins) -> None:
        """A domain that holds symmetric IDs (a record naming another
        instance) must hold every renaming of each of its values, or the
        renaming would tell the IDs apart."""
        for t in [twins, *twins.locals.values()]:
            for p, domain in t.domains.items():
                if p in self.params:
                    continue
                values = set(domain)
                for perm in self.perms:
                    if {_rename(v, perm) for v in values} != values:
                        raise ValueError(f"the domain of {p!r} is not closed under renaming the symmetric IDs, so they are not interchangeable")
        for local in diagram.locals.values():
            shared = self.values & set(local.vars)
            if shared:
                raise ValueError(f"{local.path} has local states named {sorted(shared)}, which are also symmetric IDs")

    def _check_sources(self, diagram: Diagram, twins: Twins) -> None:
        sources = [*twins.sources, *(s for t in twins.locals.values() for s in t.sources)]
        if not sources:
            raise ValueError("--symmetry reads the twins modules to check SYMMETRIC, and none was given as source")
        for path, source in sources:
            self._check(ast.parse(source, filename=path), path)
        for e in diagram.edges:
            for c in e.guard + e.post:
                if c.formal:
                    self._check(ast.parse(c.formal, mode="eval"), f"<formal line {c.line}>")

    def _check(self, tree: ast.AST, path: str) -> None:
        # A literal listing every ID of a group (the domain, written out)
        # names none of them in particular.
        skip = set()
        for node in ast.walk(tree):
            if isinstance(node, (ast.List, ast.Tuple, ast.Set)):
                listed = {e.value for e in node.elts if isinstance(e, ast.Constant)}
                if any(g <= listed for g in self.groups):
                    skip |= set(map(id, node.elts))
        symmetric = self.params | self.maps
        for node in ast.walk(tree):
            if id(node) in skip:
                continue
            line = getattr(node, "lineno", 0)
            if isinstance(node, ast.Constant) and isinstance(node.value, str) and node.value in self.values:
                raise ValueError(f"{path}:{line}: the constant {node.value!r} names one symmetric ID, so the IDs are not interchangeable; drop it from SYMMETRIC or from the twin")
            if isinstance(node, ast.Compare) and any(isinstance(op, (ast.Lt, ast.LtE, ast.Gt, ast.GtE)) for op in node.ops):
                # An ID used as a key (m[id].x < 3) is not itself ordered.
                for operand in [node.left, *node.comparators]:
                    if isinstance(operand, ast.Name) and operand.id in self.params:
                        raise ValueError(f"{path}:{line}: {operand.id} is ordered, so the IDs are not interchangeable; drop it from SYMMETRIC or from the twin")
            if isinstance(node, ast.Call) and isinstance(node.func, ast.Name) and node.func.id in ("sorted", "min", "max"):
                for n in ast.walk(node):
                    if isinstance(n, ast.Name) and n.id in symmetric:
                        raise ValueError(f"{path}:{line}: {node.func.id} orders {n.id}, so the IDs are not interchangeable; drop it from SYMMETRIC or from the twin")

    def canonical(self, state):
        """The representative of state's renamings: the least under a fixed
        order of values."""
        sid, values, ghosts = state
        best, best_key = None, None
        for perm in self.perms:
            renamed = (_rename(values, perm), _rename(ghosts, perm))
            key = _order_key(renamed)
            if best_key is None or key < best_key:
                best, best_key = renamed, key
        return (sid, best[0], best[1])


def _rename(v, perm: dict):
    if isinstance(v, FrozenMap):
        return FrozenMap({_rename(k, perm): _rename(x, perm) for k, x in v.items()})
    if isinstance(v, tuple):
        return tuple(_rename(x, perm) for x in v)
    if isinstance(v, frozenset):
        return frozenset(_rename(x, perm) for x in v)
    if isinstance(v, str):
        return perm.get(v, v)
    return v


def _order_key(v):
    if isinstance(v, Mapping):
        return (3, tuple(sorted((_order_key(k), _order_key(x)) for k, x in v.items())))
    if isinstance(v, tuple):
        return (4, tuple(_order_key(x) for x in v))
    if isinstance(v, frozenset):
        return (5, tuple(sorted(_order_key(x) for x in v)))
    return (0, type(v).__name__, repr(v))
