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


if __name__ == "__main__":
    unittest.main()
