"""The checker's side of protocol 2: atoms, whose values come by the index
of their argument."""
import unittest

from csdfbmc.protocol import NO_CLOSE, AtomClient, EvaluatorError
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


if __name__ == "__main__":
    unittest.main()
