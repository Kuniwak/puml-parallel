"""python -m csdfbmc.shapes <diagram.json> [<checks.py>]

What the formal clauses of a diagram reveal of the values of its types:
the attributes they read and the constants they compare them with.

The reading is syntactic: an attribute no clause mentions is absent, and a
value is known only where a clause compares an attribute with a constant.
The types the checks module gives in TYPES, when given, take their shapes
from TYPES instead."""
from __future__ import annotations

import argparse
import ast
import json
import sys

from csdfbmc.checks import PurityError, load_checks
from csdfbmc.values import Built, Sample, record

# The operators whose operands are numbers.
ARITHMETIC = (ast.Add, ast.Sub, ast.Mult, ast.Div, ast.FloorDiv, ast.Mod)
ORDER = (ast.Lt, ast.LtE, ast.Gt, ast.GtE)


def declared(values) -> dict:
    """The node of a type of TYPES whose values are values: a record of
    csdfbmc.values.record as the record of its fields' nodes, a sample as a
    scalar with its values as examples, and other values listed as a
    scalar with "values", all of them in order, when they are finitely many
    constants; values it cannot list are a scalar of nothing more."""
    if isinstance(values, Built) and values.build is record:
        return {"kind": "record", "fields": {k: declared(v) for k, v in values.parts.items()}}
    if not isinstance(values, (list, tuple, range)) \
            or not all(isinstance(v, (str, int, float, bool)) or v is None for v in values):
        return {"kind": "scalar"}
    out = {"kind": "scalar", ("examples" if isinstance(values, Sample) else "values"): list(values)}
    if values and all(isinstance(v, int) and not isinstance(v, bool) for v in values):
        out["numeric"] = True
    return out


def shapes(doc: dict, given: dict | None = None) -> dict:
    """What the clauses of doc, a diagram as csdfpromote -format json
    prints it, reveal: {"types": {type: node}, "vars": {variable:
    type}}. A node is {"kind": "record", "fields": {attribute: node}},
    {"kind": "collection", "items": node} or {"kind": "scalar"}, the last
    with "examples", the constants it is compared with, and "numeric": true
    when it is ordered or computed with. A type no clause reads anything of
    is absent; a map type K ⇸ V is no shape, its values being of V. A type
    given, a type -> its values as TYPES gives them, has its node from
    them (declared) in place of the one the clauses reveal, and is listed
    in "declared"."""
    var_types = {v["name"]: v["type"] for vs in doc["vars"].values() for v in vs}
    families = {f["map"]: {v["name"]: v["type"] for vs in f["vars"].values() for v in vs} for f in doc.get("families", [])}
    trees: dict = {}
    start = {"fields": [], "guard": [], "post": doc.get("init_post", [])}
    for e in [start, *doc["edges"]]:
        types = _term_types(doc, e, var_types)
        for term in _fields(e.get("guard", []) + e.get("post", [])):
            found = _resolved(term, types, families)
            if found:
                _add(trees, *found)
        for a in _atoms(e.get("guard", []) + e.get("post", [])):
            holes = [_resolved(a["args"][h], types, families) for h in a["holes"]]
            source = a["pieces"][0] + "".join(f" _{i} " + p for i, p in enumerate(a["pieces"][1:]))
            try:
                tree = ast.parse(source.strip(), mode="eval")
            except SyntaxError:
                continue
            for node in ast.walk(tree):
                found = _path(node, holes)
                if found:
                    _add(trees, *found)
                numbers = []
                if isinstance(node, ast.BinOp) and isinstance(node.op, ARITHMETIC):
                    numbers = [node.left, node.right]
                if isinstance(node, ast.Compare) and any(isinstance(op, ORDER) for op in node.ops):
                    numbers = [node.left, *node.comparators]
                for x in numbers:
                    found = _path(x, holes)
                    if found:
                        _add(trees, *found)["numeric"] = True
                if isinstance(node, ast.Compare) and len(node.ops) == 1 and isinstance(node.ops[0], (ast.In, ast.NotIn)) \
                        and _path(node.comparators[0], holes):
                    items = _add(trees, *_path(node.comparators[0], holes)).setdefault("items", {"fields": {}})
                    if isinstance(node.left, ast.Constant):
                        _example(items, node.left.value)
                elif isinstance(node, ast.Compare):
                    sides = [node.left, *node.comparators]
                    consts = [s.value for s in sides if isinstance(s, ast.Constant)]
                    consts += [x.value for s in sides if isinstance(s, (ast.Tuple, ast.List, ast.Set))
                               for x in s.elts if isinstance(x, ast.Constant)]
                    for s in sides:
                        found = _path(s, holes)
                        if found:
                            leaf = _add(trees, *found)
                            for c in consts:
                                _example(leaf, c)
                            for other in sides:
                                if isinstance(other, ast.Dict):
                                    _built(leaf, other)
    instance_vars = {v: t for vs in families.values() for v, t in vs.items()}
    inferred = {t: _node(n) for t, n in trees.items() if "⇸" not in t}
    return {"types": {**inferred, **{t: declared(vs) for t, vs in (given or {}).items()}},
            "vars": {**var_types, **instance_vars},
            "declared": sorted(given or {})}


