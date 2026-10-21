"""Diagrams written by hand for the tests, filled in the way csdfparse writes
them out: the channel and fields of each event and the clauses of each
predicate. The grammar is csdfparse's; this follows it only as far as the
tests' diagrams need: no escapes, and @opaque on its own."""
import re

CONNECTIVE = " ∧ "


def clauses(text):
    if text is None or text.strip() == "":
        return []
    return [{"form": "opaque"} if t.strip() == "@opaque" else {"form": "text", "text": t.strip()} for t in text.split(CONNECTIVE)]


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


_GLUE = None


def glue_meaning():
    """The meaning the reference Python glue gives clauses, one glue for the
    whole run (it runs in-process and holds no state between requests but its
    caches)."""
    global _GLUE
    from csdfbmc.meaning import GlueMeaning, standard
    from csdfbmc.protocol import checked, start_module
    if _GLUE is None:
        _GLUE = start_module("csdfbmc.python_glue", [])
    return standard(GlueMeaning(checked(_GLUE)))


def explorer(d, checks=None, **options):
    """An Explorer walking d, a diagram written by hand (or a Diagram), with
    its clauses read by the reference Python glue. checks holds the domains,
    ghosts, invariants and symmetric parameters."""
    from csdfbmc.cone import Cone
    from csdfbmc.explore import Explorer
    from csdfbmc.model import Diagram
    from csdfbmc.twins import Twins
    if not isinstance(d, Diagram):
        d = diagram(d)
    options.setdefault("cone", Cone.keep_all())
    return Explorer(d, checks if checks is not None else Twins(domains={}, clauses={}), glue_meaning(), **options)
