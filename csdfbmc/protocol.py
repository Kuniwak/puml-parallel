"""The evaluator protocol: how the checker asks a clause language about its
clauses, independent of the language.

A glue is a long-running process that reads one JSON request per line on its
standard input and writes one JSON answer per line on its standard output,
in order. The requests are:

- {"op": "hello"} -> {"language": NAME, "protocol": 1}, first.
- {"op": "analyse", "clause": TEXT, "role": "guard" | "post"}
  -> {"reads": [NAME, ...], "writes": [NAME, ...]}: the names the clause
  reads and, for a post, the variables it gives a value to, known without
  evaluating it; or {"opaque": true} for a clause not yet formalised.
- {"op": "guard", "clause": TEXT, "bindings": [BINDING, ...]}
  -> {"results": [true | false, ...]}, one per binding.
- {"op": "post", "clause": TEXT, "bindings": [BINDING, ...]}
  -> {"results": [[ASSIGNMENT, ...], ...]}: per binding, the successor
  values of the clause's writes, each assignment giving every one of them.

A binding and an assignment are JSON objects from names to values. Any
request may be answered with {"error": MESSAGE}. A clause must be pure and
deterministic: the checker may answer a request again from what it was told
for the same clause and the same values of the names it reads.

Values cross the protocol in a tagged JSON encoding, since JSON has no
tuples, sets or maps with keys other than strings: integers, strings,
booleans and null as themselves, and {"tuple": [...]}, {"set": [...]},
{"map": [[key, value], ...]} for the containers of csdfbmc.values.

A glue written in Python may also be loaded in-process: a module whose
evaluator(args) returns a function from a request to its answer, with the
checker's values in place of their encoding. csdfbmc.python_glue is both.
"""
from __future__ import annotations

import importlib
import json
import shlex
import subprocess
from collections.abc import Mapping

from csdfbmc.values import FrozenMap

_TAGS = ("tuple", "set", "map")


def to_wire(value):
    """value, a value of the checker's model, in the tagged encoding."""
    if value is None or isinstance(value, (bool, int, str)):
        return value
    if isinstance(value, Mapping):
        return {"map": [[to_wire(k), to_wire(v)] for k, v in value.items()]}
    if isinstance(value, tuple):
        return {"tuple": [to_wire(v) for v in value]}
    if isinstance(value, (set, frozenset)):
        items = [to_wire(v) for v in value]
        try:
            items.sort()
        except TypeError:
            pass
        return {"set": items}
    raise TypeError(f"{value!r} is not a value the protocol carries: integers, strings, booleans, null, tuples, sets and maps")


def from_wire(wire):
    """The value of the checker's model that wire encodes."""
    if wire is None or isinstance(wire, (bool, int, str)):
        return wire
    if isinstance(wire, dict) and len(wire) == 1:
        (tag, items), = wire.items()
        if tag in _TAGS and isinstance(items, list):
            if tag == "tuple":
                return tuple(from_wire(v) for v in items)
            if tag == "set":
                return frozenset(from_wire(v) for v in items)
            if all(isinstance(kv, list) and len(kv) == 2 for kv in items):
                return FrozenMap((from_wire(k), from_wire(v)) for k, v in items)
    raise ValueError(f"{wire!r} is not a value in the tagged encoding: a scalar, or one of {list(_TAGS)} as the only key")


PROTOCOL = 1


class EvaluatorError(Exception):
    """The glue could not be started, speaks another protocol, or answered
    a request with an error."""


def _bindings_to_wire(bindings: list[dict]) -> list[dict]:
    return [{k: to_wire(v) for k, v in b.items()} for b in bindings]


def _bindings_from_wire(bindings: list[dict]) -> list[dict]:
    return [{k: from_wire(v) for k, v in b.items()} for b in bindings]


def request_to_wire(request: dict) -> dict:
    if "bindings" in request:
        return dict(request, bindings=_bindings_to_wire(request["bindings"]))
    return request


def request_from_wire(request: dict) -> dict:
    if "bindings" in request:
        return dict(request, bindings=_bindings_from_wire(request["bindings"]))
    return request


def answer_to_wire(op: str, answer: dict) -> dict:
    if op == "post" and "results" in answer:
        return dict(answer, results=[_bindings_to_wire(r) for r in answer["results"]])
    return answer


