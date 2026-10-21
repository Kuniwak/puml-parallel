"""What a REPL asks the checker at a value prompt: the values the clauses
admit for the initial state, or for one outgoing edge of a given state."""
import io
import json
import unittest
from contextlib import redirect_stdout
from csdfbmc.tests import fixture

from csdfbmc.next import main as next_main
from csdfbmc.tests.test_explore import COUNTER
from csdfbmc.twins import Twins


class CandidatesTest(unittest.TestCase):
    def test_initial_candidates_are_the_start_posts_successors(self):
        self.assertEqual([[2]], fixture.explorer(COUNTER).initial_candidates())

    def test_candidates_of_an_edge_come_per_parameter_valuation(self):
        diagram = dict(COUNTER, edges=[
            {"src": "a", "dst": "a", "event": "add?k", "channel": "add", "fields": [{"dir": "?", "name": "k"}], "guard": "n + k <= 3", "post": "[{'n': n + k}]", "line": 6},
            {"src": "a", "dst": "b", "event": "stop", "channel": "stop", "fields": [], "guard": "n == 0", "post": "", "line": 7},
        ])
        ex = fixture.explorer(diagram, Twins(domains={"k": [1, 2]}))
        self.assertEqual([{"params": {"k": 1}, "values": [3]}], ex.candidates("a", [2], 0))
        self.assertEqual([], ex.candidates("a", [2], 1))  # stop needs n = 0

    def test_a_bad_edge_index_is_refused(self):
        with self.assertRaises(IndexError):
            fixture.explorer(COUNTER).candidates("a", [2], 5)


class NextCommandTest(unittest.TestCase):
    def run_next(self, tmp, *args):
        import os, tempfile
        d = tempfile.mkdtemp()
        diagram = os.path.join(d, "d.json")
        twins = os.path.join(d, "t.py")
        with open(diagram, "w", encoding="utf-8") as f:
            json.dump(fixture.parsed(COUNTER), f)
        with open(twins, "w", encoding="utf-8") as f:
            f.write("DOMAINS = {}\n")
        out = io.StringIO()
        with redirect_stdout(out):
            code = next_main([diagram, twins, "--evaluator-module", "csdfbmc.python_glue", *args])
        return code, out.getvalue()

    def test_initial_prints_the_initial_values(self):
        code, out = self.run_next(None, "--initial")
        self.assertEqual(0, code)
        self.assertEqual({"candidates": [{"params": {}, "values": [2]}]}, json.loads(out))

    def test_edge_prints_the_values_after_the_edge_from_a_runtime_state(self):
        state = json.dumps({"state_id": "a", "state_name": "counting", "values": [{"name": "n", "value": 1}]})
        code, out = self.run_next(None, "--state", state, "--edge", "0")
        self.assertEqual(0, code)
        self.assertEqual({"candidates": [{"params": {}, "values": [0]}]}, json.loads(out))
        code, out = self.run_next(None, "--state", state, "--edge", "1")
        self.assertEqual({"candidates": []}, json.loads(out))


if __name__ == "__main__":
    unittest.main()
