"""The evaluator protocol: how the checker asks a clause language about its
clauses, independent of the language.

A glue is a long-running process that reads one JSON request per line on its
standard input and writes one JSON answer per line on its standard output,
in order. The requests are:

- {"op": "hello"} -> {"language": NAME, "protocol": 1}, first. It may add
  "supports": [OP, ...], the optional requests below the glue answers.
- {"op": "analyse", "clause": TEXT, "role": "guard" | "post"}
  -> {"reads": [NAME, ...], "writes": [NAME, ...]}: the names the clause
  reads and, for a post, the variables it gives a value to, known without
  evaluating it; or {"opaque": true} for a clause not yet formalised.
- {"op": "guard", "clause": TEXT, "bindings": [BINDING, ...]}
  -> {"results": [true | false, ...]}, one per binding.
- {"op": "post", "clause": TEXT, "bindings": [BINDING, ...]}
  -> {"results": [[ASSIGNMENT, ...], ...]}: per binding, the successor
  values of the clause's writes, each assignment giving every one of them.

Two requests are optional, for what the walk's reductions need to know of a
clause and only the clause's language can tell; the checker sends them only
to a glue whose hello supports them, and does without the reduction else:

- {"op": "beyond", "clause": TEXT, "role": ROLE, "own_id": NAME | null}
  -> {"reads": [NAME, ...]}: the names the clause reads, leaving out a map it
  reads only at the key own_id (partial-order reduction); a subset of what
  analyse says it reads. own_id is null when no name in the clause is the
  instance's own ID.
- {"op": "symmetric", "clause": TEXT, "role": ROLE, "params": [NAME, ...],
  "maps": [NAME, ...], "groups": [[ID, ...], ...]} -> {"symmetric": true}, or
  {"symmetric": false, "why": MESSAGE}: whether the clause tells apart two IDs
  of a group, which params range over and maps are keyed by (symmetry
  reduction). An ID is a string.

The names of beyond and symmetric are the clause's own, as the clause reads
them: under a renaming, the checker asks in the names the clause uses.

A binding and an assignment are JSON objects from names to values. Any
request may be answered with {"error": MESSAGE}. A clause must be pure and
deterministic: the checker may answer a request again from what it was told
for the same clause and the same values of the names it reads.

Values cross the protocol in a tagged JSON encoding, since JSON has no
tuples, sets or maps with keys other than strings: integers, strings,
booleans and null as themselves, and {"tuple": [...]}, {"set": [...]},
{"map": [[key, value], ...]} for the containers of csdfbmc.values.

Protocol 2 is the protocol of atoms (docs/CLAUSE_DESIGN.md). The checker
asks for it with {"op": "hello", "protocol": 2}, and a glue that speaks it
answers "protocol": 2. An ATOM is {"text": [TEXT, ...], "holes": [INDEX, ...],
"primed": [BOOL, ...]}: the text without its holes, one piece more than the
holes; the argument each hole reads; and whether each argument is a value
after the step. A glue never sees a name; values come by argument index:

- {"op": "guard", "atom": ATOM, "values": [[VALUE, ...], ...]}
  -> {"results": [true | false, ...]}, one per list of values.
- {"op": "post", "atom": ATOM, "values": [[VALUE, ...], ...]}, the values of
  the arguments before the step -> {"results": [[[VALUE, ...], ...], ...]}:
  per list, the lists of values after the step the atom allows, one value
  per primed argument, in order.
- {"op": "symmetric", "atom": ATOM, "params": [INDEX, ...], "maps": [INDEX,
  ...], "groups": [[ID, ...], ...]}, optional as in protocol 1.

There is no analyse and no beyond: what an atom reads is its holes, which the
tools know themselves.

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
from typing import Callable, Protocol

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
        # One order for any items, so that a set is always sent the same way.
        return {"set": sorted((to_wire(v) for v in value), key=lambda w: json.dumps(w, sort_keys=True, ensure_ascii=False))}
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
# The protocol of atoms (docs/CLAUSE_DESIGN.md): a request carries an atom and
# the values of its arguments by index, never a name.
PROTOCOL_ATOMS = 2


class EvaluatorError(Exception):
    """The glue could not be started, speaks another protocol, or answered
    a request with an error."""


def _bindings_to_wire(bindings: list[dict]) -> list[dict]:
    return [{k: to_wire(v) for k, v in b.items()} for b in bindings]


def _bindings_from_wire(bindings: list[dict]) -> list[dict]:
    return [{k: from_wire(v) for k, v in b.items()} for b in bindings]


def _values_to_wire(values: list[list]) -> list[list]:
    return [[to_wire(v) for v in vs] for vs in values]


def _values_from_wire(values: list[list]) -> list[list]:
    return [[from_wire(v) for v in vs] for vs in values]


def request_to_wire(request: dict) -> dict:
    if "bindings" in request:
        return dict(request, bindings=_bindings_to_wire(request["bindings"]))
    if "values" in request:
        return dict(request, values=_values_to_wire(request["values"]))
    return request


def request_from_wire(request: dict) -> dict:
    if "bindings" in request:
        return dict(request, bindings=_bindings_from_wire(request["bindings"]))
    if "values" in request:
        return dict(request, values=_values_from_wire(request["values"]))
    return request


def answer_to_wire(request: dict, answer: dict) -> dict:
    if request.get("op") == "post" and "results" in answer:
        encode = _values_to_wire if "atom" in request else _bindings_to_wire
        return dict(answer, results=[encode(r) for r in answer["results"]])
    return answer


def answer_from_wire(request: dict, answer: dict) -> dict:
    if request.get("op") == "post" and "results" in answer:
        decode = _values_from_wire if "atom" in request else _bindings_from_wire
        return dict(answer, results=[decode(r) for r in answer["results"]])
    return answer


# Handle: what a glue is to the checker, a function from a request to its
# answer, with the checker's values in place of their encoding.
Handle = Callable[[dict], dict]


def handler(language: str, *, analyse: Callable[[str, str], "Analysis"], guard: Callable[[str, dict], bool],
            post: Callable[[str, dict], list[dict]],
            beyond: Callable[[str, str, str | None], list[str]] | None = None,
            symmetric: Callable[[str, str, list[str], list[str], list[list[str]]], str | None] | None = None,
            atom_guard: Callable[[dict, list], bool] | None = None,
            atom_post: Callable[[dict, list], list[list]] | None = None,
            atom_symmetric: Callable[[dict, list[int], list[int], list[list[str]]], str | None] | None = None) -> Handle:
    """The Handle of a glue, from what is particular to its language:
    analyse(text, role) says what a clause reads and writes, guard(text,
    binding) and post(text, binding) evaluate a clause under one binding.
    beyond(text, role, own_id) and symmetric(text, role, params, maps,
    groups), when given, answer what the walk's reductions ask (None from
    symmetric: the clause tells no two IDs apart; a message: how it does).
    hello, the batches and the error answers are the protocol's and are the
    same for every glue."""
    supports = [op for op, fn in (("beyond", beyond), ("symmetric", symmetric)) if fn is not None]

    def handle(request: dict) -> dict:
        op = request.get("op")
        try:
            if op == "hello" and request.get("protocol") == PROTOCOL_ATOMS and atom_guard and atom_post:
                hello = {"language": language, "protocol": PROTOCOL_ATOMS}
                return dict(hello, supports=["symmetric"]) if atom_symmetric else hello
            if op == "hello":
                hello = {"language": language, "protocol": PROTOCOL}
                return dict(hello, supports=supports) if supports else hello
            if op == "beyond" and beyond is not None:
                return {"reads": beyond(request["clause"], request["role"], request["own_id"])}
            if "atom" in request and op == "symmetric" and atom_symmetric is not None:
                why = atom_symmetric(request["atom"], request["params"], request["maps"], request["groups"])
                return {"symmetric": True} if why is None else {"symmetric": False, "why": why}
            if op == "symmetric" and symmetric is not None:
                why = symmetric(request["clause"], request["role"], request["params"], request["maps"], request["groups"])
                return {"symmetric": True} if why is None else {"symmetric": False, "why": why}
            if "atom" in request and op == "guard" and atom_guard is not None:
                return {"results": [atom_guard(request["atom"], vs) for vs in request["values"]]}
            if "atom" in request and op == "post" and atom_post is not None:
                return {"results": [atom_post(request["atom"], vs) for vs in request["values"]]}
            if op == "analyse":
                a = analyse(request["clause"], request["role"])
                return {"opaque": True} if a.opaque else {"reads": list(a.reads), "writes": list(a.writes)}
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
        try:
            request = json.loads(line)
            if not isinstance(request, dict):
                raise ValueError(f"a request is a JSON object, not {request!r}")
            answer = answer_to_wire(request, handle(request_from_wire(request)))
        except Exception as e:  # every line gets an answer, so the checker is never left waiting
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
            return answer_from_wire(request, json.loads(line))
        except ValueError as e:
            raise EvaluatorError(f"the glue {self._argv!r} answered {op} with {line.strip()!r}: {e}") from e

    def close(self) -> None:
        try:
            self._proc.stdin.close()
        except OSError:
            pass  # the glue stopped already; its end of the pipe is gone
        if self._proc.poll() is None:
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


class ClientLike(Protocol):
    """What the checker needs of a glue: Client, and the decorators that
    wrap one."""
    language: str
    supports: frozenset[str]

    def analyse(self, clause: str, role: str) -> "Analysis": ...
    def beyond(self, clause: str, role: str, own_id: str | None) -> tuple[str, ...]: ...
    def symmetric(self, clause: str, role: str, params: list[str], maps: list[str], groups: list[list[str]]) -> str | None: ...
    def guard(self, clause: str, bindings: list[dict]) -> list[bool]: ...
    def post(self, clause: str, bindings: list[dict]) -> list[list[dict]]: ...
    def close(self) -> None: ...


def answers(client: "ClientLike", op: str) -> bool:
    """Whether the glue behind client answers the optional request op, as
    its hello said."""
    return op in client.supports


def NO_CLOSE() -> None:
    """The close of a glue that holds nothing to release, such as one
    loaded in-process."""


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

    def __init__(self, handle: Handle, close: Callable[[], None]):
        self._handle = handle
        self._close = close
        try:
            answer = handle({"op": "hello"})
            if not isinstance(answer, dict):
                raise EvaluatorError(f"the glue answered hello with {answer!r}, not an object")
            if answer.get("protocol") != PROTOCOL:
                raise EvaluatorError(f"the glue speaks protocol {answer.get('protocol')!r}; csdfbmc speaks {PROTOCOL}")
            if not isinstance(answer.get("language"), str) or not answer["language"]:
                raise EvaluatorError(f"the glue's hello names no language: {answer!r}")
            self.language = answer["language"]
            supports = answer.get("supports", [])
            if not isinstance(supports, list) or not all(isinstance(op, str) for op in supports):
                raise EvaluatorError(f"the glue's hello says it supports {supports!r}, not a list of requests")
            self.supports = frozenset(supports)
        except EvaluatorError:
            close()
            raise

    def _request(self, request: dict) -> dict:
        answer = self._handle(request)
        if not isinstance(answer, dict):
            raise self._malformed(request["op"], request.get("clause"), answer, "an answer is an object")
        if "error" in answer:
            if not isinstance(answer["error"], str):
                raise self._malformed(request["op"], request.get("clause"), answer, "an error is a message")
            raise EvaluatorError(f"the {self.language} glue answered {request.get('op')} of {request.get('clause')!r}: {answer['error']}")
        return answer

    def _malformed(self, op: str, clause: str, answer, why: str) -> EvaluatorError:
        return EvaluatorError(f"the {self.language} glue answered {op} of {clause!r} with {answer!r}: {why}")

    def analyse(self, clause: str, role: str) -> Analysis:
        answer = self._request({"op": "analyse", "clause": clause, "role": role})
        if answer.get("opaque") is True:
            return Analysis(opaque=True)
        names = [answer.get("reads"), answer.get("writes")]
        if not all(isinstance(ns, list) and all(isinstance(n, str) for n in ns) for ns in names):
            raise self._malformed("analyse", clause, answer, "reads and writes must be lists of names, or opaque true")
        return Analysis(opaque=False, reads=tuple(answer["reads"]), writes=tuple(answer["writes"]))

    def _results(self, op: str, clause: str, bindings: list[dict]) -> list:
        answer = self._request({"op": op, "clause": clause, "bindings": bindings})
        results = answer.get("results")
        if not isinstance(results, list) or len(results) != len(bindings):
            raise self._malformed(op, clause, answer, f"want one result per binding, {len(bindings)} in all")
        return results

    def guard(self, clause: str, bindings: list[dict]) -> list[bool]:
        results = self._results("guard", clause, bindings)
        if not all(isinstance(r, bool) for r in results):
            raise self._malformed("guard", clause, results, "a guard is a truth value")
        return results

    def post(self, clause: str, bindings: list[dict]) -> list[list[dict]]:
        results = self._results("post", clause, bindings)
        if not all(isinstance(r, list) and all(isinstance(a, Mapping) for a in r) for r in results):
            raise self._malformed("post", clause, results, "a post is a list of assignments per binding")
        return results

    def _supported(self, op: str, clause: str) -> None:
        if op not in self.supports:
            raise EvaluatorError(f"the {self.language} glue does not answer {op}, which was asked of {clause!r}")

    def beyond(self, clause: str, role: str, own_id: str | None) -> tuple[str, ...]:
        """The names the clause reads, but a map it reads only at own_id."""
        self._supported("beyond", clause)
        answer = self._request({"op": "beyond", "clause": clause, "role": role, "own_id": own_id})
        reads = answer.get("reads")
        if not isinstance(reads, list) or not all(isinstance(n, str) for n in reads):
            raise self._malformed("beyond", clause, answer, "reads must be a list of names")
        return tuple(reads)

    def symmetric(self, clause: str, role: str, params: list[str], maps: list[str], groups: list[list[str]]) -> str | None:
        """How the clause tells two IDs of a group apart, or None when it
        does not."""
        self._supported("symmetric", clause)
        answer = self._request({"op": "symmetric", "clause": clause, "role": role, "params": params, "maps": maps, "groups": groups})
        if answer.get("symmetric") is True:
            return None
        if answer.get("symmetric") is not False or not isinstance(answer.get("why"), str):
            raise self._malformed("symmetric", clause, answer, "symmetric is true, or false with a why")
        return answer["why"]

    def close(self) -> None:
        self._close()

    def __enter__(self) -> "Client":
        return self

    def __exit__(self, *exc) -> None:
        self.close()


class AtomClient:
    """The checker's side of protocol 2 over a Handle: atoms and the values
    of their arguments by index. It says hello asking for protocol 2 and
    refuses a glue that answers with another."""

    def __init__(self, handle: Handle, close: Callable[[], None]):
        self._handle = handle
        self._close = close
        try:
            answer = handle({"op": "hello", "protocol": PROTOCOL_ATOMS})
            if not isinstance(answer, dict) or answer.get("protocol") != PROTOCOL_ATOMS:
                raise EvaluatorError(f"the glue answered hello with {answer!r}; csdfbmc reads atoms through protocol {PROTOCOL_ATOMS}")
            if not isinstance(answer.get("language"), str) or not answer["language"]:
                raise EvaluatorError(f"the glue's hello names no language: {answer!r}")
            self.language = answer["language"]
            supports = answer.get("supports", [])
            if not isinstance(supports, list) or not all(isinstance(op, str) for op in supports):
                raise EvaluatorError(f"the glue's hello says it supports {supports!r}, not a list of requests")
            self.supports = frozenset(supports)
        except EvaluatorError:
            close()
            raise

    def _request(self, request: dict) -> dict:
        answer = self._handle(request)
        if not isinstance(answer, dict):
            raise self._malformed(request, answer, "an answer is an object")
        if "error" in answer:
            if not isinstance(answer["error"], str):
                raise self._malformed(request, answer, "an error is a message")
            raise EvaluatorError(f"the {self.language} glue answered {request['op']} of the atom {_shown(request['atom'])}: {answer['error']}")
        return answer

    def _malformed(self, request: dict, answer, why: str) -> EvaluatorError:
        return EvaluatorError(f"the {self.language} glue answered {request['op']} of the atom {_shown(request['atom'])} with {answer!r}: {why}")

    def _results(self, op: str, atom: dict, values: list[list]) -> list:
        request = {"op": op, "atom": atom, "values": values}
        answer = self._request(request)
        results = answer.get("results")
        if not isinstance(results, list) or len(results) != len(values):
            raise self._malformed(request, answer, f"want one result per list of values, {len(values)} in all")
        return results

    def guard(self, atom: dict, values: list[list]) -> list[bool]:
        results = self._results("guard", atom, values)
        if not all(isinstance(r, bool) for r in results):
            raise self._malformed({"op": "guard", "atom": atom}, results, "a guard is a truth value")
        return results

    def post(self, atom: dict, values: list[list]) -> list[list[list]]:
        """Per list of values before the step, the lists of values after it
        the atom allows, of its primed arguments in order."""
        results = self._results("post", atom, values)
        if not all(isinstance(r, list) and all(isinstance(a, list) for a in r) for r in results):
            raise self._malformed({"op": "post", "atom": atom}, results, "a post is a list of lists of values per list of values")
        return results

    def symmetric(self, atom: dict, params: list[int], maps: list[int], groups: list[list[str]]) -> str | None:
        """How the atom tells two IDs of a group apart, or None when it does
        not; params and maps are indices of its arguments."""
        if "symmetric" not in self.supports:
            raise EvaluatorError(f"the {self.language} glue does not answer symmetric, which was asked of the atom {_shown(atom)}")
        request = {"op": "symmetric", "atom": atom, "params": params, "maps": maps, "groups": groups}
        answer = self._request(request)
        if answer.get("symmetric") is True:
            return None
        if answer.get("symmetric") is not False or not isinstance(answer.get("why"), str):
            raise self._malformed(request, answer, "symmetric is true, or false with a why")
        return answer["why"]

    def close(self) -> None:
        self._close()


def _shown(atom: dict) -> str:
    """An atom as a message shows it, its holes as $0, $1, … by argument."""
    text = atom.get("text", [])
    holes = atom.get("holes", [])
    return repr(text[0] + "".join(f"${h}" + t for h, t in zip(holes, text[1:]))) if text else repr(atom)


def start_command(argv: list[str], *, cwd: str | None = None, env: dict | None = None) -> Client:
    """A Client of the glue run as the command argv."""
    transport = CommandTransport(argv, cwd=cwd, env=env)
    return Client(transport, transport.close)


def start_module(name: str, args: list[str]) -> Client:
    """A Client of the glue a Python module's evaluator(args) returns, in-process."""
    return Client(module_handle(name, args), NO_CLOSE)


