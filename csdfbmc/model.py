"""The diagram as csdfparse prints it, or an expansion as csdfpromote
-format json prints it, with the clauses of each predicate."""
from __future__ import annotations

import json
from dataclasses import dataclass, field

# The tool word of a clause not formalised yet (docs/SYNTAX.md), which stands
# for the clause where it has no text, in a report.
OPAQUE_WORD = "@opaque"


def split_event(e: dict) -> tuple[str, tuple[str, ...]]:
    """The channel and field names of an edge's event, as csdfparse writes
    them out: the grammar of an event is read by csdfparse alone."""
    if "channel" not in e or "fields" not in e:
        raise ValueError(f"the edge on {e.get('event')!r} has no channel and fields; write the diagram with csdfparse 3 or later")
    return e["channel"], tuple(f["name"] for f in e["fields"])


@dataclass(frozen=True)
class Clause:
    """One clause of a predicate. kind is "local" for a clause the author
    wrote (map None in a flat diagram, the family's map in an expansion),
    "constrain" for a constrain directive's, and one of the generated kinds of
    csdfpromote - in_dom, not_in_dom, at_state, update, create, delete, frame,
    and empty on the start edge - whose meaning is built in. text is the
    clause as its language reads it; opaque marks the tool word @opaque, whose
    text is the word itself."""
    kind: str
    text: str
    map: str | None = None
    id: str | None = None      # the event parameter the instance ID is written as
    state: str | None = None   # the local state's name, for at_state/update/create
    var: str | None = None     # the framed variable
    path: str | None = None    # the local diagram, for a local clause of a family
    line: int = 0
    formal: str | None = None  # a constrain's formal twin, written next to it
    maps: tuple[str, ...] = ()  # the left-out maps a dropped clause read
    opaque: bool = False

    @classmethod
    def from_json(cls, c: dict) -> "Clause":
        opaque = c.get("opaque") is True
        return cls(kind=c["kind"], text=OPAQUE_WORD if opaque else c["text"], opaque=opaque, map=c.get("map"), id=c.get("id"), state=c.get("state"),
                   var=c.get("var"), path=c.get("path"), line=c.get("line", 0), formal=c.get("formal"),
                   maps=tuple(c.get("maps", ())))


def local_clauses(owner: dict, key: str) -> tuple[Clause, ...]:
    """The clauses of owner's predicate key, as csdfparse writes them out
    under key_clauses: the grammar of a clause is read by csdfparse alone."""
    clauses = owner.get(f"{key}_clauses")
    if clauses is None:
        raise ValueError(f"the {key} {owner.get(key)!r} has no {key}_clauses; write the diagram with csdfparse 3 or later")
    return tuple(Clause.from_json({"kind": "local", **c}) for c in clauses)


@dataclass(frozen=True)
class Edge:
    src: str
    dst: str
    event: str
    name: str
    params: tuple[str, ...]
    guard: tuple[Clause, ...]
    post: tuple[Clause, ...]
    line: int

    @property
    def is_tau(self) -> bool:
        return self.event == "tau"

    def implicit_ids(self) -> tuple[str, ...]:
        """The instance-ID parameters the clauses read that the event does not
        carry: a promoted tau names no instance, so its id ranges over them."""
        seen: dict[str, None] = {}
        for c in self.guard + self.post:
            if c.id and c.id not in self.params:
                seen.setdefault(c.id)
        return tuple(seen)


@dataclass(frozen=True)
class EndEdge:
    src: str
    guard: tuple[Clause, ...]


@dataclass(frozen=True)
class Promote:
    map: str
    id_param: str
    path: str
    in_state: str


@dataclass
class Local:
    """A local diagram: what the generated clauses and the local clauses of a
    family refer to. Local states are named by their name, which is what the
    clauses carry."""
    path: str
    vars: dict[str, tuple[str, ...]]  # local state name -> variables

    @classmethod
    def from_json(cls, path: str, d: dict) -> "Local":
        vars_: dict[str, tuple[str, ...]] = {}
        for sid, st in d["states"].items():
            name = st.get("name", sid)
            if name in vars_:
                raise ValueError(f"{path}: two local states are named {name!r}; the clauses name states by name, so names must be unique")
            vars_[name] = tuple(v["name"] for v in st.get("vars", []))
        return cls(path=path, vars=vars_)

    def vars_of(self, state_name: str) -> tuple[str, ...]:
        if state_name not in self.vars:
            raise KeyError(f"{self.path}: no local state named {state_name!r}")
        return self.vars[state_name]


