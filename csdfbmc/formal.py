"""What a formal expression may use besides the names of the step: the
helpers over the maps of an expansion and a few pure builtins."""
from __future__ import annotations

import ast

from csdfbmc.values import FrozenMap

# The key under which an instance's record holds its local state's name; the
# local variables are the other keys.
RECORD_STATE = "state"

# What a formal expression may call besides the names of the step.
FORMAL_GLOBALS = {
    "__builtins__": {},
    "len": len, "all": all, "any": any, "sum": sum, "min": min, "max": max, "abs": abs,
    "sorted": sorted, "set": set, "frozenset": frozenset, "tuple": tuple, "range": range,
    "FrozenMap": FrozenMap,
    "at": lambda m, i, state: i in m and m[i][RECORD_STATE] == state,
    "exists": lambda m, pred: any(pred(i, r) for i, r in m.items()),
    "count": lambda m, pred: sum(1 for i, r in m.items() if pred(i, r)),
}


def free_names(tree: ast.AST) -> set[str]:
    """The names tree reads that it does not bind itself, in a comprehension
    or a lambda."""
    loaded, bound = set(), set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Name):
            (loaded if isinstance(node.ctx, ast.Load) else bound).add(node.id)
        elif isinstance(node, ast.arg):
            bound.add(node.arg)
    return loaded - bound


def names_beyond(tree: ast.AST, own_id: str | None) -> set[str]:
    """The names tree reads, but a map read only as m[own_id]: what ties a
    clause about one instance to the others."""
    keyed = {id(n.value) for n in ast.walk(tree)
             if isinstance(n, ast.Subscript) and isinstance(n.value, ast.Name)
             and isinstance(n.slice, ast.Name) and n.slice.id == own_id}
    return {n.id for n in ast.walk(tree) if isinstance(n, ast.Name) and id(n) not in keyed}


def tells_apart(tree: ast.AST, params: set[str], maps: set[str], groups: list[frozenset]) -> tuple[int, str] | None:
    """Where and how tree tells two IDs of a group apart, the params ranging
    over them and the maps keyed by them; None when it does not. A constant
    naming one ID does, unless it is in a literal listing every ID of its
    group, as a domain does; so does an ID ordered by <, <=, >, >=, and a
    sort, min or max over an ID or such a map. The check is syntactic, and
    does not follow a value through a helper."""
    values = frozenset().union(*groups) if groups else frozenset()
    skip = set()
    for node in ast.walk(tree):
        if isinstance(node, (ast.List, ast.Tuple, ast.Set)):
            listed = {e.value for e in node.elts if isinstance(e, ast.Constant)}
            if any(g <= listed for g in groups):
                skip |= set(map(id, node.elts))
    symmetric = params | maps
    for node in ast.walk(tree):
        if id(node) in skip:
            continue
        line = getattr(node, "lineno", 0)
        if isinstance(node, ast.Constant) and isinstance(node.value, str) and node.value in values:
            return line, f"the constant {node.value!r} names one symmetric ID"
        if isinstance(node, ast.Compare) and any(isinstance(op, (ast.Lt, ast.LtE, ast.Gt, ast.GtE)) for op in node.ops):
            # An ID used as a key (m[id].x < 3) is not itself ordered.
            for operand in [node.left, *node.comparators]:
                if isinstance(operand, ast.Name) and operand.id in params:
                    return line, f"{operand.id} is ordered"
        if isinstance(node, ast.Call) and isinstance(node.func, ast.Name) and node.func.id in ("sorted", "min", "max"):
            for n in ast.walk(node):
                if isinstance(n, ast.Name) and n.id in symmetric:
                    return line, f"{node.func.id} orders {n.id}"
    return None
