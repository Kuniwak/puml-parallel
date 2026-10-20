"""The explorer walks the product of control states and valuations.

The diagram is the JSON csdfparse prints; the twins are the Python functions a
clause's prose stands for, keyed by that prose, with the domains the values
are drawn from.
"""
import unittest

from csdfbmc.model import Diagram
from csdfbmc.twins import Twins
from csdfbmc.explore import Explorer


COUNTER = {
    "states": {
        "a": {"name": "counting", "vars": [{"name": "n", "type": "Nat"}], "line": 2},
        "b": {"name": "stopped", "vars": [], "line": 4},
    },
    "start_edge": {"dst": "a", "post": "n = 2", "line": 5},
    "edges": [
        {"src": "a", "dst": "a", "event": "dec", "guard": "n > 0", "post": "n' = n - 1", "line": 6},
        {"src": "a", "dst": "b", "event": "stop", "guard": "n = 0", "post": "true", "line": 7},
    ],
    "end_edge": None,
    "assumptions": [],
}

# A post twin returns the successor values of the variables it constrains; a
# guard twin decides. Domains are needed for event parameters only.
COUNTER_TWINS = Twins(
    domains={},
    clauses={
        "n = 2": lambda: [{"n": 2}],
        "n > 0": lambda n: n > 0,
        "n' = n - 1": lambda n: [{"n": n - 1}],
        "n = 0": lambda n: n == 0,
    },
)


class ExplorerTest(unittest.TestCase):
    def test_reaches_every_valuation_the_clauses_admit(self):
        result = Explorer(Diagram.from_json(COUNTER), COUNTER_TWINS).run()
        self.assertEqual(
            {("a", (2,), ()), ("a", (1,), ()), ("a", (0,), ()), ("b", (), ())},
            set(result.states),
        )

    def test_reports_a_state_with_no_way_out_as_a_deadlock(self):
        result = Explorer(Diagram.from_json(COUNTER), COUNTER_TWINS).run()
        self.assertEqual([("b", (), ())], result.deadlocks)
        self.assertEqual(["dec", "dec", "stop"], result.trace_to(("b", (), ())))

    def test_an_end_edge_is_a_way_out(self):
        diagram = dict(COUNTER, end_edge={"src": "b", "guard": "true", "line": 8})
        result = Explorer(Diagram.from_json(diagram), COUNTER_TWINS).run()
        self.assertEqual([], result.deadlocks)

    def test_finds_a_tau_cycle_among_the_reachable_states(self):
        diagram = dict(COUNTER, edges=COUNTER["edges"] + [
            {"src": "a", "dst": "a", "event": "tau", "guard": "n = 0", "post": "n' = n", "line": 9},
        ])
        twins = Twins(domains={}, clauses=dict(COUNTER_TWINS.clauses, **{"n' = n": lambda n: [{"n": n}]}))
        result = Explorer(Diagram.from_json(diagram), twins).run()
        self.assertEqual([[("a", (0,), ())]], result.tau_cycles)

    def test_event_parameters_range_over_their_domain(self):
        diagram = dict(COUNTER, edges=[
            {"src": "a", "dst": "a", "event": "add(k)", "guard": "n + k ≤ 3", "post": "n' = n + k", "line": 6},
        ])
        twins = Twins(
            domains={"k": [1, 2]},
            clauses={
                "n = 2": lambda: [{"n": 2}],
                "n + k ≤ 3": lambda n, k: n + k <= 3,
                "n' = n + k": lambda n, k: [{"n": n + k}],
            },
        )
        result = Explorer(Diagram.from_json(diagram), twins).run()
        self.assertEqual({("a", (2,), ()), ("a", (3,), ())}, set(result.states))
        self.assertEqual(["add(k=1)"], result.trace_to(("a", (3,), ())))

    def test_a_clause_without_a_twin_is_refused_by_default(self):
        twins = Twins(domains={}, clauses={k: v for k, v in COUNTER_TWINS.clauses.items() if k != "n > 0"})
        with self.assertRaises(KeyError) as cm:
            Explorer(Diagram.from_json(COUNTER), twins).run()
        self.assertIn("n > 0", str(cm.exception))

    def test_a_clause_without_a_twin_may_be_taken_as_true(self):
        twins = Twins(domains={}, clauses={k: v for k, v in COUNTER_TWINS.clauses.items() if k != "n > 0"})
        result = Explorer(Diagram.from_json(COUNTER), twins, opaque="true", max_states=10).run()
        # dec fires at n = 0 too and counts down past zero, until the bound.
        self.assertIn(("a", (-1,), ()), set(result.states))
        self.assertEqual(["n > 0"], result.opaque_clauses)

    def test_the_bound_on_states_stops_the_walk(self):
        result = Explorer(Diagram.from_json(COUNTER), COUNTER_TWINS, max_states=2).run()
        self.assertEqual(2, len(result.states))
        self.assertTrue(result.truncated)


