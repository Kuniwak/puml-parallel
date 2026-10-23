"""What a formal expression may use besides its holes: a few pure
builtins. An instance of a family reaches an expression as the map of its
fields; its state is read only through the builtin @at, never by a glue."""
from __future__ import annotations

import ast

from csdfbmc.values import FrozenMap

# What a formal expression may call besides the names of the step.
FORMAL_GLOBALS = {
    "__builtins__": {},
    "len": len, "all": all, "any": any, "sum": sum, "min": min, "max": max, "abs": abs,
    "sorted": sorted, "set": set, "frozenset": frozenset, "tuple": tuple, "range": range,
    "FrozenMap": FrozenMap,
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