def answer_from_wire(op: str, answer: dict) -> dict:
    if op == "post" and "results" in answer:
        return dict(answer, results=[_bindings_from_wire(r) for r in answer["results"]])
    return answer


def serve(handle, stdin, stdout) -> None:
    """Answers the requests read from stdin, one JSON line each, with
    handle, and writes each answer as one JSON line to stdout. A glue
    written in Python runs this as its main loop."""
    for line in stdin:
        if not line.strip():
            continue
        request = json.loads(line)
        try:
            answer = answer_to_wire(request.get("op"), handle(request_from_wire(request)))
        except (TypeError, ValueError) as e:
            answer = {"error": f"{type(e).__name__}: {e}"}
        stdout.write(json.dumps(answer, ensure_ascii=False) + "\n")
        stdout.flush()


class Evaluator:
    """A glue the checker talks to. request sends one request with the
    checker's values and returns the answer with the checker's values, or
    raises EvaluatorError for an error answer."""

    language: str

    def _send(self, request: dict) -> dict:
        raise NotImplementedError

    def request(self, request: dict) -> dict:
        answer = self._send(request)
        if "error" in answer:
            raise EvaluatorError(f"the {getattr(self, 'language', 'clause')} glue answered {request.get('op')} of {request.get('clause')!r}: {answer['error']}")
        return answer

    def _hello(self) -> None:
        answer = self._send({"op": "hello"})
        if answer.get("protocol") != PROTOCOL:
            raise EvaluatorError(f"the glue speaks protocol {answer.get('protocol')!r}; csdfbmc speaks {PROTOCOL}")
        self.language = str(answer.get("language", "?"))

    def close(self) -> None:
        pass


class _Module(Evaluator):
    def __init__(self, spec: list[str]):
        if not spec:
            raise EvaluatorError("--evaluator-module needs a module name")
        try:
            module = importlib.import_module(spec[0])
        except ImportError as e:
            raise EvaluatorError(f"cannot import the glue module {spec[0]!r}: {e}") from e
        if not callable(getattr(module, "evaluator", None)):
            raise EvaluatorError(f"the glue module {spec[0]!r} defines no evaluator(args) function")
        self._handle = module.evaluator(spec[1:])
        self._hello()

    def _send(self, request: dict) -> dict:
        return self._handle(request)


class _Command(Evaluator):
    def __init__(self, argv: list[str], cwd: str | None):
        try:
            self._proc = subprocess.Popen(argv, stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True,
                                          encoding="utf-8", bufsize=1, cwd=cwd)
        except OSError as e:
            raise EvaluatorError(f"cannot start the glue {argv!r}: {e}") from e
        self._argv = argv
        try:
            self._hello()
        except EvaluatorError:
            self.close()
            raise

    def _send(self, request: dict) -> dict:
        op = request.get("op")
        try:
            self._proc.stdin.write(json.dumps(request_to_wire(request), ensure_ascii=False) + "\n")
            self._proc.stdin.flush()
            line = self._proc.stdout.readline()
        except (OSError, ValueError) as e:
            raise EvaluatorError(f"the glue {self._argv!r} stopped: {e}") from e
        if not line:
            raise EvaluatorError(f"the glue {self._argv!r} stopped without answering {op}")
        try:
            return answer_from_wire(op, json.loads(line))
        except ValueError as e:
            raise EvaluatorError(f"the glue {self._argv!r} answered {op} with {line.strip()!r}: {e}") from e

    def close(self) -> None:
        if self._proc.poll() is None:
            self._proc.stdin.close()
            try:
                self._proc.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self._proc.kill()
                self._proc.wait()
        self._proc.stdout.close()


def start(*, command: list[str] | None = None, module: list[str] | None = None, cwd: str | None = None) -> Evaluator:
    """The glue started as a command (its argv), or loaded in-process as a
    Python module ([module name, *args], calling its evaluator(args))."""
    if (command is None) == (module is None):
        raise ValueError("start a glue either as a command or as a module")
    return _Command(command, cwd) if command is not None else _Module(module)


def from_options(command: str | None, module: str | None) -> Evaluator | None:
    """The glue the options name: --evaluator's command line, or
    --evaluator-module's module name and arguments, each split as a shell
    would; None when neither is given."""
    if command is not None and module is not None:
        raise ValueError("give --evaluator or --evaluator-module, not both")
    if command is not None:
        return start(command=shlex.split(command))
    if module is not None:
        return start(module=shlex.split(module))
    return None
