"""The evaluator protocol: atoms and the values of their arguments by index,
in a tagged JSON encoding, since JSON has no tuples, sets or maps with keys
other than strings."""
import gc
import io
import json
import os
import sys
import unittest
import warnings

from csdfbmc.protocol import (NO_CLOSE, NO_SYMMETRIC, Client, EvaluatorError, atom_handler, checked, from_wire, serve,
                              start_command, start_module, to_wire)
from csdfbmc.python_glue import evaluator
from csdfbmc.values import FrozenMap, same_value

GREATER = {"text": ["", " > ", ""], "holes": [0, 1], "primed": [False, False]}
NEXT = {"text": ["", " == ", " + 1"], "holes": [0, 1], "primed": [True, False]}
HELLO = {"language": "fake", "protocol": 2}


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
        for wire in ([1, 2], {"a": 1}, {"set": [1], "map": []}):
            with self.subTest(wire=wire), self.assertRaises(ValueError):
                from_wire(wire)

    def test_a_set_is_encoded_in_one_order_whatever_its_items(self):
        a = frozenset({1, "a", (2, 3)})
        b = frozenset({(2, 3), "a", 1})
        self.assertEqual(json.dumps(to_wire(a)), json.dumps(to_wire(b)))

    def test_a_float_is_itself_and_stays_apart_from_the_integer_it_equals(self):
        for v in (1.5, 1.0, 0.0, -2.5, 1e300):
            with self.subTest(v=v):
                got = from_wire(json.loads(json.dumps(to_wire(v))))
                self.assertIs(float, type(got))
                self.assertTrue(same_value(v, got))
        self.assertIs(int, type(from_wire(json.loads(json.dumps(to_wire(1))))))

    def test_a_value_outside_the_model_is_refused(self):
        for v in (float("nan"), float("inf"), -float("inf"), -0.0, object()):
            with self.subTest(v=v), self.assertRaises(TypeError):
                to_wire(v)
        for wire in (float("nan"), float("inf"), -0.0):
            with self.subTest(wire=wire), self.assertRaises(ValueError):
                from_wire(wire)


def fake(answers: dict):
    """A Handle answering each op with what answers holds for it."""
    return lambda request: answers[request["op"]]


class ClientTest(unittest.TestCase):
    def test_it_asks_the_glue_about_atoms(self):
        tests = {
            "a guard": (lambda c: c.guard(GREATER, [[3, 2], [1, 2]]), [True, False]),
            "a post": (lambda c: c.post(NEXT, [[2]]), [[[3]]]),
        }
        for name, (ask, want) in tests.items():
            with self.subTest(name):
                # Setup
                client = Client(evaluator([]), NO_CLOSE)

                # Execute
                got = ask(client)

                # Assert
                self.assertEqual(want, got)

    def test_a_wrong_answer_is_refused(self):
        tests = {
            "a glue speaking protocol 1": ({"hello": {"language": "old", "protocol": 1}}, lambda c: None, "protocol 2"),
            "a hello naming no language": ({"hello": {"protocol": 2}}, lambda c: None, "names no language"),
            "a hello whose supports are not names": ({"hello": dict(HELLO, supports=[1])}, lambda c: None, "not a list of requests"),
            "fewer results than lists of values":
                ({"hello": HELLO, "guard": {"results": [True]}}, lambda c: c.guard(GREATER, [[1, 2], [3, 4]]), "one result per list of values"),
            "a guard that is not a truth value":
                ({"hello": HELLO, "guard": {"results": [1]}}, lambda c: c.guard(GREATER, [[1, 2]]), "a guard is a truth value"),
            "a post that is not lists of values":
                ({"hello": HELLO, "post": {"results": [[3]]}}, lambda c: c.post(NEXT, [[2]]), "a post is a list of lists"),
            "an error answer": ({"hello": HELLO, "guard": {"error": "boom"}}, lambda c: c.guard(GREATER, [[1, 2]]), "boom"),
            "symmetric of a glue that does not answer it":
                ({"hello": HELLO}, lambda c: c.symmetric(GREATER, [0], [], [["a1", "a2"]]), "does not answer symmetric"),
            "a post giving another number of values":
                ({"hello": HELLO, "post": {"results": [[[1, 2]]]}}, lambda c: checked(c).post(NEXT, [[2]]), "1 value after the step"),
        }
        for name, (answers, ask, error) in tests.items():
            with self.subTest(name):
                # Execute, Assert
                with self.assertRaisesRegex(EvaluatorError, error):
                    ask(Client(fake(answers), NO_CLOSE))


class CountingGlue:
    """The Python glue, counting the lists of values it is asked about."""

    def __init__(self):
        self.inner = evaluator([])
        self.asked = 0

    def __call__(self, request):
        self.asked += len(request.get("values", []))
        return self.inner(request)


class CheckedTest(unittest.TestCase):
    def test_an_atom_is_asked_once_per_list_of_values(self):
        # Setup
        glue = CountingGlue()
        client = checked(Client(glue, NO_CLOSE))

        # Execute
        client.guard(GREATER, [[3, 2], [3, 2]])
        client.guard(GREATER, [[3, 2], [1, 2]])

        # Assert
        self.assertEqual(2, glue.asked)

    def test_a_truth_value_is_not_the_number_it_equals(self):
        # Setup
        client = checked(Client(CountingGlue(), NO_CLOSE))
        same = {"text": ["", " is True"], "holes": [0], "primed": [False]}

        # Execute
        got = client.guard(same, [[1]]) + client.guard(same, [[True]])

        # Assert
        self.assertEqual([False, True], got)


