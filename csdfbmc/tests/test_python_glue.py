"""The reference glue reads clauses written in the pure subset of Python and
answers the requests of the evaluator protocol about them."""
import os
import tempfile
import unittest

from csdfbmc.python_glue import evaluator


def analyse(clause: str, role: str) -> dict:
    return {"op": "analyse", "clause": clause, "role": role}


class PythonGlueTest(unittest.TestCase):
    def setUp(self):
        self.handle = evaluator([])
        self.handle({"op": "hello"})

    def test_hello_names_the_language_and_the_protocol_version(self):
        self.assertEqual({"language": "python", "protocol": 1, "supports": ["beyond", "symmetric"]}, self.handle({"op": "hello"}))

    def test_beyond_leaves_out_a_map_read_only_at_the_own_id(self):
        for clause, want in [
            ("accounts[i]['n'] > 0", ["i"]),
            ("accounts[i]['n'] + accounts[j]['n'] > 0", ["accounts", "i", "j"]),
            ("len(accounts) < 2", ["accounts"]),
            ("n > 0", ["n"]),
        ]:
            with self.subTest(clause):
                answer = self.handle({"op": "beyond", "clause": clause, "role": "guard", "own_id": "i"})
                self.assertEqual(want, answer["reads"])

    def test_symmetric_says_whether_a_clause_tells_two_ids_apart(self):
        ask = {"op": "symmetric", "role": "guard", "params": ["i"], "maps": ["accounts"], "groups": [["a1", "a2"]]}
        for clause, symmetric, said in [
            ("i in accounts", True, None),
            ("i in ['a1', 'a2']", True, None),
            ("i == 'a1'", False, "'a1'"),
            ("i < j", False, "ordered"),
            ("sorted(accounts)[0] == i", False, "sorted"),
        ]:
            with self.subTest(clause):
                answer = self.handle(dict(ask, clause=clause))
                self.assertEqual(symmetric, answer["symmetric"])
                if said:
                    self.assertIn(said, answer["why"])

    def test_analyse_says_what_a_clause_reads_and_writes(self):
        for request, want in [
            (analyse("n + k <= 3 and all(v > 0 for v in xs)", "guard"), {"reads": ["k", "n", "xs"], "writes": []}),
            (analyse("[{'x': x + 1, 'y': y}]", "post"), {"reads": ["x", "y"], "writes": ["x", "y"]}),
            (analyse("[{'y': v} for v in (y, y + 1)]", "post"), {"reads": ["y"], "writes": ["y"]}),
        ]:
            with self.subTest(request=request):
                self.assertEqual(want, self.handle(request))

    def test_evaluation_answers_one_result_per_binding(self):
        for request, want in [
            ({"op": "guard", "clause": "n + k <= 3", "bindings": [{"n": 1, "k": 2}, {"n": 2, "k": 2}]},
             {"results": [True, False]}),
            ({"op": "post", "clause": "[{'xs': {v}} for v in (y, y + 1)]", "bindings": [{"y": 1}]},
             {"results": [[{"xs": frozenset({1})}, {"xs": frozenset({2})}]]}),
        ]:
            with self.subTest(request=request):
                self.assertEqual(want, self.handle(request))

    def test_what_the_glue_cannot_read_or_evaluate_is_answered_with_an_error(self):
        for request, said in [
            (analyse("{(1, 2): 0} and [{'z': 0}]", "post"), "list or set of assignments"),
            (analyse("[{'x': 1}, {'y': 2}]", "post"), "same variables"),
            (analyse("open('f') and 1.5", "guard"), "not allowed"),
            ({"op": "guard", "clause": "m['k'] > 0", "bindings": [{"m": {}}]}, "'k'"),
            ({"op": "frobnicate"}, "unknown request"),
            (analyse("OPAQUE", "guard"), "write the tool word @opaque"),
            (analyse("opaque", "post"), "write the tool word @opaque"),
        ]:
            with self.subTest(request=request):
                self.assertIn(said, self.handle(request)["error"])


class DefinitionsTest(unittest.TestCase):
    """The glue's arguments are modules whose public names every clause may use."""

    def setUp(self):
        d = tempfile.TemporaryDirectory()
        self.addCleanup(d.cleanup)
        path = os.path.join(d.name, "defs.py")
        with open(path, "w", encoding="utf-8") as f:
            f.write("FEE = 1\ndef fee(n):\n    return n + FEE\n")
        self.handle = evaluator([path])
        self.handle({"op": "hello"})

    def test_a_definition_is_not_read_from_the_step(self):
        self.assertEqual({"reads": ["n"], "writes": []}, self.handle(analyse("fee(n) > FEE", "guard")))

    def test_a_clause_evaluates_with_the_definitions(self):
        self.assertEqual({"results": [True]}, self.handle({"op": "guard", "clause": "fee(n) > FEE", "bindings": [{"n": 1}]}))


if __name__ == "__main__":
    unittest.main()
