"""csdfbmc --random-walk: walks from the start, each step one of the ways
out chosen at random by its event's weight, for findings deeper than a
breadth-first walk reaches."""
import unittest

from csdfbmc.randomwalk import RandomWalks
from csdfbmc.tests import fixture
from csdfbmc.tests.fixture import diagram, edge
from csdfbmc.tests.test_explore import BUMPING, COUNTER, TALLY, tally_checks


def walks(d, checks=None, max_depth=10, weights=None, seed=1, **options):
    return RandomWalks(fixture.explorer(d, checks, **options), max_depth=max_depth, weights=weights or {}, seed=seed)


class RandomWalkTest(unittest.TestCase):
    def test_reports_a_state_with_no_way_out_as_a_deadlock_with_its_trace(self):
        # Execute
        got = walks(COUNTER).run(1)

        # Assert
        self.assertEqual([("b", (), ())], got.result.deadlocks)
        self.assertEqual(["dec", "dec", "stop"], got.result.trace_to(got.result.deadlocks[0]))

    def test_a_walk_ends_at_the_depth_given(self):
        # Execute
        got = walks(BUMPING, max_depth=5, weights={"stop": 0}).run(3)

        # Assert
        self.assertEqual([], got.result.deadlocks)
        self.assertEqual((3, 15, 5), (got.walks, got.steps, got.deepest))

    def test_counts_the_events_fired_and_says_those_never_fired(self):
        # Execute
        got = walks(BUMPING, max_depth=5, weights={"stop": 0}).run(3)

        # Assert
        self.assertEqual(15, sum(got.fired.values()))
        self.assertEqual(["stop"], got.never_fired)

    def test_a_heavier_event_is_taken_more_often(self):
        # Setup
        d = diagram({"a": []}, "a", edges=[edge("a", "a", "x"), edge("a", "a", "y")])

        # Execute
        light = walks(d, max_depth=1, weights={"y": 1}).run(400)
        heavy = walks(d, max_depth=1, weights={"y": 9}).run(400)

        # Assert
        self.assertLess(light.fired["y"], 260)
        self.assertGreater(heavy.fired["y"], 320)

    def test_a_seed_gives_the_same_walks_whatever_order_they_are_taken_in(self):
        # Setup
        rw = walks(BUMPING, max_depth=6)

        # Execute
        forward = [rw.walk(i).events() for i in range(5)]
        backward = [rw.walk(i).events() for i in reversed(range(5))]

        # Assert
        self.assertEqual(forward, list(reversed(backward)))
        self.assertNotEqual(forward, [walks(BUMPING, max_depth=6, seed=2).walk(i).events() for i in range(5)])

    def test_reports_a_broken_invariant_and_ends_the_walk_there(self):
        # Setup
        checks = tally_checks(invariants={"在庫は 4 未満": lambda 在庫: 在庫 < 4})

        # Execute
        got = walks(TALLY, checks, max_depth=30, weights={"搬出": 0}).run(1)

        # Assert
        [(name, s)] = got.result.violations
        self.assertEqual("在庫は 4 未満", name)
        self.assertGreaterEqual(s[1][0], 4)
        self.assertLessEqual(len(got.result.trace_to(s)), got.deepest)

    def test_a_finding_met_again_is_kept_once(self):
        # Execute
        got = walks(COUNTER).run(5)

        # Assert
        self.assertEqual([("b", (), ())], got.result.deadlocks)
        self.assertEqual(1, got.result.groups()[0]["count"])
