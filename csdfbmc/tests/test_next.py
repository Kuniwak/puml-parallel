"""What a REPL asks the checker at a value prompt: the values the twins admit
for the initial state, or for one outgoing edge of a given state."""
import io
import json
import unittest
from contextlib import redirect_stdout

from csdfbmc.model import Diagram
from csdfbmc.explore import Explorer
from csdfbmc.next import main as next_main
from csdfbmc.tests.test_explore import COUNTER, COUNTER_TWINS


class CandidatesTest(unittest.TestCase):
    def test_initial_candidates_are_the_start_posts_successors(self):
        self.assertEqual([[2]], Explorer(Diagram.from_json(COUNTER), COUNTER_TWINS).initial_candidates())

    def test_candidates_of_an_edge_come_per_parameter_valuation(self):
        diagram = dict(COUNTER, edges=[
            {"src": "a", "dst": "a", "event": "add(k)", "guard": "n + k ≤ 3", "post": "n' = n + k", "line": 6},
            {"src": "a", "dst": "b", "event": "stop", "guard": "n = 0", "post": "true", "line": 7},
        ])
        from csdfbmc.twins import Twins
        twins = Twins(domains={"k": [1, 2]}, clauses={
            "n = 2": lambda: [{"n": 2}], "n + k ≤ 3": lambda n, k: n + k <= 3,
            "n' = n + k": lambda n, k: [{"n": n + k}], "n = 0": lambda n: n == 0})
        ex = Explorer(Diagram.from_json(diagram), twins)
        self.assertEqual([{"params": {"k": 1}, "values": [3]}], ex.candidates("a", [2], 0))
        self.assertEqual([], ex.candidates("a", [2], 1))  # stop needs n = 0

    def test_a_bad_edge_index_is_refused(self):
        with self.assertRaises(IndexError):
            Explorer(Diagram.from_json(COUNTER), COUNTER_TWINS).candidates("a", [2], 5)


class NextCommandTest(unittest.TestCase):
    def run_next(self, tmp, *args):
        import os, tempfile
        d = tempfile.mkdtemp()
        diagram = os.path.join(d, "d.json")
        twins = os.path.join(d, "t.py")
        with open(diagram, "w", encoding="utf-8") as f:
            json.dump(COUNTER, f)
        with open(twins, "w", encoding="utf-8") as f:
            f.write('DOMAINS = {}\nCLAUSES = {"n = 2": lambda: [{"n": 2}], "n > 0": lambda n: n > 0, '
                    '"n\' = n - 1": lambda n: [{"n": n - 1}], "n = 0": lambda n: n == 0}\n')
        out = io.StringIO()
        with redirect_stdout(out):
            code = next_main([diagram, twins, *args])
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
