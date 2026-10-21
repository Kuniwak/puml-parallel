"""Findings of one shape are reported once, with their count and the first
(shortest) trace; --first stops the walk at the first finding."""
import unittest
from csdfbmc.tests import fixture

from csdfbmc.twins import Twins

# A counter that deadlocks at every even value below 6 after a "halve" step,
# from several starting points: many deadlocks of one shape.
MANY = {
    "states": {"a": {"name": "running", "vars": [{"name": "n"}], "line": 2},
               "b": {"name": "stuck", "vars": [{"name": "n"}], "line": 3}},
    "start_edge": {"dst": "a", "post": "[{'n': 2}, {'n': 4}, {'n': 6}]", "line": 4},
    "edges": [
        {"src": "a", "dst": "a", "event": "dec", "channel": "dec", "fields": [], "guard": "n > 0", "post": "[{'n': n - 1}]", "line": 5},
        {"src": "a", "dst": "b", "event": "halve", "channel": "halve", "fields": [], "guard": "n % 2 == 0 and n > 0", "post": "[{'n': n // 2}]", "line": 6},
    ],
    "end_edge": {"src": "a", "guard": "n == 0", "line": 7},
    "assumptions": [],
}


class GroupTest(unittest.TestCase):
    def test_deadlocks_of_one_shape_are_one_group_with_a_count(self):
        result = fixture.explorer(MANY).run()
        self.assertGreater(len(result.deadlocks), 1)
        groups = result.groups()
        deadlock_groups = [g for g in groups if g["kind"] == "deadlock"]
        self.assertEqual(1, len(deadlock_groups))
        g = deadlock_groups[0]
        self.assertEqual(len(result.deadlocks), g["count"])
        self.assertEqual("b", g["at"])  # the control state id, as every report names it
        self.assertEqual("halve", g["after"])
        self.assertEqual(["halve"], g["trace"])  # the shortest representative

    def test_violations_group_by_invariant_and_state(self):
        checks = Twins(domains={}, clauses={}, invariants={"n < 5": lambda n: n < 5})
        result = fixture.explorer(MANY, checks).run()
        groups = [g for g in result.groups() if g["kind"] == "violation"]
        self.assertEqual([("n < 5", "a")], [(g["invariant"], g["at"]) for g in groups])
        self.assertEqual(2, groups[0]["count"])  # n = 6 and n = 5

    def test_first_stops_the_walk_at_the_first_finding(self):
        result = fixture.explorer(MANY, first=True).run()
        self.assertEqual(1, len(result.deadlocks))
        self.assertTrue(result.stopped_early)


if __name__ == "__main__":
    unittest.main()
