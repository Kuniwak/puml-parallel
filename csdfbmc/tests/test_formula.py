"""A clause's value: a guard's truth value, a post's valuations after the
step (docs/CLAUSE_DESIGN.md, O7)."""
import itertools
import unittest

from csdfbmc import environment
from csdfbmc.formula import Evaluator, PostError, solve
from csdfbmc.meaning import AtomMeaning, apply_given
from csdfbmc.model import formula_of
from csdfbmc.tests import fixture
from csdfbmc.tests.fixture import atom, builtin, eq, exists, fld, idx
from csdfbmc.values import FrozenMap, Rec, same_value

TYPES = {"T": [1, 2], "n": [0, 1, 2], "p": [0, 1], "k": ["a", "b"], "bal": [0, 1]}
# Every map from the keys "a" and "b" to an instance open or closed with a
# balance of 0 or 1, each entry possibly absent.
TYPES["m"] = [FrozenMap({k: v for k, v in zip(("a", "b"), entries) if v is not None})
              for entries in itertools.product([None] + [Rec(s, FrozenMap(bal=b)) for s in ("open", "closed") for b in TYPES["bal"]],
                                               repeat=2)]


def evaluator(env=environment.NONE, given=None):
    return Evaluator(AtomMeaning(fixture.glue(), env, lambda n: TYPES[n]), lambda t: TYPES[t], apply_given(given or {}))


def inst(state, **fields):
    return Rec(state, FrozenMap(fields))


M = FrozenMap({"a": inst("open", bal=1)})


class GuardTest(unittest.TestCase):
    def test_a_clause_holds_or_not(self):
        tests = {
            "eq of equal values": ([eq("x", "y")], {"x": 1, "y": 1}, True),
            "eq tells a truth value from the number it equals": ([eq("x", "y")], {"x": 1, "y": True}, False),
            "ne": ([builtin("ne", "x", "y")], {"x": 1, "y": 2}, True),
            "in": ([builtin("in", "k", "m")], {"k": "a", "m": M}, True),
            "notin": ([builtin("notin", "k", "m")], {"k": "b", "m": M}, True),
            "empty": ([builtin("empty", "m")], {"m": FrozenMap()}, True),
            "at the instance's state": ([builtin("at", idx("m", "k"), state="open")], {"k": "a", "m": M}, True),
            "at another state": ([builtin("at", idx("m", "k"), state="closed")], {"k": "a", "m": M}, False),
            "update": ([builtin("update", "m2", "m", "k", "v")], {"m2": M.set("b", 3), "m": M, "k": "b", "v": 3}, True),
            "frame, the others kept": ([builtin("frame", "m2", "m", "k")], {"m2": M.set("b", 3), "m": M, "k": "b"}, True),
            "frame, another changed": ([builtin("frame", "m2", "m", "k")], {"m2": M.set("b", 3), "m": M, "k": "a"}, False),
            "∃ over the type it names": ([exists("T", eq(fixture.bound(0), "x"))], {"x": 2}, True),
            "∃ with no value that holds": ([exists("T", eq(fixture.bound(0), "x"))], {"x": 3}, False),
            "an atom, through the glue": ([atom("$x > $y")], {"x": 2, "y": 1}, True),
            "an entry's field read by an atom": ([atom("${} > 0", fld(idx("m", "k"), "bal"))], {"m": M, "k": "a"}, True),
            "an atom of an absent entry is false": ([atom("${} > 0", fld(idx("m", "k"), "bal"))], {"m": FrozenMap(), "k": "a"}, False),
            "a builtin of an absent entry is false": ([builtin("at", idx("m", "k"), state="open")], {"m": FrozenMap(), "k": "a"}, False),
            "an absent entry under ∃ is false": ([exists("T", atom("${} > 0", fld(idx("m", "k"), "bal")))], {"m": FrozenMap(), "k": "a"}, False),
            "a glue sees an instance as its fields": ([atom("${} == {'bal': 1}", idx("m", "k"))], {"m": M, "k": "a"}, True),
            "true": ([], {}, True),
        }
        for name, (clause, before, want) in tests.items():
            with self.subTest(name):
                # Execute
                got = evaluator().holds(formula_of(clause), before)

                # Assert
                self.assertEqual(want, got)

    def test_an_environment_function_of_a_term_is_the_one_given(self):
        # Setup
        app = {"kind": "app", "fn": "double", "args": [fixture.term("x")]}
        clause = formula_of([eq(app, "y")])

        # Execute, Assert
        self.assertTrue(evaluator(given={"double": lambda x: 2 * x}).holds(clause, {"x": 2, "y": 4}))
        with self.assertRaisesRegex(ValueError, "no environment function double"):
            evaluator().holds(clause, {"x": 2, "y": 4})


