"""Diagrams written by hand for the tests, filled in the way csdfparse writes
them out: the channel and fields of each event and the clauses of each
predicate. The grammar is csdfparse's; this follows it only as far as the
tests' diagrams need: no escapes, and @opaque on its own."""
import re

CONNECTIVE = " ∧ "


def clauses(text):
    if text is None or text.strip() == "":
        return []
    return [{"opaque": True} if t.strip() == "@opaque" else {"text": t.strip()} for t in text.split(CONNECTIVE)]


def event(text):
    channel, *rest = re.split(r"([?!])", text)
    return {"channel": channel, "fields": [{"dir": d, "name": n} for d, n in zip(rest[::2], rest[1::2])]}


def parsed(diagram):
    """diagram, or the diagram of an expansion, as csdfparse would write it;
    what a test edited is taken apart again."""
    if "diagram" in diagram:
        return dict(diagram, diagram=parsed(diagram["diagram"]),
                    edges=[dict(e, edge=_edge(e["edge"])) for e in diagram.get("edges", [])])
    out = dict(diagram, edges=[_edge(e) for e in diagram.get("edges", [])])
    out["start_edge"] = dict(diagram["start_edge"], post_clauses=clauses(diagram["start_edge"].get("post")))
    if diagram.get("end_edge"):
        out["end_edge"] = dict(diagram["end_edge"], guard_clauses=clauses(diagram["end_edge"].get("guard")))
    return out


def _edge(e):
    return dict(e, **event(e["event"]), guard_clauses=clauses(e.get("guard")), post_clauses=clauses(e.get("post")))


def diagram(d):
    """The Diagram of a diagram written by hand."""
    from csdfbmc.model import Diagram
    return Diagram.from_json(parsed(d))
