"""Diagrams written by hand for the tests, in the JSON csdfparse prints
(core.DiagramDoc, version 2). The atoms follow the reader only as far as the
tests need: a hole is $name or $name' (a space after it ends it before a
character that could continue the name, as in docs/SYNTAX.md), and ${}
takes the next of the terms given."""
from __future__ import annotations

import re
import unicodedata


def continues_name(c: str) -> bool:
    """Whether c may appear in an identifier (csdf/ident)."""
    return c == "_" or unicodedata.category(c)[0] in "LN"


def term(t):
    """A term: "x" is the variable x before the step, "x'" after it; a dict
    is a term already."""
    if isinstance(t, dict):
        return t
    if t.endswith("'"):
        return {"kind": "free", "name": t[:-1], "primed": True}
    return {"kind": "free", "name": t}


def idx(m, k):
    return {"kind": "index", "map": term(m), "key": term(k)}


def fld(t, name):
    return {"kind": "field", "of": term(t), "name": name}


def bound(i):
    return {"kind": "bound", "index": i}


def atom(text, *terms):
    """The atom text, its holes $x, $x' and ${} (the next of terms)."""
    pieces, holes, args, given = [""], [], [], list(terms)
    i = 0
    while i < len(text):
        c = text[i]
        if c == "$" and text[i + 1:i + 3] == "{}":
            t = term(given.pop(0))
            i += 3
        elif c == "$" and i + 1 < len(text) and continues_name(text[i + 1]) and not text[i + 1].isdigit():
            j = i + 1
            while j < len(text) and continues_name(text[j]):
                j += 1
            name = text[i + 1:j]
            if j < len(text) and text[j] == "'":
                name += "'"
                j += 1
            if j + 1 < len(text) and text[j] == " " and continues_name(text[j + 1]):
                j += 1
            t = term(name)
            i = j
        else:
            pieces[-1] += c
            i += 1
            continue
        if t not in args:
            args.append(t)
        holes.append(args.index(t))
        pieces.append("")
    primed = [_primed(a) for a in args]
    wire = [pieces[0]] + [(" " + p if p and continues_name(p[0]) else p) for p in pieces[1:]]
    function = _escaped(pieces[0]) + "".join(
        f"$_{h}" + (" " if p and continues_name(p[0]) else "") + _escaped(p) for h, p in zip(holes, pieces[1:]))
    return {"kind": "atom", "pieces": pieces, "holes": holes, "args": args, "text": wire, "primed": primed, "function": function}


def _escaped(piece):
    return re.sub(r"\$(?=[{\w])", r"\\$", piece)


def _primed(t):
    while t["kind"] in ("index", "field"):
        t = t["map"] if t["kind"] == "index" else t["of"]
    return t["kind"] == "free" and t.get("primed", False)


def builtin(op, *args, state=None):
    b = {"kind": "builtin", "op": op, "args": [term(a) for a in args]}
    if state is not None:
        b["state"] = state
    return b


def eq(a, b):
    return builtin("eq", a, b)


def exists(type_, *body):
    return {"kind": "exists", "type": type_, "body": list(body)}


def edge(src, dst, event, guard=(), post=(), line=0):
    channel, *rest = re.split(r"([?!])", event)
    return {"src": src, "dst": dst, "channel": channel,
            "fields": [{"dir": d, "name": n} for d, n in zip(rest[::2], rest[1::2])],
            "label": event, "guard": list(guard), "post": list(post), "line": line}


def end(src, guard=()):
    return {"src": src, "guard": list(guard)}


def family(m, init, states):
    """The family of the map m: states maps each local state to its
    variables, in order; init is the start state, which is absence."""
    return {"map": m, "init": init, "states": list(states), "vars": {s: list(vs) for s, vs in states.items()}}


def diagram(states, init, init_post=(), edges=(), ends=(), name="D", assumes=(), families=()):
    """A diagram: states maps each state to its variables, in order."""
    return {"version": 2, "name": name, "states": list(states), "labels": {},
            "vars": {s: list(vs) for s, vs in states.items()}, "init": init, "init_post": list(init_post),
            "init_line": 0, "edges": list(edges), "ends": list(ends), "assumes": list(assumes),
            "families": list(families)}


def load(doc):
    from csdfbmc.model import Diagram
    return Diagram.from_json(doc)


_GLUE = None


def glue():
    """The reference Python glue, checked, one for the whole run (it runs
    in-process and holds no state between requests but its caches)."""
    global _GLUE
    from csdfbmc.protocol import checked, start_module
    if _GLUE is None:
        _GLUE = checked(start_module("csdfbmc.python_glue", []))
    return _GLUE


def explorer(d, checks=None, cone=False, **options):
    """An Explorer walking d, a diagram written by hand (or a Diagram), its
    atoms read by the reference Python glue, with the reductions asked for
    built as the command builds them (csdfbmc.cli.reductions)."""
    from csdfbmc import environment
    from csdfbmc.checks import Checks
    from csdfbmc.cli import reductions
    from csdfbmc.explore import Explorer
    from csdfbmc.model import Diagram
    if not isinstance(d, Diagram):
        d = load(d)
    checks = checks if checks is not None else Checks(types={})
    options.setdefault("environment", environment.NONE)
    return Explorer(d, checks, glue(), **options, **reductions(d, checks, cone=cone))


def run_main(test, main, diagram, *args, checks="TYPES = {}\n"):
    """main (csdfbmc's or csdfbmc next's) run on diagram, a diagram written by
    hand, and a checks module of the text checks, both written to files that
    test cleans up: its exit code, standard output and standard error."""
    import io
    import json
    import os
    import tempfile
    from contextlib import redirect_stderr, redirect_stdout
    d = tempfile.TemporaryDirectory()
    test.addCleanup(d.cleanup)
    path, checks_path = os.path.join(d.name, "d.json"), os.path.join(d.name, "t.py")
    with open(path, "w", encoding="utf-8") as f:
        json.dump(diagram, f)
    with open(checks_path, "w", encoding="utf-8") as f:
        f.write(checks)
    out, err = io.StringIO(), io.StringIO()
    with redirect_stdout(out), redirect_stderr(err):
        code = main([path, checks_path, *args])
    return code, out.getvalue(), err.getvalue()


def findings(result) -> set:
    """The findings of a walk by their shape, without their counts, traces
    or examples: what a reduction must leave as it is."""
    return {tuple(sorted((k, tuple(v) if isinstance(v, list) else v) for k, v in g.items() if k not in ("count", "trace", "example")))
            for g in result.groups()}


EXAMPLES = {
    "withdraw": ("withdraw_formal.json", "withdraw_formal_defs.py"),
    "wallet": ("wallet_formal.json", "wallet_formal_checks.py"),
    "attempts": ("attempts/attempts.json", "attempts/checks.py"),
}


def example(name, cone=False, **options):
    """An Explorer walking the example of examples/bmc named name, its atoms
    read by the reference Python glue given its checks module."""
    import os
    from csdfbmc import environment
    from csdfbmc.checks import load_checks
    from csdfbmc.cli import reductions
    from csdfbmc.explore import Explorer
    from csdfbmc.model import Diagram
    from csdfbmc.protocol import checked, start_module
    root = os.path.join(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))), "examples", "bmc")
    path, checks = (os.path.join(root, p) for p in EXAMPLES[name])
    d, loaded = Diagram.load(path), load_checks(checks)
    options.setdefault("environment", environment.NONE)
    return Explorer(d, loaded, checked(start_module("csdfbmc.python_glue", [checks])), **options, **reductions(d, loaded, cone=cone))
