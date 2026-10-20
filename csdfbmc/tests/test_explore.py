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

COUNTER_TWINS = Twins(
    domains={"n": range(0, 4)},
    clauses={
        "n = 2": lambda n: n == 2,
        "n > 0": lambda n: n > 0,
        "n' = n - 1": lambda n, n_: n_ == n - 1,
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
        twins = Twins(domains=COUNTER_TWINS.domains, clauses=dict(COUNTER_TWINS.clauses, **{"n' = n": lambda n, n_: n_ == n}))
        result = Explorer(Diagram.from_json(diagram), twins).run()
        self.assertEqual([[("a", (0,), ())]], result.tau_cycles)

    def test_event_parameters_range_over_their_domain(self):
        diagram = dict(COUNTER, edges=[
            {"src": "a", "dst": "a", "event": "add(k)", "guard": "n + k ≤ 3", "post": "n' = n + k", "line": 6},
        ])
        twins = Twins(
            domains={"n": range(0, 4), "k": [1, 2]},
            clauses={
                "n = 2": lambda n: n == 2,
                "n + k ≤ 3": lambda n, k: n + k <= 3,
                "n' = n + k": lambda n, k, n_: n_ == n + k,
            },
        )
        result = Explorer(Diagram.from_json(diagram), twins).run()
        self.assertEqual({("a", (2,), ()), ("a", (3,), ())}, set(result.states))
        self.assertEqual(["add(k=1)"], result.trace_to(("a", (3,), ())))

    def test_a_clause_without_a_twin_is_refused_by_default(self):
        twins = Twins(domains={"n": range(0, 4)}, clauses={k: v for k, v in COUNTER_TWINS.clauses.items() if k != "n > 0"})
        with self.assertRaises(KeyError) as cm:
            Explorer(Diagram.from_json(COUNTER), twins).run()
        self.assertIn("n > 0", str(cm.exception))

    def test_a_clause_without_a_twin_may_be_taken_as_true(self):
        twins = Twins(domains={"n": range(0, 4)}, clauses={k: v for k, v in COUNTER_TWINS.clauses.items() if k != "n > 0"})
        result = Explorer(Diagram.from_json(COUNTER), twins, opaque="true").run()
        # dec fires at n = 0 too, but no value in the domain satisfies n' = n - 1.
        self.assertEqual({("a", (2,), ()), ("a", (1,), ()), ("a", (0,), ()), ("b", (), ())}, set(result.states))
        self.assertEqual(["n > 0"], result.opaque_clauses)

    def test_the_bound_on_states_stops_the_walk(self):
        result = Explorer(Diagram.from_json(COUNTER), COUNTER_TWINS, max_states=2).run()
        self.assertEqual(2, len(result.states))
        self.assertTrue(result.truncated)


if __name__ == "__main__":
    unittest.main()
