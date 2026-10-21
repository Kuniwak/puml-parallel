"""The evaluator protocol carries values in a tagged JSON encoding, since
JSON has no tuples, sets or maps with keys other than strings."""
import gc
import io
import json
import os
import sys
import unittest
import warnings

from csdfbmc.protocol import NO_CLOSE, Analysis, Client, answers, EvaluatorError, Memoising, Validating, from_wire, serve, start_command, start_module, to_wire
from csdfbmc.values import FrozenMap


class WireEncodingTest(unittest.TestCase):
    def test_scalars_are_themselves(self):
        for v in (0, -3, "残高", True, False, None):
            with self.subTest(v=v):
                self.assertEqual(v, to_wire(v))
                self.assertEqual(v, from_wire(to_wire(v)))

    def test_containers_are_tagged(self):
        for value, wire in [
            ((1, 2), {"tuple": [1, 2]}),
            (frozenset({2, 1}), {"set": [1, 2]}),
            (FrozenMap({"k": 1}), {"map": [["k", 1]]}),
        ]:
            with self.subTest(value=value):
                self.assertEqual(wire, to_wire(value))

    def test_a_nested_value_comes_back_equal_through_json(self):
        v = FrozenMap({"t1": FrozenMap({"state": "署名済み", "手数料": 2}), 3: (1, frozenset({"a"}))})
        self.assertEqual(v, from_wire(json.loads(json.dumps(to_wire(v)))))

    def test_a_boolean_stays_a_boolean(self):
        self.assertIs(True, from_wire(to_wire(True)))
        self.assertIsNot(True, from_wire(to_wire(1)))

    def test_an_untagged_array_or_object_is_refused(self):
        for wire in ([1, 2], {"a": 1}, {"set": [1], "map": []}, 1.5):
            with self.subTest(wire=wire), self.assertRaises(ValueError):
                from_wire(wire)

    def test_a_set_is_encoded_in_one_order_whatever_its_items(self):
        a = frozenset({1, "a", (2, 3)})
        b = frozenset({(2, 3), "a", 1})
        self.assertEqual(json.dumps(to_wire(a)), json.dumps(to_wire(b)))

    def test_a_value_outside_the_model_is_refused(self):
        with self.assertRaises(TypeError):
            to_wire(1.5)


def fake(answers: dict):
    """A Handle answering each op with what answers holds for it."""
    return lambda request: answers[request["op"]]


HELLO = {"language": "fake", "protocol": 1}


class ClientTest(unittest.TestCase):
    """The client refuses an answer the protocol does not define, rather
    than reading something into it."""

    def test_a_hello_naming_no_language_is_refused(self):
        with self.assertRaises(EvaluatorError):
            Client(fake({"hello": {"protocol": 1}}), NO_CLOSE)

    def test_a_malformed_answer_is_refused(self):
        for name, answers, ask in [
            ("analyse without reads", {"analyse": {"writes": []}}, lambda c: c.analyse("x", "guard")),
            ("reads that are not names", {"analyse": {"reads": [1], "writes": []}}, lambda c: c.analyse("x", "guard")),
            ("a guard that is not a truth value", {"guard": {"results": [1]}}, lambda c: c.guard("x", [{}])),
            ("fewer results than bindings", {"guard": {"results": []}}, lambda c: c.guard("x", [{}])),
            ("a post that is not a list of assignments", {"post": {"results": [[1]]}}, lambda c: c.post("x", [{}])),
            ("an answer that is not an object", {"guard": [True]}, lambda c: c.guard("x", [{}])),
            ("an error that is not a message", {"guard": {"error": ["x"]}}, lambda c: c.guard("x", [{}])),
            ("beyond without reads", {"beyond": {}}, lambda c: c.beyond("x", "guard", "i")),
            ("symmetric that is not a truth value", {"symmetric": {"symmetric": 1}}, lambda c: c.symmetric("x", "guard", [], [], [])),
            ("not symmetric without why", {"symmetric": {"symmetric": False}}, lambda c: c.symmetric("x", "guard", [], [], [])),
        ]:
            client = Client(fake({"hello": dict(HELLO, supports=["beyond", "symmetric"]), **answers}), NO_CLOSE)
            with self.subTest(name), self.assertRaises(EvaluatorError):
                ask(client)


