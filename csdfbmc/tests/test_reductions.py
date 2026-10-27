"""The reductions together: every combination of --cone, --por and
--symmetry finds the shapes the full walk finds, on the examples and on
small diagrams."""
import unittest

from csdfbmc.checks import Checks
from csdfbmc.tests import fixture
from csdfbmc.tests.fixture import atom, bound, edge, eq, exists, fld, idx
from csdfbmc.tests.test_cone import MEMO, MEMO_TYPES
from csdfbmc.tests.test_explore import MEMBERS
from csdfbmc.tests.test_por import CLOSE, IDS, OPEN, ONE_WAY, with_post
from csdfbmc.tests.test_symmetry import SOURCE, checks as symmetric
from csdfbmc.values import sample

WITHOUT_SYMMETRY = ({"cone": True, "por": True},)
WITH_SYMMETRY = WITHOUT_SYMMETRY + ({"por": True, "symmetry": True}, {"cone": True, "symmetry": True},
                                    {"cone": True, "por": True, "symmetry": True})

# ONE_WAY with a memo of the last ID opened, which nothing reads, and a
# count every edge keeps.
NOTED = dict(ONE_WAY, vars=fixture.typed_vars({"r": ["m", "memo", "n"]}), init_post=ONE_WAY["init_post"] + [exists("k", eq("memo'", bound(0))), atom("$n' == 0")],
             edges=[with_post(OPEN, eq("memo'", "k"), eq("n'", "n")), with_post(CLOSE, eq("memo'", "memo"), eq("n'", "n"))])


class CombinedTest(unittest.TestCase):
    def test_the_examples_find_the_same_with_the_reductions_combined(self):
        tests = {
            "printer": ({}, WITHOUT_SYMMETRY),
            "printer_retry": ({"avoid": "完了", "within": ["printing"]}, WITHOUT_SYMMETRY),
            "attempts": ({"avoid": "DONE"}, WITH_SYMMETRY),
        }
        for name, (options, combinations) in tests.items():
            full = fixture.example(name, **options).run()
            for reductions in combinations:
                with self.subTest(name, **reductions):
                    # Execute
                    reduced = fixture.example(name, **options, **reductions).run()

                    # Assert
                    self.assertTrue(fixture.findings(full))
                    self.assertEqual(fixture.findings(full), fixture.findings(reduced))
                    self.assertLessEqual(len(reduced.states), len(full.states))

    def test_small_diagrams_find_the_same_with_the_reductions_combined(self):
        tests = {
            "a memo nothing reads": (MEMO, Checks(types=MEMO_TYPES), {}, WITHOUT_SYMMETRY),
            "deadlocks among instances": (ONE_WAY, symmetric(), {}, WITH_SYMMETRY),
            "a cycle through every instance": (MEMBERS, symmetric(), {"avoid": "STOP"}, WITH_SYMMETRY),
            "a memo of an ID and a framed count": (NOTED, symmetric(types=dict(IDS, n=[0, 1])), {}, WITH_SYMMETRY),
            "a broken invariant": (ONE_WAY, symmetric(SOURCE + 'INVARIANTS = {"few": lambda m: len(m) < 2}\n',
                                                      invariants={"few": lambda m: len(m) < 2}), {}, WITH_SYMMETRY),
            "the bound reached": (ONE_WAY, symmetric(types={"k": sample(["a", "b"], beyond=["c"])}), {}, WITH_SYMMETRY),
            "an empty post": (dict(ONE_WAY, edges=[OPEN, with_post(CLOSE, atom("${} in ()", fld(idx("m'", "k"), "cnt")))]), symmetric(), {}, WITH_SYMMETRY),
            "a tau cycle": (dict(MEMO, edges=MEMO["edges"][:1] + [edge("a", "a", "tau", post=[atom("$memo' == 1"), eq("n'", "n")])] + MEMO["edges"][2:]),
                            Checks(types={}), {}, WITHOUT_SYMMETRY),
        }
        for name, (d, checks, options, combinations) in tests.items():
            full = fixture.explorer(d, checks, **options).run()
            for reductions in combinations:
                with self.subTest(name, **reductions):
                    # Execute
                    reduced = fixture.explorer(d, checks, **options, **reductions).run()

                    # Assert
                    self.assertTrue(fixture.findings(full))
                    self.assertEqual(fixture.findings(full), fixture.findings(reduced))
                    self.assertLessEqual(len(reduced.states), len(full.states))