class Validating:
    """A Client whose post answers are checked against the clause's writes:
    every assignment must give a value to exactly them."""

    def __init__(self, inner: ClientLike):
        self.inner = inner
        self.language = inner.language
        self.supports = inner.supports

    def analyse(self, clause: str, role: str) -> Analysis:
        return self.inner.analyse(clause, role)

    def beyond(self, clause: str, role: str, own_id: str | None) -> tuple[str, ...]:
        """The glue's answer, refused unless it is among what the clause
        reads: a name read beyond the instance is a name read."""
        names = self.inner.beyond(clause, role, own_id)
        reads = set(self.analyse(clause, role).reads)
        if not set(names) <= reads:
            raise EvaluatorError(f"the {self.language} glue says {clause!r} reads {sorted(set(names) - reads)} beyond its instance, which it does not say the clause reads at all")
        return names

    def symmetric(self, clause: str, role: str, params: list[str], maps: list[str], groups: list[list[str]]) -> str | None:
        return self.inner.symmetric(clause, role, params, maps, groups)

    def guard(self, clause: str, bindings: list[dict]) -> list:
        return self.inner.guard(clause, bindings)

    def post(self, clause: str, bindings: list[dict]) -> list[list[dict]]:
        results = self.inner.post(clause, bindings)
        writes = set(self.analyse(clause, "post").writes)
        for result in results:
            for a in result:
                if set(a) != writes:
                    raise EvaluatorError(f"the {self.language} glue answered the post clause {clause!r} with {a!r}, which does not give a value to exactly {sorted(writes)}")
        return results

    def close(self) -> None:
        self.inner.close()


