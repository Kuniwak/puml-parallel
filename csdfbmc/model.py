"""The diagram as csdfparse prints it (core.DiagramDoc, version 1): its
clauses are formulas, a skeleton of ∧, ∃ and builtins over terms, with
atoms a glue or the environment gives the meaning of (docs/CLAUSE_DESIGN.md).

A clause is a tuple of its conjuncts. A variable bound by ∃ is a de Bruijn
index (Bound): 0 is the nearest enclosing ∃.
"""
from __future__ import annotations

import json
from dataclasses import dataclass

VERSION = 1

RECEIVED, SENT = "?", "!"
TAU = "tau"


@dataclass(frozen=True)
class Free:
    """A free variable: a name of the diagram, before the step or, when
    primed, after it."""
    name: str
    primed: bool = False


@dataclass(frozen=True)
class Bound:
    index: int


@dataclass(frozen=True)
class Index:
    """The entry of map at key, m[k]."""
    map: "Term"
    key: "Term"


@dataclass(frozen=True)
class Field:
    """The field name of of, t.f."""
    of: "Term"
    name: str


@dataclass(frozen=True)
class App:
    """An environment function applied, f(t, …)."""
    fn: str
    args: tuple


Term = Free | Bound | Index | Field | App


@dataclass(frozen=True)
class Atom:
    """An atom as a glue receives it, text and holes and which arguments are
    values after the step; the name of the environment function it denotes;
    and the terms its arguments stand for."""
    text: tuple[str, ...]
    holes: tuple[int, ...]
    primed: tuple[bool, ...]
    function: str
    args: tuple

    @property
    def wire(self) -> dict:
        return {"text": list(self.text), "holes": list(self.holes), "primed": list(self.primed)}


@dataclass(frozen=True)
class Builtin:
    """A fact a tool states: op is the builtin's word without @, state the
    state of @at."""
    op: str
    args: tuple
    state: str = ""


@dataclass(frozen=True)
class Exists:
    """∃ over the values of type, binding the bound index 0 in body."""
    type: str
    body: tuple


Formula = Atom | Builtin | Exists

BUILTIN_ARITY = {"eq": 2, "ne": 2, "in": 2, "notin": 2, "empty": 1, "update": 4, "at": 1}


def root(t: Term) -> Term:
    """The term a path starts from."""
    while isinstance(t, (Index, Field)):
        t = t.map if isinstance(t, Index) else t.of
    return t


def is_primed(t: Term) -> bool:
    """Whether t is a value after the step: its root is a primed variable."""
    r = root(t)
    return isinstance(r, Free) and r.primed


def free_names(t, primed: bool | None = None) -> set[str]:
    """The free variables of a term or formula, those primed or not when
    primed says which."""
    out: set[str] = set()

    def walk(x):
        if isinstance(x, Free):
            if primed is None or x.primed == primed:
                out.add(x.name)
        elif isinstance(x, Index):
            walk(x.map)
            walk(x.key)
        elif isinstance(x, Field):
            walk(x.of)
        elif isinstance(x, (App, Atom, Builtin)):
            for a in x.args:
                walk(a)
        elif isinstance(x, Exists):
            for c in x.body:
                walk(c)
        elif isinstance(x, tuple):
            for c in x:
                walk(c)
    walk(t)
    return out


def _fields(doc: dict, what: str, required: tuple[str, ...], optional: tuple[str, ...] = ()) -> dict:
    """doc, refused unless it has every key required and no key beyond
    those and optional: the JSON is what csdfparse writes, and nothing else
    is read."""
    if not isinstance(doc, dict):
        raise ValueError(f"{what} is {doc!r}, not an object")
    missing = [k for k in required if k not in doc]
    unknown = sorted(set(doc) - set(required) - set(optional))
    if missing or unknown:
        raise ValueError(f"{what} {doc!r} lacks {missing} or has keys it does not read, {unknown}")
    return doc


_TERMS = {"free": (("name",), ("primed",)), "bound": (("index",), ()), "index": (("map", "key"), ()),
          "field": (("of", "name"), ()), "app": (("fn", "args"), ())}
_CONJUNCTS = {"atom": (("pieces", "holes", "args", "text", "primed", "function"), ()),
              "builtin": (("op", "args"), ("state",)), "exists": (("type", "body"), ())}


def _kind(doc: dict, kinds: dict, what: str) -> str:
    kind = doc.get("kind") if isinstance(doc, dict) else None
    if kind not in kinds:
        raise ValueError(f"{what} of the kind {kind!r}, which is none of {', '.join(kinds)}")
    required, optional = kinds[kind]
    _fields(doc, f"{what} of the kind {kind}", ("kind",) + required, optional)
    return kind


