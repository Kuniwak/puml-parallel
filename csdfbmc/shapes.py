"""What the formal clauses of a diagram reveal of the values of its types:
the attributes they read and the constants they compare them with.

The reading is syntactic: an attribute no clause mentions is absent, and a
value is known only where a clause compares an attribute with a constant."""
from __future__ import annotations

import ast

# The operators whose operands are numbers.
ARITHMETIC = (ast.Add, ast.Sub, ast.Mult, ast.Div, ast.FloorDiv, ast.Mod)
ORDER = (ast.Lt, ast.LtE, ast.Gt, ast.GtE)


def shapes(doc: dict) -> dict:
    """The attribute tree of every type of doc, a diagram as csdfparse
    prints it, as {"types": {type: node}}."""
    var_types = {v["name"]: v["type"] for vs in doc["vars"].values() for v in vs}
    families = {f["map"]: {v["name"]: v["type"] for vs in f["vars"].values() for v in vs} for f in doc.get("families", [])}
    trees: dict = {}
    for e in doc["edges"]:
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
                if isinstance(node, ast.Compare):
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
    return {"types": {t: _node(n) for t, n in trees.items()}}


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
    if n["fields"]:
        return {"kind": "record", "fields": {k: _node(v) for k, v in n["fields"].items()}}
    out = {"kind": "scalar"}
    if n.get("examples"):
        out["examples"] = n["examples"]
    if n.get("numeric"):
        out["numeric"] = True
    return out
