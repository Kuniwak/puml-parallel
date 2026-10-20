"""The evaluator protocol carries values in a tagged JSON encoding, since
JSON has no tuples, sets or maps with keys other than strings."""
import io
import json
import os
import sys
import unittest

from csdfbmc.protocol import Analysis, Client, EvaluatorError, from_wire, serve, start_command, start_module, to_wire
from csdfbmc.values import FrozenMap


class WireEncodingTest(unittest.TestCase):
    def test_scalars_are_themselves(self):
        for v in (0, -3, "残高", True, False, None):
            with self.subTest(v=v):
                self.assertEqual(v, to_wire(v))
                self.assertEqual(v, from_wire(to_wire(v)))

    def test_containers_are_tagged_and_come_back_equal(self):
        v = FrozenMap({"t1": FrozenMap({"state": "署名済み", "手数料": 2}), 3: (1, frozenset({"a"}))})
        wire = json.loads(json.dumps(to_wire(v)))
        self.assertEqual(v, from_wire(wire))
        self.assertEqual({"tuple": [1, 2]}, to_wire((1, 2)))
        self.assertEqual({"set": [1, 2]}, to_wire(frozenset({2, 1})))
        self.assertEqual({"map": [["k", 1]]}, to_wire(FrozenMap({"k": 1})))

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

    def test_a_malformed_answer_is_refused(self):
        for name, answers, ask in [
            ("hello without a language", {"hello": {"protocol": 1}}, None),
            ("analyse without reads", {"analyse": {"writes": []}}, lambda c: c.analyse("x", "guard")),
            ("reads that are not names", {"analyse": {"reads": [1], "writes": []}}, lambda c: c.analyse("x", "guard")),
            ("a guard that is not a truth value", {"guard": {"results": [1]}}, lambda c: c.guard("x", [{}])),
            ("fewer results than bindings", {"guard": {"results": []}}, lambda c: c.guard("x", [{}])),
            ("a post that is not a list of assignments", {"post": {"results": [[1]]}}, lambda c: c.post("x", [{}])),
        ]:
            with self.subTest(name), self.assertRaises(EvaluatorError):
                client = Client(fake({"hello": HELLO, **answers}))
                ask(client)


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


# The repository root, from which "python -m csdfbmc.python_glue" finds the package.
ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))


class TransportTest(unittest.TestCase):
    """The checker reaches a glue as a command speaking JSON lines, or as a
    Python module loaded in-process; both answer the same requests."""

    def evaluators(self):
        command = start_command([sys.executable, "-E", "-s", "-B", "-m", "csdfbmc.python_glue"], cwd=ROOT)
        self.addCleanup(command.close)
        return {"command": command, "module": start_module("csdfbmc.python_glue", [])}

    def test_both_answer_hello_and_the_value_requests_alike(self):
        for how, ev in self.evaluators().items():
            with self.subTest(how=how):
                self.assertEqual("python", ev.language)
                self.assertEqual(Analysis(opaque=False, reads=("m",), writes=("m",)), ev.analyse("[{'m': m.set('k', (1, 2))}]", "post"))
                self.assertEqual([[{"m": FrozenMap({"k": (1, 2)})}]], ev.post("[{'m': m.set('k', (1, 2))}]", [{"m": FrozenMap()}]))
                self.assertEqual([True], ev.guard("len(s) == 2", [{"s": frozenset({1, 2})}]))

    def test_an_error_answer_is_raised_with_its_message(self):
        for how, ev in self.evaluators().items():
            with self.subTest(how=how), self.assertRaises(EvaluatorError) as cm:
                ev.guard("x >", [{}])
            self.assertIn("x >", str(cm.exception))

    def test_a_glue_speaking_another_protocol_version_is_refused(self):
        with self.assertRaises(EvaluatorError) as cm:
            start_command([sys.executable, "-c", "import sys; sys.stdin.readline(); print('{\"language\": \"x\", \"protocol\": 99}')"])
        self.assertIn("99", str(cm.exception))


if __name__ == "__main__":
    unittest.main()
