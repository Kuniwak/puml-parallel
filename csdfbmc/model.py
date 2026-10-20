"""The diagram as csdfparse prints it, with the clauses of each predicate."""
from __future__ import annotations

import json
import re
from dataclasses import dataclass, field

# The clause connective is grammar (docs/SYNTAX.md): a predicate is its clauses
# joined by " ∧ ", and the omitted predicate or the literal true has no clause.
CONNECTIVE = " ∧ "

_EVENT_RE = re.compile(r"^([^()]+?)\s*\(\s*(.*?)\s*\)$")


def clauses(text: str | None) -> tuple[str, ...]:
    if text is None or text.strip() in ("", "true"):
        return ()
    return tuple(part.strip() for part in text.split(CONNECTIVE))


def split_event(event: str) -> tuple[str, tuple[str, ...]]:
    """An event is free text; by the convention csdfpromote also follows,
    "name(a, b)" is a name with parameters and anything else a bare name."""
    m = _EVENT_RE.match(event)
    if m is None:
        return event.strip(), ()
    if m.group(2) == "":
        return m.group(1), ()
    return m.group(1), tuple(p.strip() for p in m.group(2).split(",") if p.strip())


@dataclass(frozen=True)
class Edge:
    src: str
    dst: str
    event: str
    name: str
    params: tuple[str, ...]
    guard: tuple[str, ...]
    post: tuple[str, ...]
    line: int

    @property
    def is_tau(self) -> bool:
        return self.event == "tau"


@dataclass(frozen=True)
class EndEdge:
    src: str
    guard: tuple[str, ...]


@dataclass
class Diagram:
    vars: dict[str, tuple[str, ...]]          # state id -> its variables, in declaration order
    start: str
    start_post: tuple[str, ...]
    edges: list[Edge]
    end: EndEdge | None
    assumptions: list[dict] = field(default_factory=list)

    @classmethod
    def from_json(cls, d: dict) -> "Diagram":
        vars_ = {sid: tuple(v["name"] for v in st.get("vars", [])) for sid, st in d["states"].items()}
        edges = []
        for e in d.get("edges", []):
            name, params = split_event(e["event"])
            edges.append(Edge(
                src=e["src"], dst=e["dst"], event=e["event"], name=name, params=params,
                guard=clauses(e.get("guard")), post=clauses(e.get("post")), line=e.get("line", 0),
            ))
        end = None
        if d.get("end_edge"):
            end = EndEdge(src=d["end_edge"]["src"], guard=clauses(d["end_edge"].get("guard")))
        start = d["start_edge"]
        for sid in (start["dst"], *[e.src for e in edges], *[e.dst for e in edges]):
            vars_.setdefault(sid, ())
        return cls(vars=vars_, start=start["dst"], start_post=clauses(start.get("post")),
                   edges=edges, end=end, assumptions=list(d.get("assumptions", [])))

    @classmethod
    def load(cls, path: str) -> "Diagram":
        with open(path, encoding="utf-8") as f:
            return cls.from_json(json.load(f))

    def out_edges(self, sid: str) -> list[Edge]:
        return [e for e in self.edges if e.src == sid]