@dataclass
class Diagram:
    vars: dict[str, tuple[str, ...]]          # state id -> its variables, in declaration order
    start: str
    start_post: tuple[Clause, ...]
    edges: list[Edge]
    end: EndEdge | None
    assumptions: list[dict] = field(default_factory=list)
    promotes: dict[str, Promote] = field(default_factory=dict)   # by map
    locals: dict[str, Local] = field(default_factory=dict)       # by path

    @classmethod
    def from_json(cls, d: dict) -> "Diagram":
        if "diagram" in d and "edges" in d:
            return cls.from_expansion(d)
        return cls._from_flat(d, d.get("edges", []), None)

    @classmethod
    def from_expansion(cls, x: dict) -> "Diagram":
        d = x["diagram"]
        by_text = {}
        for e in x.get("edges", []):
            raw = e["edge"]
            key = (raw["src"], raw["dst"], raw["event"], raw.get("guard"), raw.get("post"))
            if e.get("guard") or e.get("post"):
                by_text[key] = (tuple(Clause.from_json(c) for c in e.get("guard", [])),
                                tuple(Clause.from_json(c) for c in e.get("post", [])))
        out = cls._from_flat(d, d.get("edges", []), by_text)
        # The start edge's clauses one by one, the maps' empty start among
        # them; an expansion from before it carried them has only the text.
        if "start" in x:
            if not isinstance(x["start"], list):
                raise ValueError(f"the expansion's start is {x['start']!r}, not a list of clauses; run csdfpromote again")
            out.start_post = tuple(Clause.from_json(c) for c in x["start"])
        out.promotes = {p["map"]: Promote(map=p["map"], id_param=p["id_param"], path=p.get("path", ""), in_state=p["in"])
                        for p in x.get("promotes", [])}
        out.locals = {path: Local.from_json(path, local) for path, local in x.get("locals", {}).items()}
        # A bodyless block shares the file of the block that includes it.
        for p in out.promotes.values():
            if not p.path:
                for q in x.get("promotes", []):
                    if q["map"] == p.map and q.get("path"):
                        out.promotes[p.map] = Promote(p.map, p.id_param, q["path"], p.in_state)
        return out

    @classmethod
    def _from_flat(cls, d: dict, raw_edges: list, by_text) -> "Diagram":
        vars_ = {sid: tuple(v["name"] for v in st.get("vars", [])) for sid, st in d["states"].items()}
        edges = []
        for e in raw_edges:
            name, params = split_event(e)
            key = (e["src"], e["dst"], e["event"], e.get("guard"), e.get("post"))
            if by_text is not None and key in by_text:
                guard, post = by_text[key]
            else:
                guard, post = local_clauses(e, "guard"), local_clauses(e, "post")
            edges.append(Edge(src=e["src"], dst=e["dst"], event=e["event"], name=name, params=params,
                              guard=guard, post=post, line=e.get("line", 0)))
        end = None
        if d.get("end_edge"):
            end = EndEdge(src=d["end_edge"]["src"], guard=local_clauses(d["end_edge"], "guard"))
        start = d["start_edge"]
        for sid in (start["dst"], *[e.src for e in edges], *[e.dst for e in edges]):
            vars_.setdefault(sid, ())
        return cls(vars=vars_, start=start["dst"], start_post=local_clauses(start, "post"),
                   edges=edges, end=end, assumptions=list(d.get("assumptions", [])))

    @classmethod
    def load(cls, path: str) -> "Diagram":
        with open(path, encoding="utf-8") as f:
            return cls.from_json(json.load(f))

    def out_edges(self, sid: str) -> list[Edge]:
        return [e for e in self.edges if e.src == sid]

    def local_of(self, clause: Clause) -> Local:
        """The local diagram a clause about a family refers to."""
        path = clause.path or (self.promotes[clause.map].path if clause.map in self.promotes else None)
        if path is None or path not in self.locals:
            raise KeyError(f"the clause {clause.text!r} is about map {clause.map!r}, whose local diagram is not in the expansion")
        return self.locals[path]
