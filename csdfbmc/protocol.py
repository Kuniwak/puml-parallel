"""The evaluator protocol: how the checker asks a clause language about its
atoms, independent of the language (docs/CLAUSE_DESIGN.md, P8).

A glue is a long-running process that reads one JSON request per line on its
standard input and writes one JSON answer per line on its standard output,
in order. An ATOM is {"text": [TEXT, ...], "holes": [INDEX, ...], "primed":
[BOOL, ...]}: the text without its holes, one piece more than the holes; the
argument each hole reads; and whether each argument is a value after the
step. A glue never sees a name; values come by argument index. The requests
are:

- {"op": "hello", "protocol": 2} -> {"language": NAME, "protocol": 2}, first.
  It may add "supports": [OP, ...], the optional requests below it answers.
- {"op": "guard", "atom": ATOM, "values": [[VALUE, ...], ...]}
  -> {"results": [true | false, ...]}, one per list of values.
- {"op": "post", "atom": ATOM, "values": [[VALUE, ...], ...]}, the values of
  the arguments before the step -> {"results": [[[VALUE, ...], ...], ...]}:
  per list, the lists of values after the step the atom allows, one value
  per primed argument, in order.

One request is optional, for symmetry reduction, which only the atom's
language can answer; the checker sends it only to a glue whose hello
supports it:

- {"op": "symmetric", "atom": ATOM, "params": [INDEX, ...], "maps": [INDEX,
  ...], "groups": [[ID, ...], ...]} -> {"symmetric": true}, or {"symmetric":
  false, "why": MESSAGE}: whether the atom tells apart two IDs of a group,
  which the arguments params range over and the arguments maps are keyed by.

What an atom reads is its holes, which the tools know themselves, so the
protocol asks nothing about it. Which atoms are functions of the environment
is the checks module's to say (csdfbmc.environment); every other atom is
the glue's, and an atom the glue cannot read is an error it answers.

Any request may be answered with {"error": MESSAGE}. An atom must be pure
and deterministic: the checker may answer a request again from what it was
told for the same atom and the same values.

Values cross the protocol in a tagged JSON encoding, since JSON has no
tuples, sets or maps with keys other than strings: integers, strings,
booleans and null as themselves, and {"tuple": [...]}, {"set": [...]},
{"map": [[key, value], ...]} for the containers of csdfbmc.values. An
instance of a family crosses as the map of its fields: its state is a
position, which a glue never sees (docs/CLAUSE_DESIGN.md, control state).

hello comes first and agrees on the protocol for the rest of the session: a
glue refuses a request before it and a hello asking a protocol it does not
speak. Protocol 1, which named clauses and asked analyse and beyond, is no
longer spoken.

A glue written in Python may also be loaded in-process: a module whose
evaluator(args) returns a function from a request to its answer, with the
checker's values in place of their encoding. csdfbmc.python_glue is both.
"""
from __future__ import annotations

import importlib
import json
import subprocess
from collections.abc import Mapping
from typing import Callable, Protocol

from csdfbmc.values import FrozenMap

_TAGS = ("tuple", "set", "map")

# The protocol of atoms, the one the checker speaks.
PROTOCOL = 2


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


class EvaluatorError(Exception):
    """The glue could not be started, speaks another protocol, or answered
    a request with an error."""


def _values_to_wire(values: list[list]) -> list[list]:
    return [[to_wire(v) for v in vs] for vs in values]


def _values_from_wire(values: list[list]) -> list[list]:
    return [[from_wire(v) for v in vs] for vs in values]


def request_to_wire(request: dict) -> dict:
    if "values" in request:
        return dict(request, values=_values_to_wire(request["values"]))
    return request


def request_from_wire(request: dict) -> dict:
    if "values" in request:
        return dict(request, values=_values_from_wire(request["values"]))
    return request


def answer_to_wire(request: dict, answer: dict) -> dict:
    """answer to request in the tagged encoding: the results of a post are
    lists of values."""
    if request.get("op") == "post" and "results" in answer:
        return dict(answer, results=[_values_to_wire(r) for r in answer["results"]])
    return answer


def answer_from_wire(request: dict, answer: dict) -> dict:
    if request.get("op") == "post" and "results" in answer:
        return dict(answer, results=[_values_from_wire(r) for r in answer["results"]])
    return answer


# Handle: what a glue is to the checker, a function from a request to its
# answer, with the checker's values in place of their encoding.
Handle = Callable[[dict], dict]

AtomSymmetric = Callable[[dict, list[int], list[int], list[list[str]]], "str | None"]