def term_of(doc: dict) -> Term:
    kind = _kind(doc, _TERMS, "a term")
    if kind == "free":
        return Free(doc["name"], doc.get("primed", False))
    if kind == "bound":
        return Bound(doc["index"])
    if kind == "index":
        return Index(term_of(doc["map"]), term_of(doc["key"]))
    if kind == "field":
        return Field(term_of(doc["of"]), doc["name"])
    return App(doc["fn"], tuple(term_of(a) for a in doc["args"]))


def formula_of(docs: list) -> tuple:
    """The clause whose conjuncts docs are."""
    return tuple(_conjunct(d) for d in docs)


def _conjunct(doc: dict) -> Formula:
    kind = _kind(doc, _CONJUNCTS, "a conjunct")
    if kind == "atom":
        return Atom(text=tuple(doc["text"]), holes=tuple(doc["holes"]), primed=tuple(doc["primed"]),
                    function=doc["function"], args=tuple(term_of(a) for a in doc["args"]))
    if kind == "builtin":
        return Builtin(doc["op"], tuple(term_of(a) for a in doc["args"]), doc.get("state", ""))
    return Exists(doc["type"], formula_of(doc["body"]))


@dataclass(frozen=True)
class Edge:
    src: str
    dst: str
    name: str                            # the channel
    fields: tuple[tuple[str, str], ...]  # (direction, name), in the event's order
    event: str                           # the label, channel?x!y
    guard: tuple
    post: tuple
    line: int

    @property
    def params(self) -> tuple[str, ...]:
        """The received fields, c?x: values the environment offers."""
        return tuple(dict.fromkeys(n for d, n in self.fields if d == RECEIVED))

    @property
    def sent(self) -> tuple[str, ...]:
        """The sent fields, c!y: values the step has bound."""
        return tuple(n for d, n in self.fields if d == SENT)

    @property
    def is_tau(self) -> bool:
        return self.name == TAU


@dataclass(frozen=True)
class End:
    src: str
    guard: tuple


@dataclass
class Diagram:
    name: str
    states: list[str]
    labels: dict[str, str]
    vars: dict[str, tuple[str, ...]]   # state -> its variables, in declaration order
    start: str
    start_post: tuple
    edges: list[Edge]
    ends: list[End]
    assumptions: list[dict]

    @classmethod
    def from_json(cls, d: dict) -> "Diagram":
        if not isinstance(d, dict) or d.get("version") != VERSION:
            raise ValueError(f"the diagram JSON is version {d.get('version') if isinstance(d, dict) else None!r}, but csdfbmc reads "
                             f"version {VERSION}; write it with csdfparse (or csdfpromote -format json) of this release")
        _fields(d, "the diagram", ("version", "name", "states", "labels", "vars", "init", "init_post", "init_line", "edges", "ends", "assumes"))
        edges = []
        for e in d["edges"]:
            _fields(e, "an edge", ("src", "dst", "channel", "fields", "label", "guard", "post", "line"))
            fields = tuple((f["dir"], f["name"]) for f in (_fields(f, "a field", ("dir", "name")) for f in e["fields"]))
            edges.append(Edge(src=e["src"], dst=e["dst"], name=e["channel"], fields=fields, event=e["label"],
                              guard=formula_of(e["guard"]), post=formula_of(e["post"]), line=e["line"]))
        ends = [End(x["src"], formula_of(x["guard"])) for x in (_fields(x, "an end edge", ("src", "guard")) for x in d["ends"])]
        return cls(name=d["name"], states=list(d["states"]), labels=dict(d["labels"]),
                   vars={s: tuple(d["vars"][s]) for s in d["states"]}, start=d["init"], start_post=formula_of(d["init_post"]),
                   edges=edges, ends=ends, assumptions=[_fields(a, "an assume note", ("id", "text")) for a in d["assumes"]])

    @classmethod
    def load(cls, path: str) -> "Diagram":
        with open(path, encoding="utf-8") as f:
            return cls.from_json(json.load(f))

    def out_edges(self, sid: str) -> list[Edge]:
        return [e for e in self.edges if e.src == sid]

    def atoms(self) -> list[tuple[Atom, Edge | None, str]]:
        """Every atom of the diagram, with its edge (None for the start and
        end edges) and the role of its clause, guard or post."""
        out = []

        def walk(fs, edge, role):
            for f in fs:
                if isinstance(f, Atom):
                    out.append((f, edge, role))
                elif isinstance(f, Exists):
                    walk(f.body, edge, role)
        walk(self.start_post, None, "post")
        for e in self.edges:
            walk(e.guard, e, "guard")
            walk(e.post, e, "post")
        for e in self.ends:
            walk(e.guard, None, "guard")
        return out
