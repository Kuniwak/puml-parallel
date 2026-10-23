"""A clause's value: a guard's truth value, a post's valuations after the
step (docs/CLAUSE_DESIGN.md, O7)."""
import unittest

from csdfbmc import environment
from csdfbmc.formula import Evaluator, PostError, solve
from csdfbmc.meaning import AtomMeaning
from csdfbmc.model import formula_of
from csdfbmc.tests import fixture
from csdfbmc.tests.fixture import atom, builtin, eq, exists, fld, idx
from csdfbmc.values import FrozenMap, Rec

TYPES = {"T": [1, 2], "n": [0, 1, 2], "k": ["a", "b"]}


def evaluator(env=environment.NONE):
    return Evaluator(AtomMeaning(fixture.glue(), env, lambda n: TYPES[n]), lambda t: TYPES[t])


def inst(state, **fields):
    return Rec(state, FrozenMap(fields))


class GuardTest(unittest.TestCase):
    def test_a_builtin_is_the_checker_s(self):
        m = FrozenMap({"a": inst("open", x=1)})
        tests = {
            "eq of equal values": ([eq("x", "y")], {"x": 1, "y": 1}, True),
            "eq tells a truth value from the number it equals": ([eq("x", "y")], {"x": 1, "y": True}, False),
            "ne": ([builtin("ne", "x", "y")], {"x": 1, "y": 2}, True),
            "in": ([builtin("in", "k", "m")], {"k": "a", "m": m}, True),
            "notin": ([builtin("notin", "k", "m")], {"k": "b", "m": m}, True),
            "empty": ([builtin("empty", "m")], {"m": FrozenMap()}, True),
            "at the instance's state": ([builtin("at", idx("m", "k"), state="open")], {"k": "a", "m": m}, True),
            "at another state": ([builtin("at", idx("m", "k"), state="closed")], {"k": "a", "m": m}, False),
            "update": ([builtin("update", "m2", "m", "k", "v")], {"m2": m.set("b", 3), "m": m, "k": "b", "v": 3}, True),
            "frame, the others kept": ([builtin("frame", "m2", "m", "k")], {"m2": m.set("b", 3), "m": m, "k": "b"}, True),
            "frame, another changed": ([builtin("frame", "m2", "m", "k")], {"m2": m.set("b", 3), "m": m, "k": "a"}, False),
            "∃ over the type it names": ([exists("T", eq(fixture.bound(0), "x"))], {"x": 2}, True),
            "∃ with no value that holds": ([exists("T", eq(fixture.bound(0), "x"))], {"x": 3}, False),
            "an atom, through the glue": ([atom("$x > $y")], {"x": 2, "y": 1}, True),
            "true": ([], {}, True),
        }
        for name, (clause, before, want) in tests.items():
            with self.subTest(name):
                # Execute
                got = evaluator().holds(formula_of(clause), before)

                # Assert
                self.assertEqual(want, got)

    def test_a_clause_that_reads_an_entry_its_map_does_not_have_is_false(self):
        tests = {
            "an atom": [atom("${} > 0", fld(idx("m", "k"), "x"))],
            "a builtin": [builtin("at", idx("m", "k"), state="open")],
            "under ∃": [exists("T", atom("${} > 0", fld(idx("m", "k"), "x")))],
        }
        for name, clause in tests.items():
            with self.subTest(name):
                # Execute
                got = evaluator().holds(formula_of(clause), {"m": FrozenMap(), "k": "a"})

                # Assert
                self.assertFalse(got)

    def test_a_glue_sees_an_instance_as_its_fields(self):
        # Setup
        clause = formula_of([atom("${} == {'x': 1}", idx("m", "k"))])

        # Execute
        got = evaluator().holds(clause, {"m": FrozenMap({"a": inst("open", x=1)}), "k": "a"})

        # Assert
        self.assertTrue(got)


