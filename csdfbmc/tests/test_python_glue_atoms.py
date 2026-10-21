"""The reference glue answers the requests of protocol 2 about atoms: text
with holes, whose values come by the index of their argument."""
import unittest

from csdfbmc.python_glue import evaluator


def atom(text: list[str], holes: list[int], primed: list[bool]) -> dict:
    return {"text": text, "holes": holes, "primed": primed}


class PythonGlueAtomTest(unittest.TestCase):
    def setUp(self):
        self.handle = evaluator([])

    def test_a_guard_is_evaluated_per_list_of_values(self):
        # Setup
        request = {"op": "guard", "atom": atom(["", " > ", ""], [0, 1], [False, False]), "values": [[3, 2], [1, 2]]}

        # Execute
        answer = self.handle(request)

        # Assert
        self.assertEqual({"results": [True, False]}, answer)

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
            "no value after the step on the left": atom(["", " == ", ""], [0, 1], [False, True]),
            "a value after the step on the right": atom(["", " == ", ""], [0, 1], [True, True]),
            "not an equation": atom(["", " > 0"], [0], [True]),
        }
        for name, a in tests.items():
            with self.subTest(name):
                # Execute
                answer = self.handle({"op": "post", "atom": a, "values": [[1]]})

                # Assert
                self.assertIn("a post atom of this glue", answer["error"])

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
                self.assertEqual(symmetric, answer["symmetric"])
                if said:
                    self.assertIn(said, answer["why"])


if __name__ == "__main__":
    unittest.main()