class SupportsTest(unittest.TestCase):
    """A glue answers beyond and symmetric only when its hello says it
    does; the client asks nothing of one that does not."""

    def test_the_optional_requests_are_asked_of_a_glue_that_answers_them(self):
        for name, ask, want in [
            ("beyond", lambda c: c.beyond("x", "guard", "i"), ("n",)),
            ("symmetric", lambda c: c.symmetric("x", "guard", ["i"], [], [["a", "b"]]), "i is ordered"),
        ]:
            with self.subTest(name):
                # Setup
                client = Client(fake({"hello": dict(HELLO, supports=["beyond", "symmetric"]),
                                      "beyond": {"reads": ["n"]}, "symmetric": {"symmetric": False, "why": "i is ordered"}}), NO_CLOSE)

                # Execute
                got = ask(client)

                # Assert
                self.assertEqual(want, got)

    def test_a_glue_that_does_not_answer_them_is_not_asked(self):
        # Setup
        client = Client(fake({"hello": HELLO}), NO_CLOSE)

        # Execute, Assert
        self.assertFalse(answers(client, "beyond"))
        with self.assertRaises(EvaluatorError):
            client.beyond("x", "guard", "i")

    def test_a_hello_with_supports_that_are_not_names_is_refused(self):
        # Execute, Assert
        with self.assertRaises(EvaluatorError):
            Client(fake({"hello": dict(HELLO, supports="beyond")}), NO_CLOSE)

    def test_validating_refuses_a_name_beyond_that_the_clause_does_not_read(self):
        # Setup
        inner = Client(fake({"hello": dict(HELLO, supports=["beyond"]), "analyse": {"reads": ["n"], "writes": []},
                             "beyond": {"reads": ["n", "m"]}}), NO_CLOSE)

        # Execute, Assert
        with self.assertRaisesRegex(EvaluatorError, "'m'"):
            Validating(inner).beyond("x", "guard", "i")


class Recording:
    """A client answering from tables, which records the bindings it is asked about."""
    language = "fake"
    supports = frozenset()

    def __init__(self, analyses: dict, answer):
        self.analyses, self.answer, self.asked = analyses, answer, []

    def analyse(self, clause, role):
        return self.analyses[clause]

    def guard(self, clause, bindings):
        self.asked.append(bindings)
        return [self.answer(b) for b in bindings]

    post = guard

    def close(self):
        pass


class DecoratorTest(unittest.TestCase):
    def test_memoising_asks_once_per_value_of_what_a_clause_reads(self):
        inner = Recording({"n > 0": Analysis(opaque=False, reads=("n",))}, lambda b: b["n"] > 0)
        client = Memoising(inner)
        self.assertEqual([True, False, True], client.guard("n > 0", [{"n": 1}, {"n": 0}, {"n": 1}]))
        self.assertEqual([True], client.guard("n > 0", [{"n": 1}]))
        self.assertEqual([[{"n": 1}, {"n": 0}]], inner.asked)

    def test_validating_refuses_an_assignment_beside_the_writes(self):
        for name, answer in [("a variable too many", [{"n": 1, "m": 0}]), ("a variable short", [{}])]:
            client = Validating(Recording({"p": Analysis(opaque=False, writes=("n",))}, lambda b: answer))
            with self.subTest(name), self.assertRaises(EvaluatorError):
                client.post("p", [{}])

    def test_validating_passes_an_assignment_to_exactly_the_writes(self):
        client = Validating(Recording({"p": Analysis(opaque=False, writes=("n",))}, lambda b: [{"n": 1}]))
        self.assertEqual([[{"n": 1}]], client.post("p", [{}]))