class Memoising:
    """A Client that asks about a clause once per value of the names it
    reads, which a pure clause allows, and asks about the bindings of one
    batch it was not asked about yet in one request."""

    def __init__(self, inner: ClientLike):
        self.inner = inner
        self.language = inner.language
        self.supports = inner.supports
        self._analyses: dict[tuple[str, str], Analysis] = {}
        self._answers: dict[tuple, object] = {}
        self._beyond: dict[tuple, tuple[str, ...]] = {}

    def beyond(self, clause: str, role: str, own_id: str | None) -> tuple[str, ...]:
        key = (clause, role, own_id)
        if key not in self._beyond:
            self._beyond[key] = self.inner.beyond(clause, role, own_id)
        return self._beyond[key]

    def symmetric(self, clause: str, role: str, params: list[str], maps: list[str], groups: list[list[str]]) -> str | None:
        # Not kept: the symmetry check asks of each clause once, before a walk.
        return self.inner.symmetric(clause, role, params, maps, groups)

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


def checked(client: ClientLike) -> ClientLike:
    """client as the walk asks it: each answer checked, and each clause asked
    once per value of the names it reads."""
    return Memoising(Validating(client))


class ValidatingAtoms:
    """An AtomClient whose post answers are checked against the atom: each
    gives exactly one value per primed argument."""

    def __init__(self, inner: AtomClient):
        self.inner = inner
        self.language = inner.language
        self.supports = inner.supports

    def guard(self, atom: dict, values: list[list]) -> list[bool]:
        return self.inner.guard(atom, values)

    def post(self, atom: dict, values: list[list]) -> list[list[list]]:
        results = self.inner.post(atom, values)
        want = sum(atom["primed"])
        for result in results:
            for after in result:
                if len(after) != want:
                    raise EvaluatorError(f"the {self.language} glue answered the post atom {_shown(atom)} with {after!r}, "
                                         f"which does not give {want} value after the step, one per primed argument")
        return results

    def symmetric(self, atom: dict, params: list[int], maps: list[int], groups: list[list[str]]) -> str | None:
        return self.inner.symmetric(atom, params, maps, groups)

    def close(self) -> None:
        self.inner.close()


