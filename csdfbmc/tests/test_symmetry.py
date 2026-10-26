"""Symmetry reduction: states that differ by a renaming of the IDs are
stored as one, and the walk finds what it found without."""
import dataclasses
import unittest

from csdfbmc import environment
from csdfbmc.checks import Checks
from csdfbmc.tests import fixture
from csdfbmc.tests.fixture import atom, builtin, edge, exists, fld, idx
from csdfbmc.tests.test_explore import ACCOUNTS
from csdfbmc.tests.test_por import CLOSE, OPEN, ONE_WAY, with_guard
from csdfbmc.values import record, sample

SOURCE = 'TYPES = {"k": ["a", "b", "c"]}\nSYMMETRIC = ["k"]\n'


def checks(source=SOURCE, **tables):
    """The checks of the IDs a, b and c, symmetric, read from source."""
    return Checks(types=tables.pop("types", {"k": ["a", "b", "c"]}), symmetric=("k",), sources=[("checks.py", source)], **tables)


def ids_only(d, *guard):
    """d with OPEN guarded further by guard."""
    return dict(d, edges=[with_guard(OPEN, *guard)] + d["edges"][1:])


class RepresentativeTest(unittest.TestCase):
    def test_states_that_differ_by_a_renaming_of_the_ids_are_one(self):
        # Execute
        full = fixture.explorer(ONE_WAY, checks()).run()
        reduced = fixture.explorer(ONE_WAY, checks(), symmetry=True).run()

        # Assert
        self.assertEqual((27, 10), (len(full.states), len(reduced.states)))
        self.assertEqual(fixture.findings(full), fixture.findings(reduced))

    def test_every_trace_it_reports_is_a_run_of_the_diagram(self):
        # Setup
        explorer = fixture.explorer(ONE_WAY, checks())

        # Execute
        reduced = fixture.explorer(ONE_WAY, checks(), symmetry=True).run()

        # Assert
        for s in reduced.states:
            if reduced.parents[s] is not None:
                prev, label, _ = reduced.parents[s]
                self.assertIn(s, [t for _, taken, t, _ in explorer.successors(prev) if taken == label])


