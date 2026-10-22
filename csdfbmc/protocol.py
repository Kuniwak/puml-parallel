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

hello comes first and agrees on one protocol for the rest of the session: a
glue refuses a request before it, a hello asking a protocol it does not
speak, and a request of the other protocol (an atom under protocol 1, a
clause, analyse or beyond under protocol 2).

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


def answer_to_wire(request: dict, answer: dict, protocol: int) -> dict:
    """answer to request in the tagged encoding; the results of a post are
    lists of values in the protocol of atoms, and assignments in protocol 1."""
    if request.get("op") == "post" and "results" in answer:
        encode = _values_to_wire if protocol == PROTOCOL_ATOMS else _bindings_to_wire
        return dict(answer, results=[encode(r) for r in answer["results"]])
    return answer


def answer_from_wire(request: dict, answer: dict, protocol: int) -> dict:
    if request.get("op") == "post" and "results" in answer:
        decode = _values_from_wire if protocol == PROTOCOL_ATOMS else _bindings_from_wire
        return dict(answer, results=[decode(r) for r in answer["results"]])
    return answer


# Handle: what a glue is to the checker, a function from a request to its
# answer, with the checker's values in place of their encoding.
Handle = Callable[[dict], dict]


def handler(language: str, *, analyse: Callable[[str, str], "Analysis"], guard: Callable[[str, dict], bool],
            post: Callable[[str, dict], list[dict]],
            beyond: Callable[[str, str, str | None], list[str]] | None = None,
            symmetric: Callable[[str, str, list[str], list[str], list[list[str]]], str | None] | None = None) -> Handle:
    """The Handle of a glue speaking protocol 1, from what is particular to
    its language: analyse(text, role) says what a clause reads and writes,
    guard(text, binding) and post(text, binding) evaluate a clause under one
    binding. beyond(text, role, own_id) and symmetric(text, role, params,
    maps, groups), when given, answer what the walk's reductions ask (None
    from symmetric: the clause tells no two IDs apart; a message: how it
    does). hello, the batches and the error answers are the protocol's and
    are the same for every glue. A request of protocol 2 is refused."""
    supports = [op for op, fn in (("beyond", beyond), ("symmetric", symmetric)) if fn is not None]

    def handle(request: dict) -> dict:
        op = request.get("op")
        try:
            if op == "hello":
                if request.get("protocol", PROTOCOL) != PROTOCOL:
                    return {"error": f"this glue speaks protocol {PROTOCOL}, not {request.get('protocol')!r}"}
                hello = {"language": language, "protocol": PROTOCOL}
                return dict(hello, supports=supports) if supports else hello
            if "atom" in request:
                return {"error": f"a request of protocol {PROTOCOL} carries a clause, not an atom"}
            if op == "beyond" and beyond is not None:
                return {"reads": beyond(request["clause"], request["role"], request["own_id"])}
            if op == "symmetric" and symmetric is not None:
                why = symmetric(request["clause"], request["role"], request["params"], request["maps"], request["groups"])
                return {"symmetric": True} if why is None else {"symmetric": False, "why": why}
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


AtomSymmetric = Callable[[dict, list[int], list[int], list[list[str]]], "str | None"]


def NO_SYMMETRIC(atom: dict, params: list[int], maps: list[int], groups: list[list[str]]) -> str | None:
    """The symmetric of a glue that does not answer it: atom_handler leaves it
    out of hello's supports and refuses the request."""
    raise EvaluatorError("this glue does not answer symmetric")


def atom_handler(language: str, *, guard: Callable[[dict, list], bool], post: Callable[[dict, list], list[list]],
                 symmetric: AtomSymmetric) -> Handle:
    """The Handle of a glue speaking protocol 2, the protocol of atoms:
    guard(atom, values) and post(atom, values) evaluate an atom under one
    list of values of its arguments, symmetric(atom, params, maps, groups)
    answers what symmetry reduction asks, or is NO_SYMMETRIC. A request of
    protocol 1, which names a clause, is refused."""

    def handle(request: dict) -> dict:
        op = request.get("op")
        try:
            if op == "hello":
                if request.get("protocol") != PROTOCOL_ATOMS:
                    return {"error": f"this glue speaks protocol {PROTOCOL_ATOMS}, not {request.get('protocol', PROTOCOL)!r}"}
                hello = {"language": language, "protocol": PROTOCOL_ATOMS}
                return hello if symmetric is NO_SYMMETRIC else dict(hello, supports=["symmetric"])
            if "atom" not in request:
                return {"error": f"a request of protocol {PROTOCOL_ATOMS} carries an atom; {op!r} names none"}
            if op == "guard":
                return {"results": [guard(request["atom"], vs) for vs in request["values"]]}
            if op == "post":
                return {"results": [post(request["atom"], vs) for vs in request["values"]]}
            if op == "symmetric":
                why = symmetric(request["atom"], request["params"], request["maps"], request["groups"])
                return {"symmetric": True} if why is None else {"symmetric": False, "why": why}
        except Exception as e:  # what the atom raised is the answer, reported with it
            return {"error": f"{type(e).__name__}: {e}"}
        return {"error": f"protocol {PROTOCOL_ATOMS} has no request {op!r}"}
    return handle


