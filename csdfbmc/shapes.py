"""What the formal clauses of a diagram reveal of the values of its types:
the attributes they read and the constants they compare them with.

The reading is syntactic: an attribute no clause mentions is absent, and a
value is known only where a clause compares an attribute with a constant."""
from __future__ import annotations

import ast


def shapes(doc: dict) -> dict:
    """The attribute tree of every type of doc, a diagram as csdfparse
    prints it, as {"types": {type: node}}."""
    var_types = {v["name"]: v["type"] for vs in doc["vars"].values() for v in vs}
    trees: dict = {}
    for e in doc["edges"]:
        for a in _atoms(e.get("guard", []) + e.get("post", [])):
            holes = [var_types.get(a["args"][h].get("name")) for h in a["holes"]]
            source = a["pieces"][0] + "".join(f" _{i} " + p for i, p in enumerate(a["pieces"][1:]))
            try:
                tree = ast.parse(source.strip(), mode="eval")
            except SyntaxError:
                continue
            for node in ast.walk(tree):
                if isinstance(node, ast.Compare):
                    sides = [node.left, *node.comparators]
                    consts = [s.value for s in sides if isinstance(s, ast.Constant)]
                    for s in sides:
                        found = _path(s, holes)
                        if found:
                            leaf = _add(trees, *found)
                            for c in consts:
                                if c not in leaf.setdefault("examples", []):
                                    leaf["examples"].append(c)
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
        t = holes[int(node.id[1:])]
        if t is not None:
            return t, list(reversed(path))
    return None


def _add(trees, t, path):
    n = trees.setdefault(t, {"fields": {}})
    for a in path:
        n = n["fields"].setdefault(a, {"fields": {}})
    return n


def _node(n):
    if n["fields"]:
        return {"kind": "record", "fields": {k: _node(v) for k, v in n["fields"].items()}}
    out = {"kind": "scalar"}
    if n.get("examples"):
        out["examples"] = n["examples"]
    return out