class SolveTest(unittest.TestCase):
    def solve(self, post, names, before, env=environment.NONE):
        return solve(evaluator(env), formula_of(post), tuple(names), before, lambda n: TYPES[n], "the edge")

    def test_the_values_after_the_step_come_from_what_determines_them(self):
        tests = {
            "an equality builtin": ([eq("n'", "x")], ["n"], {"x": 7}, [{"n": 7}]),
            "an equality of a value outside its type (Q5)": ([eq("n", "n'")], ["n"], {"n": 9}, [{"n": 9}]),
            "a post atom of the glue": ([atom("$n' in ($x, $x + 1)")], ["n"], {"x": 1}, [{"n": 1}, {"n": 2}]),
            "@empty": ([builtin("empty", "m'")], ["m"], {}, [{"m": FrozenMap()}]),
            "@update": ([builtin("update", "m'", "m", "k", "v")], ["m"], {"m": M, "k": "b", "v": inst("open")},
                        [{"m": M.set("b", inst("open"))}]),
            "@frame with @at and the fields": ([builtin("frame", "m'", "m", "k"), builtin("at", idx("m'", "k"), state="closed"),
                                                atom("${} == ${} + 1", fld(idx("m'", "k"), "bal"), fld(idx("m", "k"), "bal"))],
                                               ["m"], {"m": M, "k": "a"}, [{"m": FrozenMap({"a": inst("closed", bal=2)})}]),
            "@frame with @notin": ([builtin("frame", "m'", "m", "k"), builtin("notin", "k", "m'")], ["m"], {"m": M, "k": "a"},
                                   [{"m": FrozenMap()}]),
            "@frame with an entry equal to a value": ([builtin("frame", "m'", "m", "k"), eq(idx("m'", "k"), "v")], ["m"],
                                                      {"m": M, "k": "b", "v": inst("open", bal=0)}, [{"m": M.set("b", inst("open", bal=0))}]),
            "an entry equal to a value at the state @at says": ([builtin("frame", "m'", "m", "k"), eq(idx("m'", "k"), "v"),
                                                                builtin("at", idx("m'", "k"), state="open")], ["m"],
                                                               {"m": M, "k": "b", "v": inst("open", bal=0)}, [{"m": M.set("b", inst("open", bal=0))}]),
            "an entry equal to a value at another state": ([builtin("frame", "m'", "m", "k"), eq(idx("m'", "k"), "v"),
                                                            builtin("at", idx("m'", "k"), state="closed")], ["m"],
                                                           {"m": M, "k": "b", "v": inst("open", bal=0)}, []),
            "a field only checked, over its type": ([builtin("frame", "m'", "m", "k"), builtin("at", idx("m'", "k"), state="open"),
                                                     builtin("ne", fld(idx("m'", "k"), "bal"), "x")], ["m"], {"m": M, "k": "b", "x": 0},
                                                    [{"m": M.set("b", inst("open", bal=1))}]),
            "two frames keep what either keeps": ([builtin("frame", "m'", "m", "k"), builtin("frame", "m'", "m", "j"),
                                                   builtin("notin", "k", "m'")], ["m"],
                                                  {"m": M.set("b", inst("open", bal=0)), "k": "a", "j": "b"}, []),
            "a variable nothing constrains, over its type": ([], ["n"], {}, [{"n": 0}, {"n": 1}, {"n": 2}]),
            "a variable only checked, over its type": ([builtin("ne", "n'", "x")], ["n"], {"x": 1}, [{"n": 0}, {"n": 2}]),
            "a choice under ∃": ([exists("T", eq("n'", fixture.bound(0)))], ["n"], {}, [{"n": 1}, {"n": 2}]),
            "two clauses that disagree": ([eq("n'", "x"), atom("$n' == $x + 1")], ["n"], {"x": 1}, []),
            "a post that reads an entry its map does not have": ([atom("$n' == ${}", fld(idx("m", "k"), "bal"))], ["n"],
                                                                {"m": FrozenMap(), "k": "a"}, []),
            "fields of an entry @notin removes": ([builtin("frame", "m'", "m", "k"), builtin("notin", "k", "m'"),
                                                   atom("${} == 1", fld(idx("m'", "k"), "bal"))], ["m"], {"m": M, "k": "a"}, []),
        }
        for name, (post, names, before, want) in tests.items():
            with self.subTest(name):
                # Execute
                got = self.solve(post, names, before)

                # Assert
                self.assertEqual(want, got)

    def test_what_it_cannot_enumerate_is_refused(self):
        tests = {
            "a variable nothing determines, of no type": ([], ["q"], {}, "q has none in TYPES"),
            "a field nothing determines, of no type": ([builtin("frame", "m'", "m", "k"), builtin("at", idx("m'", "k"), state="open"),
                                                        builtin("ne", fld(idx("m'", "k"), "fee"), "x")], ["m"], {"m": M, "k": "b", "x": 0},
                                                       "m'\\['b'\\].fee, .* fee has none in TYPES"),
            "part of a map, unframed": ([builtin("at", idx("m'", "k"), state="open")], ["m"], {"k": "a"}, "only in part"),
            "a frame's key left open": ([builtin("frame", "m'", "m", "k")], ["m"], {"m": M, "k": "a"}, "leaves m'\\['a'\\] unconstrained"),
            "fields of an entry without its state": ([builtin("frame", "m'", "m", "k"), atom("${} == 1", fld(idx("m'", "k"), "bal"))],
                                                     ["m"], {"m": M, "k": "a"}, "but not its state"),
            "a variable of no state after the step": ([eq("z'", "x")], ["n"], {"x": 1}, "z is no variable"),
        }
        for name, (post, names, before, said) in tests.items():
            with self.subTest(name):
                # Execute, Assert
                with self.assertRaisesRegex(PostError, said):
                    self.solve(post, names, before)

    def test_the_environment_decides_a_post_atom_it_declares_over_the_types(self):
        # Setup
        env = environment.over({"$_0 は $_1 以上": lambda n, x: n >= x}, {}, None)

        # Execute
        got = self.solve([atom("$n' は $x 以上")], ["n"], {"x": 1}, env)

        # Assert
        self.assertEqual([{"n": 1}, {"n": 2}], got)