class PostTwinTest(unittest.TestCase):
    """A post twin returns a collection of partial assignments to the variables
    after the step; the successors of an edge are their natural join."""

    def diagram(self, post):
        return Diagram.from_json({
            "states": {"a": {"name": "a", "vars": [{"name": "x"}, {"name": "y"}], "line": 2}},
            "start_edge": {"dst": "a", "post": "init", "line": 3},
            "edges": [{"src": "a", "dst": "a", "event": "step", "guard": "true", "post": post, "line": 4}],
            "end_edge": None, "assumptions": [],
        })

    def test_the_successors_are_the_join_of_the_clauses(self):
        twins = Twins(domains={}, clauses={
            "init": lambda: [{"x": 0, "y": 0}],
            "x' = x + 1": lambda x: [{"x": x + 1}],
            "y' ∈ {y, y + 1}": lambda y: [{"y": y}, {"y": y + 1}],
        })
        result = Explorer(self.diagram("x' = x + 1 ∧ y' ∈ {y, y + 1}"), twins, max_states=6).run()
        self.assertIn(("a", (1, 0), ()), result.states)
        self.assertIn(("a", (1, 1), ()), result.states)

    def test_two_clauses_constraining_one_variable_must_agree(self):
        twins = Twins(domains={}, clauses={
            "init": lambda: [{"x": 0, "y": 0}],
            "x' ∈ {1, 2}": lambda: [{"x": 1}, {"x": 2}],
            "x' ∈ {2, 3}": lambda: [{"x": 2}, {"x": 3}],
            "y' = y": lambda y: [{"y": y}],
        })
        result = Explorer(self.diagram("x' ∈ {1, 2} ∧ x' ∈ {2, 3} ∧ y' = y"), twins, max_states=6).run()
        self.assertEqual({("a", (0, 0), ()), ("a", (2, 0), ())}, set(result.states))

    def test_a_variable_no_clause_constrains_is_a_finding(self):
        twins = Twins(domains={}, clauses={
            "init": lambda: [{"x": 0, "y": 0}],
            "x' = x + 1": lambda x: [{"x": x + 1}],
        })
        result = Explorer(self.diagram("x' = x + 1"), twins, max_states=6).run()
        self.assertEqual([(("a", (0, 0), ()), "step", ("y",))], result.unconstrained)
        self.assertEqual([("a", (0, 0), ())], result.states)

    def test_a_post_that_returns_no_successor_is_reported(self):
        twins = Twins(domains={}, clauses={
            "init": lambda: [{"x": 0, "y": 0}],
            "never": lambda: [],
        })
        result = Explorer(self.diagram("never"), twins).run()
        self.assertEqual([(("a", (0, 0), ()), "step")], result.empty_posts)
        self.assertEqual([("a", (0, 0), ())], result.deadlocks)

    def test_successor_values_are_frozen(self):
        twins = Twins(domains={}, clauses={
            "init": lambda: [{"x": {"k": [1]}, "y": 0}],
            "same": lambda x, y: [{"x": x, "y": y}],
        })
        result = Explorer(self.diagram("same"), twins).run()
        x = result.states[0][1][0]
        self.assertEqual({"k": (1,)}, dict(x))
        hash(x)

    def test_a_post_twin_must_return_mappings(self):
        twins = Twins(domains={}, clauses={"init": lambda: [{"x": 0, "y": 0}], "bad": lambda x, y: x == y})
        with self.assertRaises(TypeError):
            Explorer(self.diagram("bad"), twins).run()


if __name__ == "__main__":
    unittest.main()
