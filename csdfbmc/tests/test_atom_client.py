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
        tests = {
            "a guard": (lambda c: c.guard(GREATER, [[3, 2], [1, 2]]), [True, False]),
            "a post": (lambda c: c.post(NEXT, [[2]]), [[[3]]]),
        }
        for name, (ask, want) in tests.items():
            with self.subTest(name):
                # Setup
                client = AtomClient(evaluator([]), NO_CLOSE)

                # Execute
                got = ask(client)

                # Assert
                self.assertEqual(want, got)

    def test_a_wrong_answer_is_refused(self):
        hello = {"language": "fake", "protocol": 2}
        tests = {
            "a glue speaking protocol 1 only":
                ({"hello": {"language": "old", "protocol": 1}}, AtomClient, lambda c: None, "protocol 2"),
            "fewer results than lists of values":
                ({"hello": hello, "guard": {"results": [True]}}, AtomClient, lambda c: c.guard(GREATER, [[1, 2], [3, 4]]), "one result per list of values"),
            "a guard that is not a truth value":
                ({"hello": hello, "guard": {"results": [1]}}, AtomClient, lambda c: c.guard(GREATER, [[1, 2]]), "a guard is a truth value"),
            "a post that is not lists of values":
                ({"hello": hello, "post": {"results": [[3]]}}, AtomClient, lambda c: c.post(NEXT, [[2]]), "a post is a list of lists"),
            "symmetric of a glue that does not answer it":
                ({"hello": hello}, AtomClient, lambda c: c.symmetric(GREATER, [0], [], [["a1", "a2"]]), "does not answer symmetric"),
            "a post giving another number of values":
                ({"hello": hello, "post": {"results": [[[1, 2]]]}}, lambda h, close: checked_atoms(AtomClient(h, close)),
                 lambda c: c.post(NEXT, [[2]]), "1 value after the step"),
        }
        for name, (answers, client_of, ask, error) in tests.items():
            with self.subTest(name):
                # Setup
                handle = fake(answers)

                # Execute, Assert
                with self.assertRaisesRegex(EvaluatorError, error):
                    ask(client_of(handle, NO_CLOSE))


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

    def test_a_truth_value_is_not_the_number_it_equals(self):
        # Setup
        glue = CountingGlue()
        client = checked_atoms(AtomClient(glue, NO_CLOSE))
        same = {"text": ["", " is True"], "holes": [0], "primed": [False]}

        # Execute
        got = client.guard(same, [[1]]) + client.guard(same, [[True]])

        # Assert
        self.assertEqual([False, True], got)

if __name__ == "__main__":
    unittest.main()
