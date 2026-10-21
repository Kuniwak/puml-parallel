"""The explorer walks the product of control states and valuations.

The diagram is the JSON csdfparse prints, its clauses written in the language
of the reference Python glue; the checks give the domains the values of
received fields are drawn from.
"""
import unittest
from csdfbmc.tests import fixture

from csdfbmc.model import Diagram
from csdfbmc.protocol import EvaluatorError
from csdfbmc.checks import Checks
from csdfbmc import environment as environments


COUNTER = {
    "states": {
        "a": {"name": "counting", "vars": [{"name": "n", "type": "Nat"}], "line": 2},
        "b": {"name": "stopped", "vars": [], "line": 4},
    },
    "start_edge": {"dst": "a", "post": "[{'n': 2}]", "line": 5},
    "edges": [
        {"src": "a", "dst": "a", "event": "dec", "channel": "dec", "fields": [], "guard": "n > 0", "post": "[{'n': n - 1}]", "line": 6},
        {"src": "a", "dst": "b", "event": "stop", "channel": "stop", "fields": [], "guard": "n == 0", "post": "", "line": 7},
    ],
    "end_edge": None,
    "assumptions": [],
}

NO_CHECKS = Checks(types={})


class ExplorerTest(unittest.TestCase):
    def test_reaches_every_valuation_the_clauses_admit(self):
        result = fixture.explorer(COUNTER).run()
        self.assertEqual(
            {("a", (2,), ()), ("a", (1,), ()), ("a", (0,), ()), ("b", (), ())},
            set(result.states),
        )

    def test_reports_a_state_with_no_way_out_as_a_deadlock(self):
        result = fixture.explorer(COUNTER).run()
        self.assertEqual([("b", (), ())], result.deadlocks)
        self.assertEqual(["dec", "dec", "stop"], result.trace_to(("b", (), ())))

    def test_an_end_edge_is_a_way_out(self):
        diagram = dict(COUNTER, end_edge={"src": "b", "guard": "", "line": 8})
        result = fixture.explorer(diagram).run()
        self.assertEqual([], result.deadlocks)

    def test_finds_a_tau_cycle_among_the_reachable_states(self):
        diagram = dict(COUNTER, edges=COUNTER["edges"] + [
            {"src": "a", "dst": "a", "event": "tau", "channel": "tau", "fields": [], "guard": "n == 0", "post": "[{'n': n}]", "line": 9},
        ])
        result = fixture.explorer(diagram).run()
        self.assertEqual([[("a", (0,), ())]], result.tau_cycles)

    def test_event_parameters_range_over_their_domain(self):
        diagram = dict(COUNTER, edges=[
            {"src": "a", "dst": "a", "event": "add?k", "channel": "add", "fields": [{"dir": "?", "name": "k"}], "guard": "n + k <= 3", "post": "[{'n': n + k}]", "line": 6},
        ])
        result = fixture.explorer(diagram, Checks(types={"k": [1, 2]})).run()
        self.assertEqual({("a", (2,), ()), ("a", (3,), ())}, set(result.states))
        self.assertEqual(["add(k=1)"], result.trace_to(("a", (3,), ())))

    def without_its_guard(self):
        """COUNTER with dec's guard not formalised yet."""
        return dict(COUNTER, edges=[dict(COUNTER["edges"][0], guard="@opaque"), COUNTER["edges"][1]])

    def test_a_clause_not_formalised_is_refused_by_default(self):
        with self.assertRaises(KeyError) as cm:
            fixture.explorer(self.without_its_guard()).run()
        self.assertIn("@opaque", str(cm.exception))

    def test_a_clause_not_formalised_may_be_taken_as_true(self):
        result = fixture.explorer(self.without_its_guard(), opaque="true", max_states=10).run()
        # dec fires at n = 0 too and counts down past zero, until the bound.
        self.assertIn(("a", (-1,), ()), set(result.states))
        self.assertEqual(["@opaque"], result.opaque_clauses)

    def test_the_environment_decides_a_clause_not_formalised(self):
        # Setup
        cases = {
            "the function that is always false": (False, {("a", (2,), ())}),
            "a function of the names it reads": (lambda n: n > 0, {("a", (2,), ()), ("a", (1,), ()), ("a", (0,), ()), ("b", (), ())}),
        }
        for name, (given, want) in cases.items():
            with self.subTest(name):
                environment = environments.over({}, {"@opaque#1": given}, None)

                # Execute
                result = fixture.explorer(self.without_its_guard(), environment=environment, max_states=10).run()

                # Assert
                self.assertEqual(want, set(result.states))
                self.assertEqual([], result.opaque_clauses)

    def test_a_seed_walks_the_same_states_again_and_is_reported(self):
        # Setup
        def walk():
            return fixture.explorer(self.without_its_guard(), environment=environments.over({}, {}, 7), max_states=10).run()

        # Execute
        first, again = walk(), walk()

        # Assert
        self.assertEqual(first.states, again.states)
        self.assertEqual({"seed": 7, "given": [], "decided": ["@opaque#1"]}, first.to_json()["environment"])

    def test_the_bound_on_states_stops_the_walk(self):
        result = fixture.explorer(COUNTER, max_states=2).run()
        self.assertEqual(2, len(result.states))
        self.assertTrue(result.truncated)


