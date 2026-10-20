"""The cone of influence: the state variables that can reach, along some
path, whether a step can be taken or what a property says.

A step can be taken when its guard holds and its post clauses admit a
successor, so both read what decides it. R is every variable read by a guard
(of any edge, the end edge included), by a post clause the author wrote, by
an invariant or by a ghost update. The built-in clauses of an expansion - a
frame, or an instance created, moved or removed - always admit exactly one
successor and only carry a value across, so they decide nothing.

A variable framed in a post that might also give it a value otherwise is in
R too, since the two must agree on it.

A variable outside R is then read by nothing that decides a step or a
property, and is only ever carried across by frames and moves or given a
value. The walk projected onto the control states and R is bisimilar to the
full one: it finds the same deadlocks, tau cycles, avoiding cycles, broken
invariants and posts that admit no successor. Every post clause is still
evaluated; only the values of the left-out variables are not kept.

What a clause reads must be known: a guard or post whose reads are unknown
(an opaque clause), or an invariant or ghost update that takes any keyword,
might read any variable, so R cannot be computed and nothing is left out.
"""
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Callable

from csdfbmc.meaning import Meaning, is_reserved_opaque
from csdfbmc.model import Clause, Diagram, Edge
from csdfbmc.twins import Twins, reads

_BUILT_IN_POSTS = ("frame", "update", "create", "delete", "empty")
_INSTANCE_GUARDS = ("in_dom", "not_in_dom", "at_state")


@dataclass(frozen=True)
class Cone:
    # relevant: the variables kept, or None when every variable is kept.
    relevant: frozenset[str] | None
    # blocked_by: what kept R from being computed, when it could not be.
    blocked_by: tuple[str, ...] = ()
    # dropped: each variable left out, with the edges whose posts set it as
    # far as writes tells (a twin says what it sets only when it runs).
    dropped: dict[str, list[str]] = field(default_factory=dict)

    @classmethod
    def keep_all(cls) -> "Cone":
        """The cone of a walk that leaves nothing out."""
        return cls(relevant=None)

    def keeps(self, var: str) -> bool:
        return self.relevant is None or var in self.relevant


def edge_label(e: Edge | None) -> str:
    return "start" if e is None else f"{e.src} -{e.name}-> {e.dst} (line {e.line})"


def _decides(meaning: Meaning, c: Clause, role: str) -> set[str] | None:
    """What a clause reads that decides whether the step can be taken; None
    when that cannot be told."""
    if c.kind in _INSTANCE_GUARDS:
        return {c.map}
    if c.kind in _BUILT_IN_POSTS or c.kind == "dropped":
        return set()
    names = meaning.names(c, role)
    if names is None:
        return None
    # A clause of a family reads the instance's own variables, in its map.
    return names | ({c.map} if c.kind == "local" and c.map else set())


# Writes: what a post clause the author wrote gives a value to, as far as
# it can be told without running it; it only names, in the report, the edges
# that set a left-out variable.
Writes = Callable[[Clause], "set[str] | None"]


def NO_WRITES(clause: Clause) -> None:
    """The writes of a meaning that cannot tell them, such as twins."""
    return None


def _sets(writes: Writes, c: Clause) -> set[str]:
    if c.kind == "frame":
        return {c.var}
    if c.kind in ("update", "create", "delete", "empty") or (c.kind == "local" and c.map):
        return {c.map}
    if c.kind == "dropped" or is_reserved_opaque(c):
        return set()
    return writes(c) or set()


def _framed_and_written(writes: Writes, post: tuple[Clause, ...]) -> set[str]:
    """The variables a post frames and might also give a value to otherwise:
    the frame and the other clause must then agree, which reads the value.
    csdfpromote frames only what nothing else on the edge sets, so this is a
    guard against a post written otherwise."""
    framed = {c.var for c in post if c.kind == "frame"}
    if not framed:
        return set()
    others = [c for c in post if c.kind != "frame"]
    out: set[str] = set()
    for c in others:
        if c.kind in ("local", "constrain") and not (c.kind == "local" and c.map) and not is_reserved_opaque(c) \
                and writes(c) is None:
            return framed  # writes unknown: it might set any of them
        out |= _sets(writes, c) & framed
    return out


def compute(d: Diagram, twins: Twins, meaning: Meaning, *, writes: Writes) -> Cone:
    variables = {v for vs in d.vars.values() for v in vs}
    relevant: set[str] = set()
    blocked: list[str] = []

    def read(names: set[str] | None, what: str) -> None:
        if names is None:
            blocked.append(what)
        else:
            relevant.update(names & variables)

    posts = [(None, d.start_post)] + [(e, e.post) for e in d.edges]
    guards = [e.guard for e in d.edges] + ([d.end.guard] if d.end is not None else [])
    for guard in guards:
        for c in guard:
            read(_decides(meaning, c, "guard"), c.text)
    for _, post in posts:
        for c in post:
            read(_decides(meaning, c, "post"), c.text)
    for _, post in posts:
        read(_framed_and_written(writes, post), "a post")
    for name, fn in twins.invariants.items():
        read(reads(fn), f"the invariant {name!r}")
    for ghost, (_, updates) in twins.ghosts.items():
        for event, fn in updates.items():
            read(reads(fn), f"the update of ghost {ghost!r} on {event}")
    if blocked:
        return Cone(relevant=None, blocked_by=tuple(dict.fromkeys(blocked)))

    dropped: dict[str, list[str]] = {v: [] for v in sorted(variables - relevant)}
    for e, post in posts:
        for c in post:
            for v in sorted(_sets(writes, c) & dropped.keys()):
                if edge_label(e) not in dropped[v]:
                    dropped[v].append(edge_label(e))
    return Cone(relevant=frozenset(relevant), dropped=dropped)