def _atoms(clauses):
    for c in clauses:
        if isinstance(c, dict):
            if c.get("kind") == "atom":
                yield c
            for v in c.values():
                if isinstance(v, (list, dict)):
                    yield from _atoms(v if isinstance(v, list) else [v])


def _path(node, holes):
    """The type node reads an attribute of and the attributes, or None."""
    if isinstance(node, ast.Name) and node.id.startswith("_") and node.id[1:].isdigit():
        return holes[int(node.id[1:])]
    if not isinstance(node, ast.Subscript):
        return None
    of = _path(node.value, holes)
    if of is None:
        return None
    if isinstance(node.slice, ast.Constant) and isinstance(node.slice.value, str):
        return of[0], of[1] + [node.slice.value]
    # A value of a map type K ⇸ V looked up is of V.
    _, arrow, value = of[0].partition("⇸")
    return (value.strip(), []) if arrow and not of[1] else None


def _term_types(doc, e, var_types):
    """The types of the names an edge's clauses use: its event's fields
    over the state variables."""
    names = [f["name"] for f in e.get("fields", [])]
    types = doc["channels"].get(e.get("channel"), [])
    return {**var_types, **dict(zip(names, types))} if len(names) == len(types) else var_types


def _resolved(term, types, families):
    """The type a term's value is of and the attributes read of it, or None."""
    if term["kind"] == "free" and term["name"] in types:
        return types[term["name"]], []
    of = term.get("of", {})
    if term["kind"] == "field" and of.get("kind") == "index" and of["map"].get("name") in families:
        # An instance of a family is the map of its variables.
        instance = families[of["map"]["name"]]
        return (instance[term["name"]], []) if term["name"] in instance else None
    if term["kind"] == "field":
        of = _resolved(term["of"], types, families)
        return of and (of[0], of[1] + [term["name"]])
    return None


def _fields(x):
    """The field terms in clauses, outermost first."""
    if isinstance(x, dict):
        if x.get("kind") == "field":
            yield x
        for v in x.values():
            yield from _fields(v)
    elif isinstance(x, list):
        for v in x:
            yield from _fields(v)


def _add(trees, t, path):
    n = trees.setdefault(t, {"fields": {}})
    for a in path:
        n = n["fields"].setdefault(a, {"fields": {}})
    return n


def _example(leaf, value):
    if value not in leaf.setdefault("examples", []):
        leaf["examples"].append(value)


def _built(n, d: ast.Dict):
    """Adds to n the keys of the dict d builds, with the constants it puts
    under them, of either branch of a conditional."""
    for k, v in zip(d.keys, d.values):
        if not (isinstance(k, ast.Constant) and isinstance(k.value, str)):
            continue
        field = n["fields"].setdefault(k.value, {"fields": {}})
        branches = [v]
        while branches:
            x = branches.pop()
            if isinstance(x, ast.IfExp):
                branches += [x.body, x.orelse]
            elif isinstance(x, ast.Constant):
                _example(field, x.value)
            elif isinstance(x, ast.Dict):
                _built(field, x)


def _node(n):
    if "items" in n:
        return {"kind": "collection", "items": _node(n["items"])}
    if n["fields"]:
        return {"kind": "record", "fields": {k: _node(v) for k, v in n["fields"].items()}}
    out = {"kind": "scalar"}
    if n.get("examples"):
        out["examples"] = n["examples"]
    if n.get("numeric"):
        out["numeric"] = True
    return out


def main(argv=None) -> int:
    ap = argparse.ArgumentParser(prog="csdfbmc shapes", description=__doc__.split("\n\n")[1])
    ap.add_argument("diagram", help="the diagram as JSON (csdfpromote -format json's output), its clauses in Python")
    ap.add_argument("checks", nargs="?", help="the checks module, whose TYPES give the shapes of the types it lists in place of what the clauses reveal")
    a = ap.parse_args(argv)
    try:
        with open(a.diagram, encoding="utf-8") as f:
            doc = json.load(f)
        got = shapes(doc, load_checks(a.checks).types if a.checks else None)
    except (OSError, ValueError, KeyError, PurityError) as e:
        print(f"csdfbmc shapes: {e}", file=sys.stderr)
        return 2
    json.dump(got, sys.stdout, ensure_ascii=False, indent=2)
    print()
    return 0


if __name__ == "__main__":
    sys.exit(main())