class PostTest(unittest.TestCase):
    """A post clause is a collection of partial assignments to the variables
    after the step; the successors of an edge are their natural join."""

    def diagram(self, post, init="[{'x': 0, 'y': 0}]"):
        return fixture.diagram({
            "states": {"a": {"name": "a", "vars": [{"name": "x"}, {"name": "y"}], "line": 2}},
            "start_edge": {"dst": "a", "post": init, "line": 3},
            "edges": [{"src": "a", "dst": "a", "event": "step", "channel": "step", "fields": [], "guard": "", "post": post, "line": 4}],
            "end_edge": None, "assumptions": [],
        })

    def test_the_successors_are_the_join_of_the_clauses(self):
        result = fixture.explorer(self.diagram("[{'x': x + 1}] ∧ [{'y': y}, {'y': y + 1}]"), max_states=6).run()
        self.assertIn(("a", (1, 0), ()), result.states)
        self.assertIn(("a", (1, 1), ()), result.states)

    def test_two_clauses_constraining_one_variable_must_agree(self):
        result = fixture.explorer(self.diagram("[{'x': 1}, {'x': 2}] ∧ [{'x': 2}, {'x': 3}] ∧ [{'y': y}]"), max_states=6).run()
        self.assertEqual({("a", (0, 0), ()), ("a", (2, 0), ())}, set(result.states))

    def test_a_variable_no_clause_constrains_is_a_finding(self):
        result = fixture.explorer(self.diagram("[{'x': x + 1}]"), max_states=6).run()
        self.assertEqual([(("a", (0, 0), ()), "step", ("y",))], result.unconstrained)
        self.assertEqual([("a", (0, 0), ())], result.states)

    def test_a_post_that_returns_no_successor_is_reported(self):
        result = fixture.explorer(self.diagram("[]")).run()
        self.assertEqual([(("a", (0, 0), ()), "step")], result.empty_posts)
        self.assertEqual([("a", (0, 0), ())], result.deadlocks)

    def test_successor_values_are_frozen(self):
        result = fixture.explorer(self.diagram("[{'x': x, 'y': y}]", init="[{'x': {'k': [1]}, 'y': 0}]")).run()
        x = result.states[0][1][0]
        self.assertEqual({"k": (1,)}, dict(x))
        hash(x)

    def test_a_post_must_be_assignments(self):
        with self.assertRaisesRegex(EvaluatorError, "list or set of assignments"):
            fixture.explorer(self.diagram("x == y")).run()


class EventFieldsTest(unittest.TestCase):
    def test_an_edge_without_its_channel_and_fields_is_refused(self):
        diagram = {"states": {"a": {"name": "a", "vars": []}}, "start_edge": {"dst": "a", "post": ""},
                   "edges": [{"src": "a", "dst": "a", "event": "c?x", "guard": "", "post": ""}],
                   "end_edge": None, "assumptions": []}
        with self.assertRaisesRegex(ValueError, "no channel and fields"):
            Diagram.from_json(diagram)


if __name__ == "__main__":
    unittest.main()
