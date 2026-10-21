"""The checker's side of protocol 2: atoms, whose values come by the index
of their argument."""
import unittest

from csdfbmc.protocol import NO_CLOSE, AtomClient, EvaluatorError, checked_atoms
from csdfbmc.python_glue import evaluator

GREATER = {"text": ["", " > ", ""], "holes": [0, 1], "primed": [False, False]}
NEXT = {"text": ["", " == ", " + 1"], "holes": [0, 1], "primed": [True, False]}


def fake(answers: dict):
    return lambda request: answers[request["op"]]


class AtomClientTest(unittest.TestCase):
    def test_it_asks_the_glue_about_atoms(self):
        # Setup
        client = AtomClient(evaluator([]), NO_CLOSE)

        # Execute
        guards = client.guard(GREATER, [[3, 2], [1, 2]])
        posts = client.post(NEXT, [[2]])

        # Assert
        self.assertEqual([True, False], guards)
        self.assertEqual([[[3]]], posts)

    def test_a_glue_speaking_protocol_1_only_is_refused(self):
        # Setup
        handle = fake({"hello": {"language": "old", "protocol": 1}})

        # Execute, Assert
        with self.assertRaisesRegex(EvaluatorError, "protocol 2"):
            AtomClient(handle, NO_CLOSE)


class CountingGlue:
    """The Python glue, counting the lists of values it is asked about."""

    def __init__(self):
        self.inner = evaluator([])
        self.asked = 0

    def __call__(self, request):
        self.asked += len(request.get("values", []))
        return self.inner(request)


class CheckedAtomsTest(unittest.TestCase):
    def test_an_atom_is_asked_once_per_list_of_values(self):
        # Setup
        glue = CountingGlue()
        client = checked_atoms(AtomClient(glue, NO_CLOSE))

        # Execute
        client.guard(GREATER, [[3, 2], [3, 2]])
        client.guard(GREATER, [[3, 2], [1, 2]])

        # Assert
        self.assertEqual(2, glue.asked)

    def test_a_post_giving_another_number_of_values_is_refused(self):
        # Setup
        handle = fake({"hello": {"language": "fake", "protocol": 2}, "post": {"results": [[[1, 2]]]}})
        client = checked_atoms(AtomClient(handle, NO_CLOSE))

        # Execute, Assert
        with self.assertRaisesRegex(EvaluatorError, "1 value after the step"):
            client.post(NEXT, [[2]])


if __name__ == "__main__":
    unittest.main()
