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
    trees: dict = {}
    for e in doc["edges"]:
        for a in _atoms(e.get("guard", []) + e.get("post", [])):
            holes = [_resolved(a["args"][h], _term_types(doc, e, var_types)) for h in a["holes"]]
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
    path = []
    while isinstance(node, ast.Subscript) and isinstance(node.slice, ast.Constant) and isinstance(node.slice.value, str):
        path.append(node.slice.value)
        node = node.value
    if isinstance(node, ast.Name) and node.id.startswith("_") and node.id[1:].isdigit():
        hole = holes[int(node.id[1:])]
        if hole is not None:
            return hole[0], hole[1] + list(reversed(path))
    return None


def _term_types(doc, e, var_types):
    """The types of the names an edge's clauses use: its event's fields
    over the state variables."""
    names = [f["name"] for f in e.get("fields", [])]
    types = doc["channels"].get(e.get("channel"), [])
    return {**var_types, **dict(zip(names, types))} if len(names) == len(types) else var_types


def _resolved(term, types):
    """The type a term's value is of and the attributes read of it, or None."""
    if term["kind"] == "free" and term["name"] in types:
        return types[term["name"]], []
    return None


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
