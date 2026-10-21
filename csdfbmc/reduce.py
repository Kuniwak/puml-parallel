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
What a clause reads is what the glue says it reads, and what it reads beyond
its own instance what the glue answers when asked beyond (a glue that does
not answer it has the clause read every instance of a map it reads); an
opaque clause, taken as true, reads nothing. A ghost update and an invariant
read what their signatures ask for.

Symmetry stores a state by one representative of its renamings of the
instance IDs the checks module declares SYMMETRIC. The walk still expands the
representative it first met, not a renamed copy, so every trace it reports
is a run of the diagram. The declaration is refused when a clause, a ghost
update or an invariant could tell two IDs apart: an ID written as a constant
(a literal listing every ID of its group, as a domain does, names none), an
ID ordered by <, <=, >, >=, sorted, min or max, or a domain holding IDs that
is not closed under renaming them. The glue answers it for the clauses; the
checks module's source is read for the rest. The check is syntactic, and it
does not follow a value through a helper.
"""
from __future__ import annotations

import ast
import inspect
import itertools
from collections.abc import Mapping
from typing import TYPE_CHECKING, Callable

from csdfbmc import formal
from csdfbmc.model import Clause, Diagram, Edge
from csdfbmc.checks import Checks
from csdfbmc.values import FrozenMap

if TYPE_CHECKING:
    from csdfbmc.meaning import Meaning

_MOVES = ("update", "create", "delete")
_INSTANCE_KINDS = ("in_dom", "not_in_dom", "at_state") + _MOVES


def _signature(fn: Callable) -> set[str]:
    return set(inspect.signature(fn).parameters)


def _with_roles(e: Edge) -> list[tuple[Clause, str]]:
    return [(c, "guard") for c in e.guard] + [(c, "post") for c in e.post]


def moved_maps(e: Edge) -> set[str]:
    return {c.map for c in e.post if c.kind in _MOVES}


class PartialOrder:
    def __init__(self, explorer):
        self.ex = explorer
        d: Diagram = explorer.d
        checks: Checks = explorer.checks
        self.names = {v for vs in d.vars.values() for v in vs} | set(checks.ghosts)
        self.invariant_reads = set().union(*(_signature(fn) for fn in checks.invariants.values()))
        self.ghost_events = {event for _, updates in checks.ghosts.values() for event in updates}
        self._independent: dict[str, set[str]] = {}

    def _clause_reads(self, c: Clause, role: str) -> set[str]:
        if c.kind in _INSTANCE_KINDS:
            return {c.map}
        if c.kind in ("frame", "dropped"):
            return set()  # a frame copies a variable; it neither enables nor changes anything
        names = self.ex.meaning.names(c, role)
        if names is None:
            return {c.map} if c.map else set()
        if c.kind == "local" and c.map:
            # A local clause reads the instance's own variables, by their local names.
            return {c.map} | (names & set(self.ex.checks.ghosts))
        return names & self.names

    def _edge_reads(self, e: Edge) -> set[str]:
        reads = set().union(*(self._clause_reads(c, role) for c, role in _with_roles(e)))
        for _, updates in self.ex.checks.ghosts.values():
            if e.name in updates:
                reads |= _signature(updates[e.name]) & self.names
        return reads

    def _shared_reads(self, e: Edge) -> set[str]:
        """What e's clauses read beyond the one instance they are about: a
        constrain or hand-written clause reading a map reads every instance
        of it, so it ties the instances together - unless the glue says it
        reads the map only as m[id], at the edge's own ID."""
        own_id = next((c.id or self.ex.d.promotes[c.map].id_param for c in e.post if c.kind in _MOVES), None)
        reads: set[str] = set()
        for c, role in _with_roles(e):
            if c.kind in _INSTANCE_KINDS or (c.kind == "local" and c.map):
                continue
            beyond = self.ex.meaning.names_beyond(c, role, own_id)
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
    def __init__(self, diagram: Diagram, checks: Checks, meaning: "Meaning"):
        if not checks.symmetric:
            raise ValueError("--symmetry needs the checks module to declare SYMMETRIC, the instance-ID parameters whose values are interchangeable")
        ids = {p.id_param for p in diagram.promotes.values()}
        self.params = set(checks.symmetric)
        self.maps = {p.map for p in diagram.promotes.values() if p.id_param in self.params}
        groups: list[frozenset] = []
        for p in checks.symmetric:
            if p not in ids:
                raise ValueError(f"SYMMETRIC names {p!r}, which is not the instance-ID parameter of any promoted map")
            values = frozenset(checks.domain(p))
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
        self._check_domains(diagram, checks)
        self._check_sources(diagram, checks)
        self._check_clauses(diagram, meaning)

    def _check_domains(self, diagram: Diagram, checks: Checks) -> None:
        """A domain that holds symmetric IDs (a record naming another
        instance) must hold every renaming of each of its values, or the
        renaming would tell the IDs apart."""
        for p, domain in checks.typed_names().items():
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

    def _check_sources(self, diagram: Diagram, checks: Checks) -> None:
        """Reads the checks module, whose ghost updates and invariants are
        Python the checker loads."""
        if not checks.sources:
            raise ValueError("--symmetry reads the checks module to check SYMMETRIC, and none was given as source")
        for path, source in checks.sources:
            self._check(ast.parse(source, filename=path), path)

    def _check_clauses(self, diagram: Diagram, meaning: "Meaning") -> None:
        """Asks the meaning of each clause the author wrote whether it tells
        two IDs apart: a glue answers it for the clauses it reads."""
        for e in diagram.edges:
            for clauses, role in ((e.guard, "guard"), (e.post, "post")):
                for c in clauses:
                    if c.kind not in ("local", "constrain"):
                        continue
                    why = meaning.tells_apart(c, role, self.params, self.maps, self.groups)
                    if why is not None:
                        raise ValueError(f"line {c.line}: the clause {c.text!r} tells the IDs apart: {why}; drop it from SYMMETRIC or from the clause")

    def _check(self, tree: ast.AST, path: str) -> None:
        found = formal.tells_apart(tree, self.params, self.maps, self.groups)
        if found is not None:
            line, how = found
            raise ValueError(f"{path}:{line}: {how}, so the IDs are not interchangeable; drop it from SYMMETRIC or from the checks module")

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
