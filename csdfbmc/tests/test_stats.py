"""csdfbmc --stats: per edge, how much the walk tried and enumerated, and
what nothing determined, which ranged over its type."""
import json
import unittest

from csdfbmc.tests import fixture
from csdfbmc.tests.fixture import atom, diagram, edge

# n counts to 2; inc forgets to keep q, which so ranges over its type.
FORGETFUL = diagram({"a": ["n", "q"]}, "a", init_post=[atom("$n' == 0"), atom("$q' == 0")], edges=[
    edge("a", "a", "inc", guard=[atom("$n < 2")], post=[atom("$n' == $n + 1")], line=7),
], types={"n": "N", "q": "Q"})
CHECKS = 'TYPES = {"N": [0, 1, 2], "Q": [0, 1]}\n'


def walk(test, *args):
    from csdfbmc.__main__ import main
    return fixture.run_main(test, main, FORGETFUL, "--evaluator-module", "csdfbmc.python_glue", *args, checks=CHECKS)


class StatsTest(unittest.TestCase):
    def test_each_edge_says_what_it_tried_and_what_ranged_over_its_type(self):
        # Execute
        code, out, err = walk(self, "--stats")

        # Assert
        self.assertEqual(1, code, err)
        self.assertIn("stats per edge, most valuations checked first:\n"
                      "  a --> a : inc (line 7): tried 5, guard held 3, valuations checked 6, successors 6; "
                      "ranged over their types: q'\n", out)

    def test_the_json_gives_them_too_with_workers_or_without(self):
        for args in ((), ("--jobs", "2")):
            with self.subTest(args=args):
                # Execute
                _, out, err = walk(self, "--stats", "--json", *args)

                # Assert
                self.assertEqual([{"edge": "a --> a : inc (line 7)", "tried": 5, "guard_held": 3, "checked": 6,
                                   "successors": 6, "ranged": ["q'"]}], json.loads(out)["stats"], err)

    def test_without_it_nothing_is_said(self):
        # Execute
        _, out, _ = walk(self)
        _, js, _ = walk(self, "--json")

        # Assert
        self.assertNotIn("stats", out)
        self.assertNotIn("stats", json.loads(js))