def speaking(handles: dict[int, Handle]) -> Handle:
    """The Handle of a glue that speaks each protocol of handles, by its
    number: hello picks the one it asks for, protocol 1 when it names none,
    and every later request goes to that one. A request before hello is
    refused, and so is a hello asking a protocol not among them."""
    agreed: list[Handle] = []

    def handle(request: dict) -> dict:
        if request.get("op") == "hello":
            asked = request.get("protocol", PROTOCOL)
            if asked not in handles:
                spoken = ", ".join(str(p) for p in sorted(handles))
                return {"error": f"this glue speaks protocol {spoken}, not {asked!r}"}
            agreed[:] = [handles[asked]]
            return agreed[0](request)
        if not agreed:
            return {"error": "say hello first: the protocol is agreed on by hello"}
        return agreed[0](request)
    return handle


def agreed_protocol(request: dict, answer: dict, protocol: int) -> int:
    """The protocol in force after answer to request, protocol before it:
    the one a hello answer names."""
    if request.get("op") == "hello" and isinstance(answer, dict) and isinstance(answer.get("protocol"), int):
        return answer["protocol"]
    return protocol


def serve(handle: Handle, stdin, stdout) -> None:
    """Answers the requests read from stdin, one JSON line each, with
    handle, and writes each answer as one JSON line to stdout, encoded in
    the protocol hello agreed on. A glue written in Python runs this as its
    main loop."""
    protocol = PROTOCOL
    for line in stdin:
        if not line.strip():
            continue
        try:
            request = json.loads(line)
            if not isinstance(request, dict):
                raise ValueError(f"a request is a JSON object, not {request!r}")
            answer = handle(request_from_wire(request))
            protocol = agreed_protocol(request, answer, protocol)
            answer = answer_to_wire(request, answer, protocol)
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
        self._protocol = PROTOCOL

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
            answer = json.loads(line)
            self._protocol = agreed_protocol(request, answer, self._protocol)
            return answer_from_wire(request, answer, self._protocol)
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


class _Session:
    """What both protocols' clients share: hello first, refusing a glue that
    answers with another protocol or names no language, closing the glue if
    hello fails; then requests whose error answers are raised as
    EvaluatorError, each message showing the request's subject."""

    def __init__(self, handle: Handle, close: Callable[[], None], *, hello: dict,
                 wrong_protocol: Callable[[object], str], subject: Callable[[dict], str]):
        self._handle = handle
        self._close = close
        self._subject = subject
        try:
            answer = handle(hello)
            if not isinstance(answer, dict) or answer.get("protocol") != hello.get("protocol", PROTOCOL):
                raise EvaluatorError(wrong_protocol(answer))
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
            raise EvaluatorError(f"the {self.language} glue answered {request.get('op')} of {self._subject(request)}: {answer['error']}")
        return answer

    def _malformed(self, request: dict, answer, why: str) -> EvaluatorError:
        return EvaluatorError(f"the {self.language} glue answered {request.get('op')} of {self._subject(request)} with {answer!r}: {why}")

    def _supported(self, request: dict) -> None:
        if request["op"] not in self.supports:
            raise EvaluatorError(f"the {self.language} glue does not answer {request['op']}, which was asked of {self._subject(request)}")

    def _symmetric(self, request: dict) -> str | None:
        self._supported(request)
        answer = self._request(request)
        if answer.get("symmetric") is True:
            return None
        if answer.get("symmetric") is not False or not isinstance(answer.get("why"), str):
            raise self._malformed(request, answer, "symmetric is true, or false with a why")
        return answer["why"]

    def close(self) -> None:
        self._close()

    def __enter__(self):
        return self

    def __exit__(self, *exc) -> None:
        self.close()


def _wrong_protocol_1(answer) -> str:
    if not isinstance(answer, dict):
        return f"the glue answered hello with {answer!r}, not an object"
    return f"the glue speaks protocol {answer.get('protocol')!r}; csdfbmc speaks {PROTOCOL}"