class SolveIsTheMeaningTest(unittest.TestCase):
    """What solve finds is what checking the whole post against every
    valuation over TYPES finds, where TYPES holds every value it allows."""

    def test_solve_finds_the_valuations_the_post_holds_of(self):
        tests = {
            "an equality": ([eq("n'", "x")], {"x": 1}),
            "an equality and a check": ([eq("n'", "x"), builtin("ne", "p'", "x")], {"x": 1}),
            "a post atom": ([atom("$n' in ($x, $x + 1)")], {"x": 1}),
            "a post atom and another variable free": ([atom("$n' == $x")], {"x": 0}),
            "∃ choosing two variables alike": ([exists("T", eq("n'", fixture.bound(0)), eq("p'", "x"))], {"x": 1}),
            "a check of both": ([builtin("ne", "n'", "p'")], {}),
            "nothing": ([], {}),
            "contradicting clauses": ([eq("n'", "x"), builtin("ne", "n'", "x")], {"x": 2}),
        }
        names = ("n", "p")
        ev = evaluator()
        for name, (post, before) in tests.items():
            with self.subTest(name):
                # Setup
                clause = formula_of(post)
                want = [dict(zip(names, vs)) for vs in itertools.product(*(TYPES[n] for n in names))
                        if ev.holds(clause, before, dict(zip(names, vs)))]

                # Execute
                got = solve(ev, clause, names, before, lambda n: TYPES[n], "the edge")

                # Assert
                self.assertTrue(_same(want, got), f"want {want}, got {got}")


    def test_solve_finds_the_maps_the_post_holds_of(self):
        # Setup
        a0, a1, b0 = inst("open", bal=0), inst("open", bal=1), inst("closed", bal=0)
        tests = {
            "@update": ([builtin("update", "m'", "m", "k", "v")], {"m": FrozenMap({"a": a0}), "k": "b", "v": b0}),
            "@frame with an entry equal to a value": ([builtin("frame", "m'", "m", "k"), eq(idx("m'", "k"), "v")],
                                                      {"m": FrozenMap({"a": a0}), "k": "b", "v": b0}),
            "@frame with @at and a field fixed": ([builtin("frame", "m'", "m", "k"), builtin("at", idx("m'", "k"), state="closed"),
                                                   eq(fld(idx("m'", "k"), "bal"), "x")], {"m": FrozenMap({"a": a0}), "k": "a", "x": 1}),
            "@frame with @at and a field over TYPES": ([builtin("frame", "m'", "m", "k"), builtin("at", idx("m'", "k"), state="open"),
                                                        builtin("ne", fld(idx("m'", "k"), "bal"), "x")],
                                                       {"m": FrozenMap({"b": b0}), "k": "a", "x": 0}),
            "@frame with @notin": ([builtin("frame", "m'", "m", "k"), builtin("notin", "k", "m'")],
                                   {"m": FrozenMap({"a": a0, "b": b0}), "k": "a"}),
            "@notin of a key no frame opens": ([builtin("frame", "m'", "m", "k"), builtin("notin", "k", "m'"),
                                                builtin("notin", "j", "m'")],
                                               {"m": FrozenMap({"a": a0, "b": b0}), "k": "a", "j": "b"}),
            "two frames of one base": ([builtin("frame", "m'", "m", "k"), builtin("frame", "m'", "m", "k", "j"),
                                        builtin("notin", "k", "m'")], {"m": FrozenMap({"a": a0, "b": b0}), "k": "a", "j": "b"}),
            "two frames each keeping what the other opens": ([builtin("frame", "m'", "m", "k"), builtin("frame", "m'", "w", "j")],
                                                             {"m": FrozenMap({"a": a0, "b": b0}), "w": FrozenMap({"a": a1, "b": a0}),
                                                              "k": "a", "j": "b"}),
            "two frames whose bases disagree on what both keep": ([builtin("frame", "m'", "m", "k"), builtin("frame", "m'", "w", "k"),
                                                                   builtin("notin", "k", "m'")], {"m": FrozenMap({"b": b0}), "w": FrozenMap({"b": a0}), "k": "a"}),
            "two frames whose bases agree on what both keep": ([builtin("frame", "m'", "m", "k"), builtin("frame", "m'", "w", "k"),
                                                                builtin("notin", "k", "m'")],
                                                               {"m": FrozenMap({"a": a0, "b": b0}), "w": FrozenMap({"b": b0}), "k": "a"}),
            "a map nothing constrains": ([], {}),
        }
        ev = evaluator()
        for name, (post, before) in tests.items():
            with self.subTest(name):
                # Setup
                clause = formula_of(post)
                want = [{"m": m} for m in TYPES["m"] if ev.holds(clause, before, {"m": m})]

                # Execute
                got = solve(ev, clause, ("m",), before, lambda n: TYPES[n], "the edge")

                # Assert
                self.assertTrue(_same(want, got), f"want {want}, got {got}")


def _same(a: list[dict], b: list[dict]) -> bool:
    return len(a) == len(b) and all(any(all(same_value(x[n], y[n]) for n in x) for y in b) for x in a)


if __name__ == "__main__":
    unittest.main()