class MemoisingAtoms:
    """An AtomClient that asks about an atom once per list of values, which a
    pure atom allows, and asks about the lists of one batch it was not asked
    about yet in one request."""

    def __init__(self, inner):
        self.inner = inner
        self.language = inner.language
        self.supports = inner.supports
        self._answers: dict[tuple, object] = {}

    def _ask(self, op: str, atom: dict, values: list[list]) -> list:
        shape = json.dumps(atom, sort_keys=True, ensure_ascii=False)
        keys = [(op, shape, tuple(vs)) for vs in values]
        asked: dict[tuple, list] = {}
        for key, vs in zip(keys, values):
            if key not in self._answers:
                asked.setdefault(key, vs)
        if asked:
            ask = self.inner.guard if op == "guard" else self.inner.post
            for key, result in zip(asked, ask(atom, list(asked.values()))):
                self._answers[key] = result
        return [self._answers[key] for key in keys]

    def guard(self, atom: dict, values: list[list]) -> list[bool]:
        return self._ask("guard", atom, values)

    def post(self, atom: dict, values: list[list]) -> list[list[list]]:
        return self._ask("post", atom, values)

    def symmetric(self, atom: dict, params: list[int], maps: list[int], groups: list[list[str]]) -> str | None:
        # Not kept: the symmetry check asks of each atom once, before a walk.
        return self.inner.symmetric(atom, params, maps, groups)

    def close(self) -> None:
        self.inner.close()


def checked_atoms(client: AtomClient) -> MemoisingAtoms:
    """client as a walk asks it: each answer checked, and each atom asked
    once per list of values."""
    return MemoisingAtoms(ValidatingAtoms(client))
