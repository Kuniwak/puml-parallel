"""Partial-order reduction: at a state, the transitions of one instance in
place of every enabled one (an ample set), when that instance moves
independently of everything else (docs/CLAUSE_DESIGN.md, Q1 and O9).

What a clause reads and writes is its free names, and an entry m[k] of a
map is told from the map as a whole, so the instances of a family are told
apart without a glue. A variable of its state that a post
does not name ranges over its type (csdfbmc.formula.solve), so the post
writes it too; only a frame @eq(x', x) keeps it. An edge moves the
instance of m at its field p alone when its post writes m and frames every
other variable of its state, every name its guard and the rest of its post
read or write is m or a field it receives, and every m they name is
m[p] or m'[p], or p in the membership @in/@notin(p, m) or @notin(p, m'),
or @frame(m', m, p). A map m is independent in a control state g when

- every edge out of g is a self-loop and none is a tau edge, so the walk
  never leaves g and no tau cycle is lost there;
- every edge out of g that writes m moves the instance of m at its field
  alone, and no ghost has an update for its event;
- no other edge out of g names m, and no invariant and no ghost update
  reads m (the visibility proviso).

A transition of an instance then commutes with every transition of another
instance and of every other edge, and none of them can enable or disable
it. The transitions of one instance are taken alone when none of them is
the --avoid event and none leads to a state already met; otherwise the
state is expanded in full. That is the cycle proviso: the walk is
breadth-first, and the step that closes a cycle leads to a state already
met, so every cycle has a state expanded in full. The reduced walk then
reaches every deadlock, every state with no way out the bound reached, and
a broken invariant, an empty post and a cycle of the same shape as the full
one.

A property whose reads are unknown, an invariant or a ghost update taking
any keyword, is refused: what it reads cannot be told.
"""
from __future__ import annotations

from typing import Callable

from csdfbmc.model import Builtin, Diagram, Edge, Exists, Free, Index, free_names, primed_roots
from csdfbmc.checks import Checks, property_reads
from csdfbmc.values import Strict


def _only_at(clause, m: str, p: str) -> bool:
    """Whether every m clause names, before or after the step, is the entry
    at p, its membership or its frame at p. Under an ∃ that binds a name
    like p, the name is the ∃'s index (Bound), never Free(p), so a key
    under it is compared as the index it is and is no entry at p."""
    key = Free(p)

    def term(t) -> bool:
        if isinstance(t, Free):
            return t.name != m
        if isinstance(t, Index):
            if isinstance(t.map, Free) and t.map.name == m:
                return t.key == key
            return term(t.map) and term(t.key)
        if hasattr(t, "of"):
            return term(t.of)
        if hasattr(t, "args"):
            return all(term(a) for a in t.args)
        return True

    def conjunct(c) -> bool:
        if isinstance(c, Exists):
            return all(conjunct(b) for b in c.body)
        if isinstance(c, Builtin):
            a = c.args
            if c.op in ("in", "notin") and isinstance(a[1], Free) and a[1].name == m:
                return a[0] == key
            if c.op == "frame" and isinstance(a[0], Free) and a[0].name == m:
                return a[0].primed and a[1] == Free(m) and tuple(a[2:]) == (key,)
        return all(term(t) for t in c.args)
    return all(conjunct(c) for c in clause)


def _frame(c) -> str | None:
    """x when c is the frame @eq(x', x) of a variable, which keeps it."""
    if isinstance(c, Builtin) and c.op == "eq":
        for x, y in ((c.args[0], c.args[1]), (c.args[1], c.args[0])):
            if isinstance(x, Free) and x.primed and y == Free(x.name):
                return x.name
    return None


def _moving(e: Edge) -> tuple:
    """The conjuncts of e's post but the frames @eq(x', x)."""
    return tuple(c for c in e.post if _frame(c) is None)


def written(e: Edge, variables: tuple[str, ...]) -> set[str]:
    """The variables of e's destination, variables, its step may change:
    those its post names after the step, but a frame @eq(x', x) alone, and
    those it does not name, which range over their types
    (csdfbmc.formula.solve)."""
    framed = {x for x in map(_frame, e.post) if x is not None}
    return primed_roots(_moving(e)) | (set(variables) - framed)


def _read(e: Edge) -> set[str]:
    """The names e's guard and post read or write, but the frames."""
    return free_names(e.guard) | free_names(_moving(e))


def own_key(e: Edge, m: str, variables: tuple[str, ...]) -> str | None:
    """The field of e whose instance of m it moves alone, every other
    variable of its destination, variables, framed; None when it moves no
    instance alone."""
    if written(e, variables) != {m} or not _read(e) <= {m} | set(e.params):
        return None
    for p in e.params:
        if _only_at(e.guard, m, p) and _only_at(_moving(e), m, p):
            return p
    return None


class PartialOrder:
    def __init__(self, d: Diagram, checks: Checks, avoid: str | None):
        self.d = d
        self.avoid = avoid
        self.property_reads = property_reads(checks, "--por")
        self.ghost_events = {event for _, updates in checks.ghosts.values() for event in updates}
        self._independent: dict[str, dict[str, dict[str, str]]] = {}

    def independent(self, g: str) -> dict[str, dict[str, str]]:
        """The maps whose instances move independently in control state g,
        each with the edges that move it, by event, and their key field."""
        if g in self._independent:
            return self._independent[g]
        edges = self.d.out_edges(g)
        out: dict[str, dict[str, str]] = {}
        if all(e.dst == g and not e.is_tau for e in edges):
            variables = self.d.vars[g]
            writes = {id(e): written(e, variables) for e in edges}
            for m in set().union(*writes.values()) - self.property_reads:
                own = [e for e in edges if m in writes[id(e)]]
                others = [e for e in edges if m not in writes[id(e)]]
                keys = {id(e): own_key(e, m, variables) for e in own}
                if (all(keys[id(e)] is not None and e.name not in self.ghost_events for e in own)
                        and all(m not in _read(e) for e in others)):
                    out[m] = {id(e): keys[id(e)] for e in own}
        self._independent[g] = out
        return out

    def ample(self, state, transitions: list, known: Callable[[object], bool]) -> list:
        """The transitions to take from state: those of one independent
        instance, or all of them. transitions are (name, label, next, edge,
        fields); known says whether a state was met already."""
        independent = self.independent(state[0])
        if not independent:
            return transitions
        groups: dict[tuple, list] = {}
        for tr in transitions:
            edge, params = tr[3], tr[4]
            for m, keys in independent.items():
                if id(edge) in keys:
                    groups.setdefault(Strict((m, params[keys[id(edge)]])), []).append(tr)
        for group in sorted(groups.values(), key=len):
            if len(group) == len(transitions):
                break
            if any(tr[0] == self.avoid or known(tr[2]) or tr[2] == state for tr in group):
                continue
            return group
        return transitions