class ServeTest(unittest.TestCase):
    """A glue's main loop answers every line, even one it cannot read."""

    def serve(self, lines: list[str], handle) -> list[dict]:
        out = io.StringIO()
        serve(handle, io.StringIO("".join(line + "\n" for line in lines)), out)
        return [json.loads(line) for line in out.getvalue().splitlines()]

    def test_a_line_that_is_not_a_request_is_answered_with_an_error(self):
        answers = self.serve(["not json", json.dumps({"op": "hello"})], lambda r: HELLO)
        self.assertIn("error", answers[0])
        self.assertEqual(HELLO, answers[1])

    def test_what_the_handle_raises_is_answered_with_an_error(self):
        def handle(request):
            raise KeyError("boom")
        self.assertIn("boom", self.serve([json.dumps({"op": "hello"})], handle)[0]["error"])

    def test_the_values_of_an_atom_post_are_encoded_both_ways(self):
        # Setup
        def handle(request):
            return {"results": [[[frozenset(vs)]] for vs in request["values"]]}
        request = {"op": "post", "atom": {"text": ["", " == ", ""], "holes": [0, 1], "primed": [True, False]},
                   "values": [[{"set": [1]}]]}

        # Execute
        answers = self.serve([json.dumps(request)], handle)

        # Assert
        self.assertEqual([{"results": [[[{"set": [{"set": [1]}]}]]]}], answers)


# The repository root, from which "python -m csdfbmc.python_glue" finds the package.
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


class TransportTest(unittest.TestCase):
    """The checker reaches a glue as a command speaking JSON lines, or as a
    Python module loaded in-process; both answer the same requests."""

    def setUp(self):
        command = start_command([sys.executable, "-E", "-s", "-B", "-m", "csdfbmc.python_glue"], cwd=ROOT)
        self.addCleanup(command.close)
        self.evaluators = {"command": command, "module": start_module("csdfbmc.python_glue", [])}

    def test_both_answer_alike(self):
        for ask, want in [
            (lambda ev: ev.language, "python"),
            (lambda ev: ev.analyse("[{'m': m.set('k', (1, 2))}]", "post"), Analysis(opaque=False, reads=("m",), writes=("m",))),
            (lambda ev: ev.post("[{'m': m.set('k', (1, 2))}]", [{"m": FrozenMap()}]), [[{"m": FrozenMap({"k": (1, 2)})}]]),
            (lambda ev: ev.guard("len(s) == 2", [{"s": frozenset({1, 2})}]), [True]),
        ]:
            for how, ev in self.evaluators.items():
                with self.subTest(how=how, want=want):
                    self.assertEqual(want, ask(ev))

    def test_an_error_answer_is_raised_with_its_message(self):
        for how, ev in self.evaluators.items():
            with self.subTest(how=how), self.assertRaises(EvaluatorError) as cm:
                ev.guard("x >", [{}])
            self.assertIn("x >", str(cm.exception))

    def test_a_glue_that_stops_before_hello_leaves_no_pipe_open(self):
        with warnings.catch_warnings(record=True) as caught:
            warnings.simplefilter("always", ResourceWarning)
            with self.assertRaises(EvaluatorError):
                start_command([sys.executable, "-c", "pass"])
            gc.collect()
        self.assertEqual([], [w for w in caught if issubclass(w.category, ResourceWarning)])

    def test_a_glue_speaking_another_protocol_version_is_refused(self):
        with self.assertRaises(EvaluatorError) as cm:
            start_command([sys.executable, "-c", "import sys; sys.stdin.readline(); print('{\"language\": \"x\", \"protocol\": 99}')"])
        self.assertIn("99", str(cm.exception))


if __name__ == "__main__":
    unittest.main()
