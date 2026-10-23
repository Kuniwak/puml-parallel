"""Whether the diagram is data independent in its sample types: whether it
tells the values of a type given as a sample (csdfbmc.values.sample) apart
by nothing but =, ≠ and the keys of maps. Only then does one value that
occurs nowhere stand for every value beyond the sample, which is what the
walk asks at a state with no way out (Explorer.bound_ends).

A term carries a value of a sample type when it is a name of a sample type
or of a type some value of which holds one (a record of them, a map keyed by
them), a name an ∃ over a sample type binds, a map some clause keys by such
a term, an entry of a map that carries one, or a field of a term that does
(of an instance, a field of a sample type). The builtins tell such values
apart only by equality and membership, so every builtin may read them. An
environment function a term applies, and an atom the environment declares,
may tell them apart in any way, so a term carrying one is refused as their
argument. An atom of the glue may read one only when the glue answers the
symmetric request of the protocol (the one symmetry reduction asks, Q2 of
docs/CLAUSE_DESIGN.md) and says the atom tells no two values of the sample
apart; a glue that does not answer it cannot read one. The check is as
sound as the glue's answer: csdfbmc.python_glue's is syntactic
(csdfbmc.formal.tells_apart).
"""
from __future__ import annotations

from csdfbmc.checks import Checks
from csdfbmc.environment import Environment
from csdfbmc.model import App, Atom, Bound, Builtin, Diagram, Field, Free, Index, conjuncts, root, subterms
from csdfbmc.protocol import ClientLike, answers, shown_atom
from csdfbmc.values import scalars


class _Carrying:
    """Which terms carry a value of a sample type."""

    def __init__(self, diagram: Diagram, checks: Checks):
        self.d = diagram
        self.checks = checks
        values = set().union(*(scalars(v) for t in checks.samples for v in checks.types[t]))
        self.types = set(checks.samples) | {t for t, vs in checks.types.items()
                                            if any(scalars(v) & values for v in vs)}
        self.keyed: set[str] = set()

    def named(self, name: str) -> bool:
        return self.checks.type_of(name) in self.types or name in self.keyed

    def term(self, t, bound: tuple[bool, ...]) -> bool:
        if isinstance(t, Free):
            return self.named(t.name)
        if isinstance(t, Bound):
            return t.index < len(bound) and bound[len(bound) - 1 - t.index]
        if isinstance(t, Index):
            if isinstance(t.map, Free) and t.map.name in self.d.families:
                return any(self.named(f) for vs in self.d.families[t.map.name].vars.values() for f in vs)
            return self.term(t.map, bound)
        if isinstance(t, Field):
            if isinstance(t.of, Index) and isinstance(t.of.map, Free) and t.of.map.name in self.d.families:
                return self.named(t.name)
            return self.term(t.of, bound)
        if isinstance(t, App):
            return any(self.term(a, bound) for a in t.args)
        return False

    def key(self, m, k, bound) -> bool:
        """Records that m is keyed by k when k carries a sample value; whether
        that is news."""
        r = root(m)
        if isinstance(r, Free) and r.name not in self.keyed and self.term(k, bound):
            self.keyed.add(r.name)
            return True
        return False


def _conjuncts(clause: tuple, types: set):
    """(formula, bound) for every conjunct of clause, inside its ∃s too;
    bound says, per bound index, whether it ranges over a sample type."""
    for f, binders in conjuncts(clause):
        yield f, tuple(t in types for t in binders)


def _keys(c: _Carrying) -> None:
    """Finds every map keyed by a term carrying a sample value."""
    grown = True
    while grown:
        grown = False
        for clause, _, _ in c.d.clauses():
            for f, bound in _conjuncts(clause, c.types):
                for a in f.args:
                    for t in subterms(a):
                        if isinstance(t, Index):
                            grown |= c.key(t.map, t.key, bound)
                if isinstance(f, Builtin) and f.op in ("in", "notin"):
                    grown |= c.key(f.args[1], f.args[0], bound)
                elif isinstance(f, Builtin) and f.op in ("update", "frame"):
                    for k in (f.args[2:3] if f.op == "update" else f.args[2:]):
                        grown |= c.key(f.args[0], k, bound)
                        grown |= c.key(f.args[1], k, bound)


def check_independent(diagram: Diagram, checks: Checks, client: ClientLike, environment: Environment) -> frozenset[Atom]:
    """Refuses, naming the clause, a diagram that may tell two values of a
    sample type apart by more than =, ≠ and the keys of maps; otherwise
    the atoms that read a value of a sample type, which the glue said tell
    no two of them apart, and which alone may read a value beyond a sample
    (csdfbmc.meaning.AtomMeaning)."""
    symmetric: set[Atom] = set()
    if not checks.samples:
        return frozenset()
    c = _Carrying(diagram, checks)
    _keys(c)
    samples = ", ".join(sorted(checks.samples))
    groups = [[v for v in checks.types[t]] for t in sorted(checks.samples)]
    for clause, where, role in diagram.clauses():
        def refuse(why: str):
            raise ValueError(f"{where}, {role}: {why}; a sample type ({samples}) may be told apart only by =, ≠ and "
                             "the keys of maps, since one value occurring nowhere stands for every value beyond the sample")
        for f, bound in _conjuncts(clause, c.types):
            for a in f.args:
                for t in subterms(a):
                    if isinstance(t, App) and any(c.term(x, bound) for x in t.args):
                        refuse(f"the environment function {t.fn} reads a value of a sample type, and may tell the values apart")
            if not isinstance(f, Atom):
                continue
            carrying = [i for i, a in enumerate(f.args) if c.term(a, bound)]
            if not carrying:
                continue
            if f.function in environment.declared:
                refuse(f"the atom {shown_atom(f.wire)} is a function of the environment that reads a value of a sample type, "
                       "and may tell the values apart")
            if not answers(client, "symmetric"):
                refuse(f"the atom {shown_atom(f.wire)} reads a value of a sample type, and the {client.language} glue "
                       "does not answer symmetric, which would say whether it tells the values apart")
            maps = [i for i in carrying if isinstance(f.args[i], Free) and f.args[i].name in c.keyed]
            params = [i for i in carrying if i not in maps]
            why = client.symmetric(f.wire, params, maps, groups)
            if why is not None:
                refuse(f"the atom {shown_atom(f.wire)} tells the values apart: {why}")
            symmetric.add(f)
    return frozenset(symmetric)