class RefusalTest(unittest.TestCase):
    def test_what_could_tell_the_ids_apart_is_refused(self):
        tests = {
            "no SYMMETRIC": (ONE_WAY, Checks(types={"k": ["a"]}, sources=[("c.py", "")]), {}, "declare SYMMETRIC"),
            "a type TYPES does not declare": (ONE_WAY, dataclasses.replace(checks(), symmetric=("j",)), {}, "'j', which TYPES"),
            "values that are not strings": (ONE_WAY, checks(types={"k": [1, 2]}), {}, "not all strings"),
            "a type holding some IDs only": (ONE_WAY, checks(types={"k": ["a", "b", "c"], "j": ["a"]}), {}, "'j' are not closed"),
            "an atom comparing an ID with a constant": (ids_only(ONE_WAY, atom("$k != 'a'")), checks(), {}, "tells the symmetric IDs apart.*'a'"),
            "an atom ordering IDs": (ids_only(ONE_WAY, exists("k", atom("${} < $k", fixture.bound(0)))), checks(), {}, "ordered"),
            "a function of the environment nothing gives": (
                ids_only(ONE_WAY, atom("$k が 予約済み")), checks(), {"environment": environment.over({"$_0 が 予約済み": None}, {})},
                "^nothing decides the environment's functions '\\$_0 が 予約済み'; give each in ENVIRONMENT or with --env NAME=true\\|false$"),
            "two functions of the environment nothing gives": (
                ids_only(ONE_WAY, atom("$k が 予約済み"), atom("$k が 空き")), checks(),
                {"environment": environment.over({"$_0 が 予約済み": None, "$_0 が 空き": None}, {})},
                "^nothing decides the environment's functions '\\$_0 が 予約済み', '\\$_0 が 空き'; "),
            "a function of the environment given as Python that tells IDs apart": (
                ids_only(ONE_WAY, atom("$k が 予約済み")), checks(), {"environment": environment.over({"$_0 が 予約済み": lambda k: k == "a"}, {})},
                "'\\$_0 が 予約済み'.*tells the symmetric IDs apart"),
            "a function of the environment given as Python whose result is no renaming": (
                ids_only(ONE_WAY, builtin("ne", {"kind": "app", "fn": "next", "args": [fixture.term("k")]}, "k")),
                checks(environment={"next": lambda k: {"a": "b", "b": "c", "c": "a"}[k]}), {}, "'next'.*tells the symmetric IDs apart"),
            "an invariant naming an ID": (ONE_WAY, checks(SOURCE + 'INVARIANTS = {"x": lambda m: "a" not in m}\n'), {}, "checks.py:3.*'a'"),
            "an invariant sorting a map of IDs": (ONE_WAY, checks(SOURCE + 'INVARIANTS = {"x": lambda m: sorted(m) != []}\n'), {}, "sorted orders m"),
        }
        for name, (d, c, options, said) in tests.items():
            with self.subTest(name):
                # Execute, Assert
                with self.assertRaisesRegex(ValueError, said):
                    fixture.explorer(d, c, symmetry=True, **options)

    def test_an_id_an_atom_puts_where_none_was_foreseen_is_refused(self):
        # Setup: last has no type, and only the atom puts an ID in it, so the
        # atoms reading it were asked without it.
        d = dict(ONE_WAY, vars={"r": ["m", "last"]}, init_post=ONE_WAY["init_post"] + [atom("$last' == ''")],
                 edges=[dict(OPEN, post=OPEN["post"] + [atom("$last' == $k")]), dict(CLOSE, post=CLOSE["post"] + [builtin("eq", "last'", "last")])])
        explorer = fixture.explorer(d, checks(), symmetry=True)

        # Execute, Assert
        with self.assertRaisesRegex(ValueError, "last holds 'a'"):
            explorer.run()

    def test_what_tells_no_ids_apart_is_accepted(self):
        tests = {
            "a builtin comparing IDs": (ids_only(ONE_WAY, exists("k", builtin("ne", fixture.bound(0), "k"))), checks(), {}),
            "an atom looking an ID up": (ids_only(ONE_WAY, atom("$k not in $m")), checks(), {}),
            "an atom ordering what an ID indexes": (dict(ONE_WAY, edges=[OPEN, with_guard(CLOSE, atom("${} >= 0", fld(idx("m", "k"), "bal")))]), checks(), {}),
            "a function of the environment given a constant": (
                ids_only(ONE_WAY, atom("$k が 予約済み")), checks(), {"environment": environment.over({"$_0 が 予約済み": True}, {})}),
            "a function of the environment given as Python that renames with the IDs": (
                ids_only(ONE_WAY, atom("$k が 予約済み")), checks(), {"environment": environment.over({"$_0 が 予約済み": lambda k: k in ("a", "b", "c")}, {})}),
            "a type of records of IDs": (ONE_WAY, checks(types={"k": ["a", "b", "c"], "r": record(id=["a", "b", "c"])}), {}),
        }
        for name, (d, c, options) in tests.items():
            with self.subTest(name):
                # Execute
                full = fixture.explorer(d, c, **options).run()
                reduced = fixture.explorer(d, c, symmetry=True, **options).run()

                # Assert
                self.assertLess(len(reduced.states), len(full.states))
                self.assertEqual(fixture.findings(full), fixture.findings(reduced))


class SameFindingsTest(unittest.TestCase):
    def test_the_attempts_example_finds_the_same_in_fewer_states(self):
        for options in ({}, {"avoid": "CONFIRMED"}, {"por": True, "cone": True}):
            with self.subTest(**options):
                # Execute
                full = fixture.example("attempts", **options).run()
                reduced = fixture.example("attempts", symmetry=True, **options).run()

                # Assert
                self.assertTrue(fixture.findings(full))
                self.assertEqual(fixture.findings(full), fixture.findings(reduced))
                self.assertLess(len(reduced.states), len(full.states))

    def test_small_diagrams_find_the_same_with_every_reduction(self):
        tests = {
            "deadlocks": (ONE_WAY, checks(), {}),
            "a cycle that avoids an event": (ACCOUNTS, checks(), {"avoid": "OPEN"}),
            "a broken invariant": (ONE_WAY, checks(SOURCE + 'INVARIANTS = {"few": lambda m: len(m) < 2}\n', invariants={"few": lambda m: len(m) < 2}), {}),
            "the bound reached": (ONE_WAY, checks(types={"k": sample(["a", "b"], beyond=["c"])}), {}),
            "an empty post": (dict(ONE_WAY, edges=[OPEN, dict(CLOSE, post=CLOSE["post"] + [atom("${} in ()", fld(idx("m'", "k"), "bal"))])]), checks(), {}),
        }
        for name, (d, c, options) in tests.items():
            for reductions in ({"symmetry": True}, {"symmetry": True, "por": True, "cone": True}):
                with self.subTest(name, **reductions):
                    # Execute
                    full = fixture.explorer(d, c, **options).run()
                    reduced = fixture.explorer(d, c, **options, **reductions).run()

                    # Assert
                    self.assertEqual(fixture.findings(full), fixture.findings(reduced))
