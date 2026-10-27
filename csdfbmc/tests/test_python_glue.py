"""The reference glue answers the requests of the protocol about atoms: text
with holes, whose values come by the index of their argument."""
import os
import tempfile
import unittest

from csdfbmc.python_glue import evaluator


def atom(text: list[str], holes: list[int], primed: list[bool]) -> dict:
    return {"text": text, "holes": holes, "primed": primed}


class PythonGlueAtomTest(unittest.TestCase):
    def setUp(self):
        self.handle = evaluator([])
        self.handle({"op": "hello", "protocol": 2})

    def test_a_guard_is_evaluated_per_list_of_values(self):
        tests = {
            "two lists of values": (atom(["", " > ", ""], [0, 1], [False, False]), [[3, 2], [1, 2]], [True, False]),
            "no holes": (atom(["1 < 2"], [], []), [[]], [True]),
        }
        for name, (a, values, want) in tests.items():
            with self.subTest(name):
                # Setup
                request = {"op": "guard", "atom": a, "values": values}

                # Execute
                answer = self.handle(request)

                # Assert
                self.assertEqual({"results": want}, answer)

    def test_a_post_gives_the_values_after_the_step_per_list_of_values_before_it(self):
        tests = {
            "one value, by ==": (atom(["", " == ", " + 1"], [0, 1], [True, False]), [[2]], [[[3]]]),
            "a choice, by in": (atom(["", " in (", ", ", " + 1)"], [0, 1, 1], [True, False]), [[1]], [[[1], [2]]]),
            "two values at once": (atom(["(", ", ", ") == (", ", ", ")"], [0, 1, 2, 3], [True, True, False, False]), [[5, 6]], [[[5, 6]]]),
        }
        for name, (a, values, want) in tests.items():
            with self.subTest(name):
                # Setup
                request = {"op": "post", "atom": a, "values": values}

                # Execute
                answer = self.handle(request)

                # Assert
                self.assertEqual({"results": want}, answer)

    def test_a_post_of_another_shape_is_refused(self):
        tests = {
            "no value after the step on the left": (atom(["", " == ", ""], [0, 1], [False, True]), [[1]],
                                                    "argument 0 is a value before the step"),
            "a value after the step on the right": (atom(["", " == ", ""], [0, 1], [True, True]), [[]], "a post atom of this glue"),
            "not an equation": (atom(["", " > 0"], [0], [True]), [[]], "a post atom of this glue"),
        }
        for name, (a, values, said) in tests.items():
            with self.subTest(name):
                # Execute
                answer = self.handle({"op": "post", "atom": a, "values": values})

                # Assert
                self.assertIn(said, answer["error"])

    def test_a_malformed_request_is_refused(self):
        tests = {
            "text not one piece more than the holes": (
                {"op": "guard", "atom": atom(["", " > 0", ""], [0], [False]), "values": [[1]]}, "one piece of text more than"),
            "a hole reading no argument": (
                {"op": "guard", "atom": atom(["", " > 0"], [1], [False]), "values": [[1]]}, "argument 1"),
            "values of another number than the arguments": (
                {"op": "guard", "atom": atom(["", " > 0"], [0], [False]), "values": [[1, 2]]}, "2 values"),
            "an atom that is not Python": (
                {"op": "guard", "atom": atom(["", " が停止されていない"], [0], [False]), "values": [[1]]}, "SyntaxError"),
            "an atom outside the pure subset": (
                {"op": "guard", "atom": atom(["open(", ")"], [0], [False]), "values": [["f"]]}, "calling open"),
            "text naming what stands for a hole": (
                {"op": "guard", "atom": atom(["_csdf_hole_0 + ", " > 0"], [0], [False]), "values": [[1]]}, "_csdf_hole_"),
            "an unprimed hole on the left of a post": (
                {"op": "post", "atom": atom(["(", ", ", ") == (1, 2)"], [0, 1], [True, False]), "values": [[1]]},
                "argument 1 is a value before the step"),
            "a choice of another length than the values after the step": (
                {"op": "post", "atom": atom(["(", ", ", ") in [(1, 2, 3)]"], [0, 1], [True, True]), "values": [[]]},
                "3 values"),
        }
        for name, (request, said) in tests.items():
            with self.subTest(name):
                # Execute
                answer = self.handle(request)

                # Assert
                self.assertIn(said, answer.get("error", ""))

    def test_symmetric_says_whether_an_atom_tells_two_ids_apart(self):
        tests = {
            "an ID looked up": (atom(["", " in ", ""], [0, 1], [False, False]), [1], True, None),
            "an ID compared with a constant": (atom(["", " == 'a1'"], [0], [False]), [], False, "'a1'"),
            "an ID ordered": (atom(["", " < ", "['x']"], [0, 1], [False, False]), [1], False, "argument 0 is ordered"),
        }
        for name, (a, maps, symmetric, said) in tests.items():
            with self.subTest(name):
                # Setup
                request = {"op": "symmetric", "atom": a, "params": [0], "maps": maps, "groups": [["a1", "a2"]]}

                # Execute
                answer = self.handle(request)

                # Assert
                why = answer.get("why")
                self.assertEqual(symmetric, answer["symmetric"])
                self.assertEqual(said is None, why is None)
                self.assertIn(said or "", why or "")


class DefinitionsTest(unittest.TestCase):
    """The glue's arguments are modules whose public names every atom may use."""

    def test_an_atom_evaluates_with_the_definitions(self):
        # Setup
        d = tempfile.TemporaryDirectory()
        self.addCleanup(d.cleanup)
        path = os.path.join(d.name, "defs.py")
        with open(path, "w", encoding="utf-8") as f:
            f.write("STEP = 1\ndef step(n):\n    return n + STEP\n")
        handle = evaluator([path])
        handle({"op": "hello", "protocol": 2})

        # Execute
        answer = handle({"op": "guard", "atom": atom(["step(", ") > STEP"], [0], [False]), "values": [[1]]})

        # Assert
        self.assertEqual({"results": [True]}, answer)


if __name__ == "__main__":
    unittest.main()
