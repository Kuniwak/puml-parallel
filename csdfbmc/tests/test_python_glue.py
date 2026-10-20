"""The reference glue reads clauses written in the pure subset of Python and
answers the requests of the evaluator protocol about them."""
import unittest

from csdfbmc.python_glue import evaluator


class PythonGlueTest(unittest.TestCase):
    def setUp(self):
        self.handle = evaluator([])

    def test_hello_names_the_language_and_the_protocol_version(self):
        self.assertEqual({"language": "python", "protocol": 1}, self.handle({"op": "hello"}))

    def test_a_guard_reads_its_free_names(self):
        answer = self.handle({"op": "analyse", "clause": "n + k <= 3 and all(v > 0 for v in xs)", "role": "guard"})
        self.assertEqual({"reads": ["k", "n", "xs"], "writes": []}, answer)

    def test_a_post_writes_the_keys_of_its_assignments(self):
        for clause, reads, writes in [
            ("[{'x': x + 1, 'y': y}]", ["x", "y"], ["x", "y"]),
            ("[{'y': v} for v in (y, y + 1)]", ["y"], ["y"]),
            ("{(1, 2): 0} and [{'z': 0}]", None, None),
        ]:
            with self.subTest(clause=clause):
                answer = self.handle({"op": "analyse", "clause": clause, "role": "post"})
                if writes is None:
                    self.assertIn("error", answer)
                else:
                    self.assertEqual({"reads": reads, "writes": writes}, answer)

    def test_the_assignments_of_a_display_must_agree_on_their_keys(self):
        answer = self.handle({"op": "analyse", "clause": "[{'x': 1}, {'y': 2}]", "role": "post"})
        self.assertIn("same variables", answer["error"])

    def test_opaque_marks_a_clause_not_yet_formalised(self):
        for role in ("guard", "post"):
            with self.subTest(role=role):
                self.assertEqual({"opaque": True}, self.handle({"op": "analyse", "clause": "OPAQUE", "role": role}))

    def test_a_clause_outside_the_pure_subset_is_refused(self):
        answer = self.handle({"op": "analyse", "clause": "open('f') and 1.5", "role": "guard"})
        self.assertIn("not allowed", answer["error"])

    def test_a_guard_is_evaluated_under_each_binding(self):
        answer = self.handle({"op": "guard", "clause": "n + k <= 3", "bindings": [{"n": 1, "k": 2}, {"n": 2, "k": 2}]})
        self.assertEqual({"results": [True, False]}, answer)

    def test_a_post_is_evaluated_to_frozen_assignments(self):
        answer = self.handle({"op": "post", "clause": "[{'xs': {v}} for v in (y, y + 1)]", "bindings": [{"y": 1}]})
        self.assertEqual({"results": [[{"xs": frozenset({1})}, {"xs": frozenset({2})}]]}, answer)
        hash(answer["results"][0][0]["xs"])

    def test_an_error_in_evaluation_is_answered_with_its_message(self):
        answer = self.handle({"op": "guard", "clause": "m['k'] > 0", "bindings": [{"m": {}}]})
        self.assertIn("'k'", answer["error"])

    def test_definitions_shared_by_the_clauses_come_from_the_named_modules(self):
        import os
        import tempfile
        with tempfile.TemporaryDirectory() as d:
            path = os.path.join(d, "defs.py")
            with open(path, "w", encoding="utf-8") as f:
                f.write("FEE = 1\ndef fee(n):\n    return n + FEE\n")
            handle = evaluator([path])
            self.assertEqual({"reads": ["n"], "writes": []}, handle({"op": "analyse", "clause": "fee(n) > FEE", "role": "guard"}))
            self.assertEqual({"results": [True]}, handle({"op": "guard", "clause": "fee(n) > FEE", "bindings": [{"n": 1}]}))


if __name__ == "__main__":
    unittest.main()