class SolveTest(unittest.TestCase):
    def solve(self, post, names, before=None, env=environment.NONE):
        return solve(evaluator(env), formula_of(post), tuple(names), before or {}, lambda n: TYPES[n], "the edge")

    def test_the_values_after_the_step_come_from_what_determines_them(self):
        m = FrozenMap({"a": inst("open", x=1)})
        tests = {
            "an equality builtin": ([eq("n'", "x")], ["n"], {"x": 7}, [{"n": 7}]),
            "an equality of a value outside its type (Q5)": ([eq("n", "n'")], ["n"], {"n": 9}, [{"n": 9}]),
            "a post atom of the glue": ([atom("$n' in ($x, $x + 1)")], ["n"], {"x": 1}, [{"n": 1}, {"n": 2}]),
            "@empty": ([builtin("empty", "m'")], ["m"], {}, [{"m": FrozenMap()}]),
            "@update": ([builtin("update", "m'", "m", "k", "v")], ["m"], {"m": m, "k": "b", "v": inst("open")},
                        [{"m": m.set("b", inst("open"))}]),
            "@frame with @at and the fields": ([builtin("frame", "m'", "m", "k"), builtin("at", idx("m'", "k"), state="closed"),
                                                atom("${} == ${} + 1", fld(idx("m'", "k"), "x"), fld(idx("m", "k"), "x"))],
                                               ["m"], {"m": m, "k": "a"}, [{"m": FrozenMap({"a": inst("closed", x=2)})}]),
            "@frame with @notin": ([builtin("frame", "m'", "m", "k"), builtin("notin", "k", "m'")], ["m"], {"m": m, "k": "a"},
                                   [{"m": FrozenMap()}]),
            "a variable nothing constrains, over its type": ([], ["n"], {}, [{"n": 0}, {"n": 1}, {"n": 2}]),
            "a variable only checked, over its type": ([builtin("ne", "n'", "x")], ["n"], {"x": 1}, [{"n": 0}, {"n": 2}]),
            "a choice under ∃": ([exists("T", eq("n'", fixture.bound(0)))], ["n"], {}, [{"n": 1}, {"n": 2}]),
        }
        for name, (post, names, before, want) in tests.items():
            with self.subTest(name):
                # Execute
                got = self.solve(post, names, before)

                # Assert
                self.assertEqual(want, got)

    def test_the_whole_post_is_checked(self):
        tests = {
            "two clauses that disagree": [eq("n'", "x"), atom("$n' == $x + 1")],
            "a post that reads an entry its map does not have": [atom("$n' == ${}", fld(idx("m", "k"), "x"))],
            "an @at a frame's key contradicts": [builtin("frame", "m'", "m", "k"), builtin("notin", "k", "m'"),
                                                  atom("${} == 1", fld(idx("m'", "k"), "x"))],
        }
        for name, post in tests.items():
            with self.subTest(name):
                # Execute
                got = self.solve(post, ["n"] if "m'" not in str(post) else ["m"], {"x": 1, "m": FrozenMap(), "k": "a"})

                # Assert
                self.assertEqual([], got)

    def test_what_it_cannot_enumerate_is_refused(self):
        m = FrozenMap({"a": inst("open", x=1)})
        tests = {
            "a variable nothing determines, of no type": ([], ["q"], {}, "q has none in TYPES"),
            "part of a map, unframed": ([builtin("at", idx("m'", "k"), state="open")], ["m"], {"k": "a"}, "only in part"),
            "a frame's key left open": ([builtin("frame", "m'", "m", "k")], ["m"], {"m": m, "k": "a"}, "leaves m'\\['a'\\] unconstrained"),
            "fields of an entry without its state": ([builtin("frame", "m'", "m", "k"), atom("${} == 1", fld(idx("m'", "k"), "x"))],
                                                     ["m"], {"m": m, "k": "a"}, "but not its state"),
            "a variable of no state after the step": ([eq("z'", "x")], ["n"], {"x": 1}, "z is no variable"),
        }
        for name, (post, names, before, said) in tests.items():
            with self.subTest(name):
                # Execute, Assert
                with self.assertRaisesRegex(PostError, said):
                    self.solve(post, names, before)

    def test_the_environment_decides_a_post_atom_it_declares_over_the_types(self):
        # Setup
        env = environment.over({"$n' は $x 以上": lambda n, x: n >= x}, {}, None)

        # Execute
        got = self.solve([atom("$n' は $x 以上")], ["n"], {"x": 1}, env)

        # Assert
        self.assertEqual([{"n": 1}, {"n": 2}], got)


if __name__ == "__main__":
    unittest.main()
