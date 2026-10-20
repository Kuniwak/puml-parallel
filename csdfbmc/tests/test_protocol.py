"""The evaluator protocol carries values in a tagged JSON encoding, since
JSON has no tuples, sets or maps with keys other than strings."""
import json
import os
import sys
import unittest

from csdfbmc.protocol import Analysis, EvaluatorError, from_wire, start_command, start_module, to_wire
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

    def test_a_value_outside_the_model_is_refused(self):
        with self.assertRaises(TypeError):
            to_wire(1.5)


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
