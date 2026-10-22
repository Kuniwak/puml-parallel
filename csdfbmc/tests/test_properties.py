"""Invariants over the reachable states, ghost values updated by the events,
and cycles that avoid a given event (livelock under a fairness bound)."""
import unittest
from csdfbmc.tests import fixture

from csdfbmc.checks import Checks

from csdfbmc.tests.test_explore import COUNTER

# A balance that withdrawals reduce by amount plus fee and deposits raise by
# amount minus fee; the ghosts keep the running totals the diagram has not.
LEDGER = {
    "states": {"a": {"name": "ledger", "vars": [{"name": "残高"}], "line": 2}},
    "start_edge": {"dst": "a", "post": "[{'残高': 0}]", "line": 3},
    "edges": [
        {"src": "a", "dst": "a", "event": "入庫?額", "channel": "入庫", "fields": [{"dir": "?", "name": "額"}], "guard": "", "post": "[{'残高': 残高 + 額 - 1}]", "line": 4},
        {"src": "a", "dst": "a", "event": "出庫?額", "channel": "出庫", "fields": [{"dir": "?", "name": "額"}], "guard": "額 + 1 <= 残高", "post": "[{'残高': 残高 - 額 - 1}]", "line": 5},
    ],
    "end_edge": None,
    "assumptions": [],
}


def ledger_checks(**extra):
    return Checks(types={"額": [1, 2, 3]}, **extra)


# COUNTER with a bump that goes back up at n = 0, so the walk can avoid stop
# forever.
BUMPING = dict(COUNTER, edges=COUNTER["edges"] + [
    {"src": "a", "dst": "a", "event": "bump", "channel": "bump", "fields": [], "guard": "n == 0", "post": "[{'n': n + 1}]", "line": 9},
])


class InvariantTest(unittest.TestCase):
    def test_an_invariant_that_holds_reports_nothing(self):
        checks = ledger_checks(invariants={"残高は負にならない": lambda 残高: 残高 >= 0})
        result = fixture.explorer(LEDGER, checks, max_states=200).run()
        self.assertEqual([], result.violations)

    def test_a_broken_invariant_names_the_state_and_the_trace(self):
        checks = ledger_checks(invariants={"残高は 4 未満": lambda 残高: 残高 < 4})
        result = fixture.explorer(LEDGER, checks, max_states=200).run()
        # every balance from 4 up to the bound breaks it; the first one met comes first.
        self.assertGreater(len(result.violations), 1)
        name, state = result.violations[0]
        self.assertEqual("残高は 4 未満", name)
        self.assertEqual(("a", (4,), ()), state)
        self.assertEqual(["入庫(額=3)", "入庫(額=3)"], result.trace_to(state))


class GhostTest(unittest.TestCase):
    def test_ghosts_follow_the_events_and_invariants_may_read_them(self):
        checks = ledger_checks(
            ghosts={
                "総入庫": (0, {"入庫": lambda 総入庫, 額: 総入庫 + 額}),
                "総出庫": (0, {"出庫": lambda 総出庫, 額: 総出庫 + 額}),
                "総fee": (0, {"入庫": lambda 総fee: 総fee + 1, "出庫": lambda 総fee: 総fee + 1}),
            },
            invariants={"帳簿は総和に一致": lambda 残高, 総入庫, 総出庫, 総fee: 残高 == 総入庫 - 総出庫 - 総fee},
        )
        # The totals grow without bound, so the walk is cut at the bound.
        result = fixture.explorer(LEDGER, checks, max_states=300).run()
        self.assertEqual([], result.violations)
        # The ghosts are part of the state, so the walk tells states apart by them.
        self.assertTrue(any(ghost == (3, 0, 1) for _, _, ghost in result.states))

    def test_a_ghost_only_bounded_by_the_domain_bound_stops_the_walk(self):
        checks = ledger_checks(ghosts={"回数": (0, {"入庫": lambda 回数: 回数 + 1})})
        result = fixture.explorer(LEDGER, checks, max_states=50).run()
        self.assertTrue(result.truncated)


class AvoidingCycleTest(unittest.TestCase):
    def test_a_cycle_that_avoids_the_event_is_reported(self):
        # dec loops at n > 0 only downwards, so no cycle avoids stop; with the
        # bump that goes back up the walk can avoid stop forever.
        result = fixture.explorer(BUMPING, avoid="stop").run()
        self.assertEqual(1, len(result.avoiding_cycles))
        self.assertIn(("a", (0,), ()), result.avoiding_cycles[0])
        self.assertEqual([], fixture.explorer(COUNTER, avoid="stop").run().avoiding_cycles)


class WithinTest(unittest.TestCase):
    def test_within_restricts_the_avoiding_cycles_to_the_named_states(self):
        # The counter at n = 0 can bump and dec forever without stopping; the
        # same loop restricted to state b (which has no transitions) is empty.
        self.assertEqual(1, len(fixture.explorer(BUMPING, avoid="stop", within=["a"]).run().avoiding_cycles))
        self.assertEqual([], fixture.explorer(BUMPING, avoid="stop", within=["b"]).run().avoiding_cycles)


if __name__ == "__main__":
    unittest.main()