class HandlerTest(unittest.TestCase):
    """A glue speaks protocol 2 after its hello agreed on it, and only that."""

    def handle(self):
        return atom_handler("fake", guard=lambda a, vs: True, post=lambda a, vs: [], symmetric=NO_SYMMETRIC)

    def test_a_request_it_does_not_speak_is_refused(self):
        tests = {
            "a request before hello": (False, {"op": "guard", "atom": GREATER, "values": [[1, 2]]}, "hello"),
            "a clause of protocol 1": (True, {"op": "guard", "clause": "n > 0", "bindings": [{"n": 1}]}, "carries an atom"),
            "analyse of protocol 1": (True, {"op": "analyse", "clause": "n > 0", "role": "guard"}, "carries an atom"),
            "symmetric of a glue that does not answer it": (True, {"op": "symmetric", "atom": GREATER, "params": [0], "maps": [], "groups": [["a", "b"]]}, "no request"),
        }
        for name, (greet, request, said) in tests.items():
            with self.subTest(name):
                # Setup
                handle = self.handle()
                if greet:
                    handle({"op": "hello", "protocol": 2})

                # Execute
                answer = handle(request)

                # Assert
                self.assertIn(said, answer.get("error", ""))

    def test_a_hello_asking_another_protocol_is_refused(self):
        for name, hello in {"protocol 1, named by none": {"op": "hello"}, "protocol 3": {"op": "hello", "protocol": 3}}.items():
            with self.subTest(name):
                # Execute
                answer = self.handle()(hello)

                # Assert
                self.assertIn("speaks protocol 2", answer.get("error", ""))

    def test_a_glue_without_symmetric_supports_nothing(self):
        # Execute
        hello = self.handle()({"op": "hello", "protocol": 2})

        # Assert
        self.assertEqual(HELLO, hello)


class ServeTest(unittest.TestCase):
    """A glue's main loop answers every line, even one it cannot read."""

    def serve(self, lines: list[str], handle) -> list[dict]:
        out = io.StringIO()
        serve(handle, io.StringIO("".join(line + "\n" for line in lines)), out)
        return [json.loads(line) for line in out.getvalue().splitlines()]

    def test_a_line_that_is_not_a_request_is_answered_with_an_error(self):
        answers = self.serve(["not json", json.dumps({"op": "hello", "protocol": 2})], lambda r: HELLO)
        self.assertIn("error", answers[0])
        self.assertEqual(HELLO, answers[1])

    def test_what_the_handle_raises_is_answered_with_an_error(self):
        def handle(request):
            raise KeyError("boom")
        self.assertIn("boom", self.serve([json.dumps({"op": "hello", "protocol": 2})], handle)[0]["error"])

    def test_the_values_of_an_atom_post_are_encoded_both_ways(self):
        # Setup
        def handle(request):
            if request["op"] == "hello":
                return {"language": "fake", "protocol": 2}
            return {"results": [[[frozenset(vs)]] for vs in request["values"]]}
        request = {"op": "post", "atom": {"text": ["", " == ", ""], "holes": [0, 1], "primed": [True, False]},
                   "values": [[{"set": [1]}]]}

        # Execute
        answers = self.serve([json.dumps({"op": "hello", "protocol": 2}), json.dumps(request)], handle)

        # Assert
        self.assertEqual({"results": [[[{"set": [{"set": [1]}]}]]]}, answers[1])


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
        set_m = {"text": ["", " == ", ".set('k', (1, 2))"], "holes": [0, 1], "primed": [True, False]}
        two = {"text": ["len(", ") == 2"], "holes": [0], "primed": [False]}
        for ask, want in [
            (lambda ev: ev.language, "python"),
            (lambda ev: ev.post(set_m, [[FrozenMap()]]), [[[FrozenMap({"k": (1, 2)})]]]),
            (lambda ev: ev.guard(two, [[frozenset({1, 2})]]), [True]),
        ]:
            for how, ev in self.evaluators.items():
                with self.subTest(how=how, want=want):
                    self.assertEqual(want, ask(ev))

    def test_an_error_answer_is_raised_with_its_message(self):
        broken = {"text": ["", " >"], "holes": [0], "primed": [False]}
        for how, ev in self.evaluators.items():
            with self.subTest(how=how), self.assertRaises(EvaluatorError) as cm:
                ev.guard(broken, [[1]])
            self.assertIn("$0 >", str(cm.exception))

    def test_a_glue_that_stops_before_hello_leaves_no_pipe_open(self):
        with warnings.catch_warnings(record=True) as caught:
            warnings.simplefilter("always", ResourceWarning)
            with self.assertRaises(EvaluatorError):
                start_command([sys.executable, "-c", "pass"])
            gc.collect()
        self.assertEqual([], [w for w in caught if issubclass(w.category, ResourceWarning)])

    def test_a_glue_speaking_another_protocol_version_is_refused(self):
        with self.assertRaises(EvaluatorError) as cm:
            start_command([sys.executable, "-c", 'import sys; sys.stdin.readline(); print(\'{"language": "x", "protocol": 99}\')'])
        self.assertIn("99", str(cm.exception))


if __name__ == "__main__":
    unittest.main()
