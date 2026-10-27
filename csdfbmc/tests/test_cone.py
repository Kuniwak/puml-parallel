"""The cone of influence: a variable nothing that decides a step or a
property reads is left out, and the walk finds what it found with it."""
import unittest

from csdfbmc.checks import Checks
from csdfbmc.tests import fixture
from csdfbmc.tests.fixture import atom, diagram, edge, eq
from csdfbmc.tests.test_explore import MEMBERS, COUNTER

# COUNTER with a memo of the last value noted, which nothing reads.
MEMO = diagram({"a": ["n", "memo"], "b": []}, "a", init_post=[atom("$n' == 2"), atom("$memo' == 0")], edges=[
    edge("a", "a", "dec", guard=[atom("$n > 0")], post=[atom("$n' == $n - 1"), eq("memo'", "memo")]),
    edge("a", "a", "note?x", post=[eq("memo'", "x"), eq("n'", "n")]),
    edge("a", "b", "stop", guard=[atom("$n == 0")]),
])
MEMO_TYPES = {"x": [1, 2, 3]}


def with_edge(d, i, e):
    """d with its i-th edge e."""
    edges = list(d["edges"])
    edges[i] = e
    return fixture.redeclared(dict(d, edges=edges))


class LeftOutTest(unittest.TestCase):
    def test_a_variable_nothing_reads_is_left_out(self):
        # Execute
        full = fixture.explorer(MEMO, Checks(types=MEMO_TYPES)).run()
        reduced = fixture.explorer(MEMO, Checks(types=MEMO_TYPES), cone=True).run()

        # Assert
        self.assertEqual(("memo",), reduced.left_out)
        self.assertEqual((13, 4), (len(full.states), len(reduced.states)))
        self.assertEqual(fixture.findings(full), fixture.findings(reduced))

    def test_a_variable_that_decides_something_is_kept(self):
        tests = {
            "read by a guard": (with_edge(MEMO, 2, edge("a", "b", "stop", guard=[atom("$n == 0"), atom("$memo >= 0")])), Checks(types=MEMO_TYPES)),
            "read by an end edge": (dict(MEMO, ends=[fixture.end("a", [atom("$memo == 3")])]), Checks(types=MEMO_TYPES)),
            "read by an invariant": (MEMO, Checks(types=MEMO_TYPES, invariants={"small": lambda memo: memo < 3})),
            "read by a ghost update": (MEMO, Checks(types=MEMO_TYPES, ghosts={"g": (0, {"dec": lambda memo: memo})})),
            "sent by an event": (with_edge(MEMO, 0, edge("a", "a", "dec!memo", guard=[atom("$n > 0")], post=[atom("$n' == $n - 1"), eq("memo'", "memo")])), Checks(types=MEMO_TYPES)),
            "read by a post atom": (with_edge(MEMO, 0, edge("a", "a", "dec", guard=[atom("$n > 0")], post=[atom("$n' == $n - 1 + $memo * 0"), eq("memo'", "memo")])), Checks(types=MEMO_TYPES)),
            "copied into a kept variable": (with_edge(MEMO, 1, edge("a", "a", "note?x", post=[eq("memo'", "x"), eq("n'", "memo")])), Checks(types=MEMO_TYPES)),
            "written by two clauses that must agree": (with_edge(MEMO, 0, edge("a", "a", "dec", guard=[atom("$n > 0")], post=[atom("$n' == $n - 1"), eq("memo'", "memo"), atom("$memo' in (0, 1, 3)")])), Checks(types=MEMO_TYPES)),
        }
        for name, (d, checks) in tests.items():
            with self.subTest(name):
                # Execute
                full = fixture.explorer(d, checks).run()
                reduced = fixture.explorer(d, checks, cone=True).run()

                # Assert
                self.assertEqual((), reduced.left_out)
                self.assertEqual(len(full.states), len(reduced.states))
                self.assertEqual(fixture.findings(full), fixture.findings(reduced))

    def test_a_map_of_a_family_is_kept(self):
        # Execute
        reduced = fixture.explorer(MEMBERS, Checks(types={"k": ["a"]}), cone=True).run()

        # Assert
        self.assertEqual((), reduced.left_out)

    def test_what_a_property_reads_must_be_known(self):
        tests = {
            "an invariant": Checks(types=MEMO_TYPES, invariants={"any": lambda **names: True}),
            "a ghost update": Checks(types=MEMO_TYPES, ghosts={"g": (0, {"dec": lambda **names: 0})}),
        }
        for name, checks in tests.items():
            with self.subTest(name):
                # Execute, Assert
                with self.assertRaisesRegex(ValueError, "--cone"):
                    fixture.explorer(MEMO, checks, cone=True)


class SameFindingsTest(unittest.TestCase):
    """The findings with and without the cone, on the examples and on small
    diagrams: deadlocks, broken invariants, cycles and the bound reached."""

    def test_the_examples_find_the_same_with_the_cone(self):
        for name in fixture.EXAMPLES:
            with self.subTest(name):
                # Execute
                full, reduced = fixture.example(name).run(), fixture.example(name, cone=True).run()

                # Assert
                self.assertTrue(fixture.findings(full))
                self.assertEqual(fixture.findings(full), fixture.findings(reduced))
                self.assertLessEqual(len(reduced.states), len(full.states))

    def test_small_diagrams_find_the_same_with_the_cone(self):
        bumping = dict(MEMO, edges=MEMO["edges"] + [edge("a", "a", "bump", guard=[atom("$n == 0")], post=[atom("$n' == $n + 1"), eq("memo'", "memo")])])
        tests = {
            "a deadlock": (MEMO, Checks(types=MEMO_TYPES), {}),
            "a broken invariant": (MEMO, Checks(types=MEMO_TYPES, invariants={"positive": lambda n: n > 0}), {}),
            "a cycle that avoids an event": (bumping, Checks(types=MEMO_TYPES), {"avoid": "stop"}),
            "a tau cycle": (with_edge(MEMO, 1, edge("a", "a", "tau", post=[atom("$memo' == 1"), eq("n'", "n")])), Checks(types={}), {}),
            "a counter": (COUNTER, Checks(types={}), {}),
        }
        for name, (d, checks, options) in tests.items():
            with self.subTest(name):
                # Execute
                full = fixture.explorer(d, checks, **options).run()
                reduced = fixture.explorer(d, checks, cone=True, **options).run()

                # Assert
                self.assertEqual(fixture.findings(full), fixture.findings(reduced))
