"""Partial-order reduction: the transitions of one instance that moves
independently of everything else are taken alone, and the walk finds what
it found without."""
import unittest

from csdfbmc.checks import Checks
from csdfbmc.tests import fixture
from csdfbmc.tests.fixture import atom, builtin, diagram, edge, end, eq, family, fld, idx
from csdfbmc.tests.test_explore import ACCOUNTS
from csdfbmc.values import sample

IDS = {"k": ["a", "b", "c"]}
OPEN, CLOSE, DROP = ACCOUNTS["edges"]

# ACCOUNTS without DROP: an instance never returns to where it was, so the
# cycle proviso does not expand every state in full.
ONE_WAY = dict(ACCOUNTS, edges=[OPEN, CLOSE])


def with_guard(e, *more):
    return dict(e, guard=e["guard"] + list(more))


def with_post(e, *more):
    return dict(e, post=e["post"] + list(more))


class IndependenceTest(unittest.TestCase):
    def test_independent_instances_are_walked_one_at_a_time(self):
        # Execute
        full = fixture.explorer(ONE_WAY, Checks(types=IDS)).run()
        reduced = fixture.explorer(ONE_WAY, Checks(types=IDS), por=True).run()

        # Assert
        self.assertEqual(27, len(full.states))
        self.assertLess(len(reduced.states), len(full.states))
        self.assertEqual(fixture.findings(full), fixture.findings(reduced))
        self.assertEqual(set(full.deadlocks), set(reduced.deadlocks))

    def test_what_ties_the_instances_together_keeps_every_transition(self):
        others = {"count": [0]}
        tests = {
            "a guard reading the whole map": (dict(ONE_WAY, edges=[with_guard(OPEN, atom("len($m) < 2")), CLOSE]), Checks(types=IDS)),
            "a guard reading another instance": (dict(ONE_WAY, edges=[with_guard(edge("r", "r", "OPEN?k?j", guard=OPEN["guard"], post=OPEN["post"]), builtin("notin", "j", "m")), CLOSE]),
                                                 Checks(types=IDS, typeof={"j": "k"})),
            "an invariant reading the map": (ONE_WAY, Checks(types=IDS, invariants={"few": lambda m: len(m) < 3})),
            "a ghost update on its event": (ONE_WAY, Checks(types=IDS, ghosts={"opened": (0, {"OPEN": lambda opened: opened + 1})})),
            "a ghost update reading the map": (ONE_WAY, Checks(types=IDS, ghosts={"size": (0, {"CLOSE": lambda m: len(m)})})),
            "a tau edge out of the state": (dict(ONE_WAY, edges=[OPEN, dict(CLOSE, channel="tau", label="tau?k")]), Checks(types=IDS)),
            "an edge to another state": (dict(ONE_WAY, states=["r", "s"], vars={"r": ["m"], "s": []}, edges=[OPEN, CLOSE, edge("r", "s", "STOP")]),
                                         Checks(types=IDS)),
            "an edge that moves the map and a variable": (
                dict(ONE_WAY, vars={"r": ["m", "n"]}, init_post=ONE_WAY["init_post"] + [atom("$n' == 0")],
                     edges=[with_post(OPEN, eq("n'", "n")), with_post(CLOSE, atom("$n' == $n"))]), Checks(types=dict(IDS, **others))),
        }
        for name, (d, checks) in tests.items():
            with self.subTest(name):
                # Execute
                full = fixture.explorer(d, checks).run()
                reduced = fixture.explorer(d, checks, por=True).run()

                # Assert
                self.assertEqual(len(full.states), len(reduced.states))
                self.assertEqual(fixture.findings(full), fixture.findings(reduced))


class SameFindingsTest(unittest.TestCase):
    """The findings with and without the reduction: deadlocks, broken
    invariants, cycles, empty posts and the bound reached."""

    def test_the_examples_find_the_same_with_partial_order_reduction(self):
        tests = {"withdraw": ({}, False), "wallet": ({"avoid": "確定", "within": ["sending"]}, False),
                 "attempts": ({"avoid": "CONFIRMED"}, False)}
        # No example moves an instance alone: RESEND reads the instance it
        # replaces, which ties the attempts together.
        for name, (options, fewer) in tests.items():
            with self.subTest(name):
                # Execute
                full, reduced = fixture.example(name, **options).run(), fixture.example(name, por=True, **options).run()

                # Assert
                self.assertTrue(fixture.findings(full))
                self.assertEqual(fixture.findings(full), fixture.findings(reduced))
                self.assertEqual(fewer, len(reduced.states) < len(full.states))

    def test_small_diagrams_find_the_same_with_partial_order_reduction(self):
        stuck = dict(CLOSE, guard=CLOSE["guard"] + [atom("${} > 0", fld(idx("m", "k"), "bal"))])
        tests = {
            "deadlocks with the map closed": (ONE_WAY, Checks(types=IDS), {}),
            "a cycle through every instance": (ACCOUNTS, Checks(types=IDS), {"avoid": "OPEN"}),
            "a broken invariant over another variable": (
                dict(ONE_WAY, vars={"r": ["m", "n"]}, init_post=ONE_WAY["init_post"] + [atom("$n' == 0")],
                     edges=[with_post(OPEN, eq("n'", "n")), with_post(CLOSE, eq("n'", "n")),
                            edge("r", "r", "INC", guard=[atom("$n < 2")], post=[atom("$n' == $n + 1"), eq("m'", "m")])]),
                Checks(types=IDS, invariants={"small": lambda n: n < 2}), {}),
            "an empty post": (dict(ONE_WAY, edges=[OPEN, with_post(CLOSE, atom("${} in ()", fld(idx("m'", "k"), "bal")))]), Checks(types=IDS), {}),
            "a guard that never holds": (dict(ONE_WAY, edges=[OPEN, stuck]), Checks(types=IDS), {}),
            "the bound reached": (ONE_WAY, Checks(types={"k": sample(["a", "b"], beyond=["c"])}), {}),
            "an end edge": (dict(ONE_WAY, ends=[end("r", [builtin("empty", "m")])]), Checks(types=IDS), {}),
        }
        for name, (d, checks, options) in tests.items():
            with self.subTest(name):
                # Execute
                full = fixture.explorer(d, checks, **options).run()
                reduced = fixture.explorer(d, checks, por=True, **options).run()

                # Assert
                self.assertEqual(fixture.findings(full), fixture.findings(reduced))
                self.assertEqual(set(full.deadlocks) <= set(full.states), set(reduced.deadlocks) <= set(full.states))
                self.assertEqual({s[0] for s in full.deadlocks}, {s[0] for s in reduced.deadlocks})