class Client(_Session):
    """The checker's side of the protocol over a Handle: the requests as
    typed operations, an error answer raised as EvaluatorError. It says
    hello first and refuses a glue that speaks another protocol version."""

    def __init__(self, handle: Handle, close: Callable[[], None]):
        super().__init__(handle, close, hello={"op": "hello"},
                         wrong_protocol=_wrong_protocol_1,
                         subject=lambda r: repr(r.get("clause")))

    def analyse(self, clause: str, role: str) -> Analysis:
        request = {"op": "analyse", "clause": clause, "role": role}
        answer = self._request(request)
        if answer.get("opaque") is True:
            return Analysis(opaque=True)
        names = [answer.get("reads"), answer.get("writes")]
        if not all(isinstance(ns, list) and all(isinstance(n, str) for n in ns) for ns in names):
            raise self._malformed(request, answer, "reads and writes must be lists of names, or opaque true")
        return Analysis(opaque=False, reads=tuple(answer["reads"]), writes=tuple(answer["writes"]))

    def _results(self, op: str, clause: str, bindings: list[dict]) -> list:
        request = {"op": op, "clause": clause, "bindings": bindings}
        answer = self._request(request)
        results = answer.get("results")
        if not isinstance(results, list) or len(results) != len(bindings):
            raise self._malformed(request, answer, f"want one result per binding, {len(bindings)} in all")
        return results

    def guard(self, clause: str, bindings: list[dict]) -> list[bool]:
        results = self._results("guard", clause, bindings)
        if not all(isinstance(r, bool) for r in results):
            raise self._malformed({"op": "guard", "clause": clause}, results, "a guard is a truth value")
        return results

    def post(self, clause: str, bindings: list[dict]) -> list[list[dict]]:
        results = self._results("post", clause, bindings)
        if not all(isinstance(r, list) and all(isinstance(a, Mapping) for a in r) for r in results):
            raise self._malformed({"op": "post", "clause": clause}, results, "a post is a list of assignments per binding")
        return results

    def beyond(self, clause: str, role: str, own_id: str | None) -> tuple[str, ...]:
        """The names the clause reads, but a map it reads only at own_id."""
        request = {"op": "beyond", "clause": clause, "role": role, "own_id": own_id}
        self._supported(request)
        answer = self._request(request)
        reads = answer.get("reads")
        if not isinstance(reads, list) or not all(isinstance(n, str) for n in reads):
            raise self._malformed(request, answer, "reads must be a list of names")
        return tuple(reads)

    def symmetric(self, clause: str, role: str, params: list[str], maps: list[str], groups: list[list[str]]) -> str | None:
        """How the clause tells two IDs of a group apart, or None when it
        does not."""
        return self._symmetric({"op": "symmetric", "clause": clause, "role": role, "params": params, "maps": maps, "groups": groups})


class AtomClientLike(Protocol):
    """What the checker needs of a glue that speaks protocol 2: AtomClient,
    and the decorators that wrap one."""
    language: str
    supports: frozenset[str]

    def guard(self, atom: dict, values: list[list]) -> list[bool]: ...
    def post(self, atom: dict, values: list[list]) -> list[list[list]]: ...
    def symmetric(self, atom: dict, params: list[int], maps: list[int], groups: list[list[str]]) -> str | None: ...
    def close(self) -> None: ...


class AtomClient(_Session):
    """The checker's side of protocol 2 over a Handle: atoms and the values
    of their arguments by index. It says hello asking for protocol 2 and
    refuses a glue that answers with another."""

    def __init__(self, handle: Handle, close: Callable[[], None]):
        super().__init__(handle, close, hello={"op": "hello", "protocol": PROTOCOL_ATOMS},
                         wrong_protocol=lambda a: f"the glue answered hello with {a!r}; csdfbmc reads atoms through protocol {PROTOCOL_ATOMS}",
                         subject=lambda r: f"the atom {shown_atom(r['atom'])}")

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
        return self._symmetric({"op": "symmetric", "atom": atom, "params": params, "maps": maps, "groups": groups})


def atom_text(atom: dict, hole: Callable[[int], str]) -> str:
    """The text of atom with each hole written as hole(i), i the index of
    the argument it reads."""
    text = atom["text"]
    return text[0] + "".join(hole(h) + t for h, t in zip(atom["holes"], text[1:]))


def shown_atom(atom: dict) -> str:
    """An atom as a message shows it, its holes as $0, $1, … by argument."""
    return repr(atom_text(atom, lambda i: f"${i}")) if atom.get("text") else repr(atom)


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


def _typed_key(values: list) -> str:
    """values as a memo key that tells them apart as the protocol does: in
    Python, True == 1 and they hash alike, but a glue may answer differently
    for them."""
    return json.dumps([to_wire(v) for v in values], sort_keys=True, ensure_ascii=False)


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
        keys = [(clause, role, _typed_key([b[n] for n in reads])) for b in bindings]
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

    def __init__(self, inner: AtomClientLike):
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
                    raise EvaluatorError(f"the {self.language} glue answered the post atom {shown_atom(atom)} with {after!r}, "
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

    def __init__(self, inner: AtomClientLike):
        self.inner = inner
        self.language = inner.language
        self.supports = inner.supports
        self._answers: dict[tuple, object] = {}

    def _ask(self, op: str, atom: dict, values: list[list]) -> list:
        shape = json.dumps(atom, sort_keys=True, ensure_ascii=False)
        keys = [(op, shape, _typed_key(vs)) for vs in values]
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


def checked_atoms(client: AtomClientLike) -> AtomClientLike:
    """client as a walk asks it: each answer checked, and each atom asked
    once per list of values."""
    return MemoisingAtoms(ValidatingAtoms(client))
