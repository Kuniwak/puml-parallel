"""The reference glue of the evaluator protocol, for clauses written in the
pure subset of Python that csdfbmc.twins.check_pure enforces.

evaluator(args) returns the handler of the protocol's requests, which
csdfbmc loads in-process with --evaluator-module; run as a command
(python3 -m csdfbmc.python_glue), it speaks the same requests as JSON lines
on its standard input and output.
"""
from __future__ import annotations

import ast
import importlib.util
from typing import Callable

from csdfbmc.formal import FORMAL_GLOBALS
from csdfbmc.twins import check_pure
from csdfbmc.values import freeze

LANGUAGE = "python"
PROTOCOL = 1
OPAQUE = "OPAQUE"


def _free_names(tree: ast.AST) -> set[str]:
    """The names tree reads that it does not bind itself, in a comprehension
    or a lambda."""
    loaded, bound = set(), set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Name):
            (loaded if isinstance(node.ctx, ast.Load) else bound).add(node.id)
        elif isinstance(node, ast.arg):
            bound.add(node.arg)
    return loaded - bound


def _post_writes(tree: ast.Expression) -> list[str]:
    """The variables a post gives a value to: the keys of its assignments,
    which are dict displays with constant string keys, in a list, set or
    tuple display or a comprehension."""
    body = tree.body
    if isinstance(body, (ast.ListComp, ast.SetComp, ast.GeneratorExp)):
        dicts = [body.elt]
    elif isinstance(body, (ast.List, ast.Set, ast.Tuple)):
        dicts = body.elts
    else:
        raise ValueError("a post is a list or set of assignments, such as [{'x': x + 1}] or [{'y': v} for v in ys]")
    keys = []
    for d in dicts:
        if not isinstance(d, ast.Dict) or not all(isinstance(k, ast.Constant) and isinstance(k.value, str) for k in d.keys):
            raise ValueError("each assignment of a post is a dict display whose keys are the variables, written as strings")
        keys.append(sorted(k.value for k in d.keys))
    if any(k != keys[0] for k in keys):
        raise ValueError("the assignments of a post must give the same variables a value")
    return keys[0] if keys else []


class Glue:
    """Answers the requests of the protocol. defs are the paths of modules
    whose public names every clause may use: constants and helpers shared by
    the clauses, checked against the pure subset like a twins module."""

    def __init__(self, defs: list[str]):
        self.globals = dict(FORMAL_GLOBALS)
        for path in defs:
            self.globals.update(_load_defs(path))
        self._compiled: dict[str, tuple] = {}

    def _compile(self, clause: str, role: str):
        key = (clause, role)
        if key not in self._compiled:
            check_pure(clause, f"<clause {clause!r}>")
            tree = ast.parse(clause, mode="eval")
            writes = _post_writes(tree) if role == "post" else []
            reads = sorted(_free_names(tree) - set(self.globals))
            self._compiled[key] = (compile(tree, f"<clause {clause!r}>", "eval"), reads, writes)
        return self._compiled[key]

    def _eval(self, clause: str, role: str, binding: dict):
        code, _, _ = self._compile(clause, role)
        return eval(code, self.globals, dict(binding))

    def handle(self, request: dict) -> dict:
        op = request.get("op")
        if op == "hello":
            return {"language": LANGUAGE, "protocol": PROTOCOL}
        try:
            if op == "analyse":
                if request["clause"].strip() == OPAQUE:
                    return {"opaque": True}
                _, reads, writes = self._compile(request["clause"], request["role"])
                return {"reads": reads, "writes": writes}
            if op == "guard":
                return {"results": [bool(self._eval(request["clause"], "guard", b)) for b in request["bindings"]]}
            if op == "post":
                return {"results": [[{k: freeze(v) for k, v in a.items()} for a in self._eval(request["clause"], "post", b)]
                                    for b in request["bindings"]]}
        except Exception as e:  # what the clause raised is the answer, reported with it
            return {"error": f"{type(e).__name__}: {e}"}
        return {"error": f"unknown request {op!r}"}


def _load_defs(path: str) -> dict:
    with open(path, encoding="utf-8") as f:
        check_pure(f.read(), path)
    spec = importlib.util.spec_from_file_location("csdf_clause_defs", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return {k: v for k, v in vars(module).items() if not k.startswith("_")}


def evaluator(args: list[str]) -> Callable[[dict], dict]:
    """The handler of the protocol's requests; args are the paths of the
    modules of shared definitions."""
    return Glue(args).handle


if __name__ == "__main__":
    import sys

    from csdfbmc.protocol import serve
    serve(evaluator(sys.argv[1:]), sys.stdin, sys.stdout)
