"""Why a state is stuck: per edge out of a deadlock, the conjunct of the
guard false on the state, or the one no value of the fields tried makes true."""
import json
import unittest

from csdfbmc.checks import Checks
from csdfbmc.tests import fixture
from csdfbmc.tests.fixture import builtin, diagram, edge, eq, lit
from csdfbmc.values import sample

# n is 0; the first edge needs n to be 1, the second a value of x other than n.
STUCK = diagram({"a": ["n"], "b": []}, "a", init_post=[eq("n'", lit(0))], edges=[
    edge("a", "b", "go", guard=[eq("n", lit(1))], line=3),
    edge("a", "b", "pick?x", guard=[builtin("ne", "x", "n")], line=4),
], types={"n": "N"}, channels={"pick": ["X"]})


def walk(test, *args, checks='TYPES = {"N": [0, 1], "X": [0]}\n'):
    from csdfbmc.__main__ import main
    return fixture.run_main(test, main, STUCK, "--evaluator-module", "csdfbmc.python_glue", *args, checks=checks)


class WhyStuckTest(unittest.TestCase):
    def test_each_edge_says_the_conjunct_false_on_the_state_or_for_every_value_tried(self):
        # Setup
        e = fixture.explorer(STUCK, Checks(types={"N": [0, 1], "X": [0]}))

        # Execute
        stuck = e.why_stuck(("a", (0,), ()))

        # Assert
        self.assertEqual([
            {"edge": "a --> b : go (line 3)", "reason": "state", "conjunct": "n == 1", "tried": 1},
            {"edge": "a --> b : pick?x (line 4)", "reason": "fields", "fields": ["x"], "conjunct": "x != n", "tried": 1,
             "hint": "X is a plain list: give it as sample(values, beyond=[...]) for a value outside it to tell the bound from a deadlock"},
        ], stuck)

    def test_no_hint_for_a_type_given_as_a_sample(self):
        # Setup
        e = fixture.explorer(STUCK, Checks(types={"N": [0, 1], "X": sample([0], beyond=[0.5])}))

        # Execute
        stuck = e.why_stuck(("a", (0,), ()))

        # Assert
        self.assertNotIn("hint", stuck[1])

    def test_the_report_says_it_under_the_deadlock(self):
        # Execute
        code, out, err = walk(self)

        # Assert
        self.assertEqual(1, code, err)
        self.assertIn("deadlock at a [0] after []\n"
                      "  stuck: a --> b : go (line 3): guard false on the state: n == 1\n"
                      "  stuck: a --> b : pick?x (line 4): no value of ?x satisfies x != n (tried 1); "
                      "X is a plain list: give it as sample(values, beyond=[...]) for a value outside it to tell the bound from a deadlock\n", out)

    def test_the_json_gives_it_with_the_deadlock(self):
        # Execute
        _, out, _ = walk(self, "--json")

        # Assert
        group = [g for g in json.loads(out)["groups"] if g["kind"] == "deadlock"][0]
        self.assertEqual(["state", "fields"], [s["reason"] for s in group["stuck"]])
