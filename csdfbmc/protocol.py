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
import subprocess
from collections.abc import Mapping
from dataclasses import dataclass
from typing import Callable

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


# Handle: what a glue is to the checker, a function from a request to its
# answer, with the checker's values in place of their encoding.
Handle = Callable[[dict], dict]


def handler(language: str, *, analyse: Callable[[str, str], dict], guard: Callable[[str, dict], object],
            post: Callable[[str, dict], list]) -> Handle:
    """The Handle of a glue, from what is particular to its language:
    analyse(text, role) answers the analyse request, guard(text, binding)
    and post(text, binding) evaluate a clause under one binding. hello, the
    batches and the error answers are the protocol's and are the same for
    every glue."""

    def handle(request: dict) -> dict:
        op = request.get("op")
        try:
            if op == "hello":
                return {"language": language, "protocol": PROTOCOL}
            if op == "analyse":
                return analyse(request["clause"], request["role"])
            if op == "guard":
                return {"results": [guard(request["clause"], b) for b in request["bindings"]]}
            if op == "post":
                return {"results": [post(request["clause"], b) for b in request["bindings"]]}
        except Exception as e:  # what the clause raised is the answer, reported with it
            return {"error": f"{type(e).__name__}: {e}"}
        return {"error": f"unknown request {op!r}"}
    return handle


def serve(handle: Handle, stdin, stdout) -> None:
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


class CommandTransport:
    """The Handle of a glue run as a command, speaking JSON lines on its
    standard input and output."""

    def __init__(self, argv: list[str], *, cwd: str | None = None, env: dict | None = None):
        try:
            self._proc = subprocess.Popen(argv, stdin=subprocess.PIPE, stdout=subprocess.PIPE, text=True,
                                          encoding="utf-8", bufsize=1, cwd=cwd, env=env)
        except OSError as e:
            raise EvaluatorError(f"cannot start the glue {argv!r}: {e}") from e
        self._argv = argv

    def __call__(self, request: dict) -> dict:
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


def module_handle(name: str, args: list[str]) -> Handle:
    """The Handle a Python module's evaluator(args) returns, in-process."""
    try:
        module = importlib.import_module(name)
    except ImportError as e:
        raise EvaluatorError(f"cannot import the glue module {name!r}: {e}") from e
    if not callable(getattr(module, "evaluator", None)):
        raise EvaluatorError(f"the glue module {name!r} defines no evaluator(args) function")
    return module.evaluator(args)


@dataclass(frozen=True)
class Analysis:
    """What a glue says about a clause without evaluating it."""
    opaque: bool
    reads: tuple[str, ...] = ()
    writes: tuple[str, ...] = ()


class Client:
    """The checker's side of the protocol over a Handle: the requests as
    typed operations, an error answer raised as EvaluatorError. It says
    hello first and refuses a glue that speaks another protocol version."""

    def __init__(self, handle: Handle, close: Callable[[], None] = lambda: None):
        self._handle = handle
        self._close = close
        try:
            answer = handle({"op": "hello"})
            if answer.get("protocol") != PROTOCOL:
                raise EvaluatorError(f"the glue speaks protocol {answer.get('protocol')!r}; csdfbmc speaks {PROTOCOL}")
            self.language = str(answer.get("language", "?"))
        except EvaluatorError:
            close()
            raise

    def _request(self, request: dict) -> dict:
        answer = self._handle(request)
        if "error" in answer:
            raise EvaluatorError(f"the {self.language} glue answered {request.get('op')} of {request.get('clause')!r}: {answer['error']}")
        return answer

    def analyse(self, clause: str, role: str) -> Analysis:
        answer = self._request({"op": "analyse", "clause": clause, "role": role})
        if answer.get("opaque"):
            return Analysis(opaque=True)
        return Analysis(opaque=False, reads=tuple(answer["reads"]), writes=tuple(answer["writes"]))

    def guard(self, clause: str, bindings: list[dict]) -> list:
        return self._request({"op": "guard", "clause": clause, "bindings": bindings})["results"]

    def post(self, clause: str, bindings: list[dict]) -> list[list[dict]]:
        return self._request({"op": "post", "clause": clause, "bindings": bindings})["results"]

    def close(self) -> None:
        self._close()

    def __enter__(self) -> "Client":
        return self

    def __exit__(self, *exc) -> None:
        self.close()


def start_command(argv: list[str], *, cwd: str | None = None, env: dict | None = None) -> Client:
    """A Client of the glue run as the command argv."""
    transport = CommandTransport(argv, cwd=cwd, env=env)
    return Client(transport, transport.close)


def start_module(name: str, args: list[str]) -> Client:
    """A Client of the glue a Python module's evaluator(args) returns, in-process."""
    return Client(module_handle(name, args))


class Validating:
    """A Client whose post answers are checked against the clause's writes:
    every assignment must give a value to exactly them."""

    def __init__(self, inner):
        self.inner = inner
        self.language = inner.language
        self._analyses: dict[tuple[str, str], Analysis] = {}

    def analyse(self, clause: str, role: str) -> Analysis:
        key = (clause, role)
        if key not in self._analyses:
            self._analyses[key] = self.inner.analyse(clause, role)
        return self._analyses[key]

    def guard(self, clause: str, bindings: list[dict]) -> list:
        return self.inner.guard(clause, bindings)

    def post(self, clause: str, bindings: list[dict]) -> list[list[dict]]:
        results = self.inner.post(clause, bindings)
        writes = set(self.analyse(clause, "post").writes)
        for result in results:
            for a in result:
                if set(a) != writes:
                    raise ValueError(f"the glue answered the post clause {clause!r} with {a!r}, which does not give a value to exactly {sorted(writes)}")
        return results

    def close(self) -> None:
        self.inner.close()


class Memoising:
    """A Client that asks about a clause once per value of the names it
    reads, which a pure clause allows, and asks about the bindings of one
    batch it was not asked about yet in one request."""

    def __init__(self, inner):
        self.inner = inner
        self.language = inner.language
        self._analyses: dict[tuple[str, str], Analysis] = {}
        self._answers: dict[tuple, object] = {}

    def analyse(self, clause: str, role: str) -> Analysis:
        key = (clause, role)
        if key not in self._analyses:
            self._analyses[key] = self.inner.analyse(clause, role)
        return self._analyses[key]

    def _ask(self, role: str, clause: str, bindings: list[dict]) -> list:
        reads = self.analyse(clause, role).reads
        keys = [(clause, role, tuple(b[n] for n in reads)) for b in bindings]
        asked: dict[tuple, dict] = {}
        for key, b in zip(keys, bindings):
            if key not in self._answers:
                asked.setdefault(key, b)
        if asked:
            ask = self.inner.guard if role == "guard" else self.inner.post
            for key, result in zip(asked, ask(clause, list(asked.values()))):
                self._answers[key] = result
        return [self._answers[key] for key in keys]

    def guard(self, clause: str, bindings: list[dict]) -> list:
        return self._ask("guard", clause, bindings)

    def post(self, clause: str, bindings: list[dict]) -> list[list[dict]]:
        return self._ask("post", clause, bindings)

    def close(self) -> None:
        self.inner.close()