def NO_SYMMETRIC(atom: dict, params: list[int], maps: list[int], groups: list[list[str]]) -> str | None:
    """The symmetric of a glue that does not answer it: atom_handler leaves it
    out of hello's supports and refuses the request."""
    raise EvaluatorError("this glue does not answer symmetric")


def atom_handler(language: str, *, guard: Callable[[dict, list], bool], post: Callable[[dict, list], list[list]],
                 symmetric: AtomSymmetric) -> Handle:
    """The Handle of a glue: guard(atom, values) and post(atom, values)
    evaluate an atom under one list of values of its arguments,
    symmetric(atom, params, maps, groups) answers what symmetry reduction
    asks, or is NO_SYMMETRIC. hello comes first, and a request before it is
    refused."""
    greeted: list[bool] = []

    def handle(request: dict) -> dict:
        op = request.get("op")
        try:
            if op == "hello":
                if request.get("protocol") != PROTOCOL:
                    return {"error": f"this glue speaks protocol {PROTOCOL}, not {request.get('protocol', 1)!r}"}
                greeted[:] = [True]
                hello = {"language": language, "protocol": PROTOCOL}
                return hello if symmetric is NO_SYMMETRIC else dict(hello, supports=["symmetric"])
            if not greeted:
                return {"error": "say hello first: the protocol is agreed on by hello"}
            if "atom" not in request:
                return {"error": f"a request of protocol {PROTOCOL} carries an atom; {op!r} names none"}
            if op == "guard":
                return {"results": [guard(request["atom"], vs) for vs in request["values"]]}
            if op == "post":
                return {"results": [post(request["atom"], vs) for vs in request["values"]]}
            if op == "symmetric" and symmetric is not NO_SYMMETRIC:
                why = symmetric(request["atom"], request["params"], request["maps"], request["groups"])
                return {"symmetric": True} if why is None else {"symmetric": False, "why": why}
        except Exception as e:  # what the atom raised is the answer, reported with it
            return {"error": f"{type(e).__name__}: {e}"}
        return {"error": f"protocol {PROTOCOL} has no request {op!r}"}
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

    def guard(self, atom: dict, values: list[list]) -> list[bool]: ...
    def post(self, atom: dict, values: list[list]) -> list[list[list]]: ...
    def symmetric(self, atom: dict, params: list[int], maps: list[int], groups: list[list[str]]) -> str | None: ...
    def close(self) -> None: ...


def answers(client: ClientLike, op: str) -> bool:
    """Whether the glue behind client answers the optional request op, as
    its hello said."""
    return op in client.supports


def NO_CLOSE() -> None:
    """The close of a glue that holds nothing to release, such as one
    loaded in-process."""


class Client:
    """The checker's side of the protocol over a Handle: atoms and the values
    of their arguments by index, an error answer raised as EvaluatorError.
    It says hello first and refuses a glue that answers with another
    protocol, closing the glue if hello fails."""

    def __init__(self, handle: Handle, close: Callable[[], None]):
        self._handle = handle
        self._close = close
        try:
            answer = handle({"op": "hello", "protocol": PROTOCOL})
            if not isinstance(answer, dict) or answer.get("protocol") != PROTOCOL:
                raise EvaluatorError(f"the glue answered hello with {answer!r}; csdfbmc reads atoms through protocol {PROTOCOL}")
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
            raise EvaluatorError(f"the {self.language} glue answered {request.get('op')} of the atom {shown_atom(request['atom'])}: {answer['error']}")
        return answer

    def _malformed(self, request: dict, answer, why: str) -> EvaluatorError:
        return EvaluatorError(f"the {self.language} glue answered {request.get('op')} of the atom {shown_atom(request['atom'])} with {answer!r}: {why}")

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
        request = {"op": "symmetric", "atom": atom, "params": params, "maps": maps, "groups": groups}
        if "symmetric" not in self.supports:
            raise EvaluatorError(f"the {self.language} glue does not answer symmetric, which was asked of the atom {shown_atom(atom)}")
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
    """A Client whose post answers are checked against the atom: each gives
    exactly one value per primed argument."""

    def __init__(self, inner: ClientLike):
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


def _typed_key(values: list) -> str:
    """values as a memo key that tells them apart as the protocol does: in
    Python, True == 1 and they hash alike, but a glue may answer differently
    for them."""
    return json.dumps([to_wire(v) for v in values], sort_keys=True, ensure_ascii=False)


class Memoising:
    """A Client that asks about an atom once per list of values, which a
    pure atom allows, and asks about the lists of one batch it was not asked
    about yet in one request."""

    def __init__(self, inner: ClientLike):
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


def checked(client: ClientLike) -> ClientLike:
    """client as a walk asks it: each answer checked, and each atom asked
    once per list of values."""
    return Memoising(Validating(client))
