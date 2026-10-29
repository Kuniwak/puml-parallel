"""A clause's value: a guard's truth value, a post's valuations after the
step (docs/CLAUSE_DESIGN.md, O7)."""
import itertools
import unittest

from csdfbmc import environment
from csdfbmc.checks import Checks
from csdfbmc.formula import Evaluator, PostError, solve
from csdfbmc.meaning import AtomMeaning, apply_given
from csdfbmc.model import Family, formula_of
from csdfbmc.tests import fixture
from csdfbmc.tests.fixture import atom, builtin, eq, exists, fld, idx, lit
from csdfbmc.values import FrozenMap, Rec, same_value

TYPES = {"T": [1, 2], "n": [0, 1, 2], "p": [0, 1], "k": ["a", "b"], "cnt": [0, 1]}
# Every map from the keys "a" and "b" to an instance open or closed with a
# count of 0 or 1, each entry possibly absent.
TYPES["m"] = [FrozenMap({k: v for k, v in zip(("a", "b"), entries) if v is not None})
              for entries in itertools.product([None] + [Rec(s, FrozenMap(cnt=b)) for s in ("open", "closed") for b in TYPES["cnt"]],
                                               repeat=2)]
# Every map from "a" and "b" to an instance open with a count and a p, or
# shut with no field.
TYPES["w"] = [FrozenMap({k: v for k, v in zip(("a", "b"), entries) if v is not None})
              for entries in itertools.product([None, Rec("shut", FrozenMap())] +
                                               [Rec("open", FrozenMap(cnt=b, p=q)) for b in TYPES["cnt"] for q in TYPES["p"]], repeat=2)]
# The families of the maps: what an instance in each may be. An instance of
# m is dear only in a refusal, tag having no type.
FAMILIES = {"m": Family("none", ("none", "open", "closed", "dear"), FrozenMap({"none": (), "open": ("cnt",), "closed": ("cnt",), "dear": ("tag",)}), "k",
                        "k", FrozenMap({"cnt": "cnt", "tag": "tag"})),
            "w": Family("none", ("none", "open", "shut"), FrozenMap({"none": (), "open": ("cnt", "p"), "shut": ()}), "k",
                        "k", FrozenMap({"cnt": "cnt", "p": "p"}))}
CARRIER = Checks(types=TYPES).carrier


class ByName(dict):
    """The types of the variables of the tests: each of the type named like
    it."""

    def __missing__(self, name):
        return name


def evaluator(env=environment.NONE, given=None):
    return Evaluator(AtomMeaning(fixture.glue(), env, CARRIER), CARRIER, apply_given(given or {}))


def inst(state, **fields):
    return Rec(state, FrozenMap(fields))


M = FrozenMap({"a": inst("open", cnt=1)})


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
            "frame, a key of another kind changed": ([builtin("frame", "m2", "m", "k")],
                                                     {"m2": FrozenMap([(1, 0), (True, 5)]), "m": FrozenMap([(1, 0), (True, 0)]), "k": 1}, False),
            "notin tells a key from one of another kind": ([builtin("notin", "k", "m")], {"k": True, "m": FrozenMap({1: 0})}, True),
            "∃ over the type it names": ([exists("T", eq(fixture.bound(0), "x"))], {"x": 2}, True),
            "∃ with no value that holds": ([exists("T", eq(fixture.bound(0), "x"))], {"x": 3}, False),
            "an atom, through the glue": ([atom("$x > $y")], {"x": 2, "y": 1}, True),
            "an entry's field read by an atom": ([atom("${} > 0", fld(idx("m", "k"), "cnt"))], {"m": M, "k": "a"}, True),
            "an atom of an absent entry is false": ([atom("${} > 0", fld(idx("m", "k"), "cnt"))], {"m": FrozenMap(), "k": "a"}, False),
            "a builtin of an absent entry is false": ([builtin("at", idx("m", "k"), state="open")], {"m": FrozenMap(), "k": "a"}, False),
            "an absent entry under ∃ is false": ([exists("T", atom("${} > 0", fld(idx("m", "k"), "cnt")))], {"m": FrozenMap(), "k": "a"}, False),
            "a glue sees an instance as its fields": ([atom("${} == {'cnt': 1}", idx("m", "k"))], {"m": M, "k": "a"}, True),
            "true": ([], {}, True),
            "a literal integer": ([eq("x", lit(0))], {"x": 0}, True),
            "a literal integer is not the float": ([eq("x", lit(1.0))], {"x": 1}, False),
            "a literal float is not the integer": ([eq("x", lit(1))], {"x": 1.0}, False),
            "a literal 0.0 is 0.0": ([eq("x", lit(0.0))], {"x": 0.0}, True),
            "a literal truth value is not the number it equals": ([eq("x", lit(True))], {"x": 1}, False),
            "a literal string is not the number it spells": ([builtin("ne", "x", lit("1"))], {"x": 1}, True),
            "a literal array is a tuple": ([eq("x", lit([1, "a"]))], {"x": (1, "a")}, True),
            "a literal object is a map": ([eq("x", lit({"b": 2, "a": 1}))], {"x": FrozenMap(a=1, b=2)}, True),
            "a literal key": ([builtin("in", lit("a"), "m")], {"m": M}, True),
            "a literal in an atom": ([atom("${} > $x", lit(2))], {"x": 1}, True),
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
        return solve(evaluator(env), formula_of(post), tuple(names), before, ByName(), "the edge", FAMILIES)

    def test_the_values_after_the_step_come_from_what_determines_them(self):
        tests = {
            "an equality builtin": ([eq("n'", "x")], ["n"], {"x": 7}, [{"n": 7}]),
            "an equality of a value outside its type (Q5)": ([eq("n", "n'")], ["n"], {"n": 9}, [{"n": 9}]),
            "a post atom of the glue": ([atom("$n' in ($x, $x + 1)")], ["n"], {"x": 1}, [{"n": 1}, {"n": 2}]),
            "@empty": ([builtin("empty", "m'")], ["m"], {}, [{"m": FrozenMap()}]),
            "@update": ([builtin("update", "m'", "m", "k", "v")], ["m"], {"m": M, "k": "b", "v": inst("open", cnt=0)},
                        [{"m": M.set("b", inst("open", cnt=0))}]),
            "@eq of the map": ([eq("m'", "v")], ["m"], {"v": M}, [{"m": M}]),
            "@frame with @at and the fields": ([builtin("frame", "m'", "m", "k"), builtin("at", idx("m'", "k"), state="closed"),
                                                atom("${} == ${} + 1", fld(idx("m'", "k"), "cnt"), fld(idx("m", "k"), "cnt"))],
                                               ["m"], {"m": M, "k": "a"}, [{"m": FrozenMap({"a": inst("closed", cnt=2)})}]),
            "@frame with @notin": ([builtin("frame", "m'", "m", "k"), builtin("notin", "k", "m'")], ["m"], {"m": M, "k": "a"},
                                   [{"m": FrozenMap()}]),
            "@frame with an entry equal to a value": ([builtin("frame", "m'", "m", "k"), eq(idx("m'", "k"), "v")], ["m"],
                                                      {"m": M, "k": "b", "v": inst("open", cnt=0)}, [{"m": M.set("b", inst("open", cnt=0))}]),
            "an entry equal to a value at the state @at says": ([builtin("frame", "m'", "m", "k"), eq(idx("m'", "k"), "v"),
                                                                builtin("at", idx("m'", "k"), state="open")], ["m"],
                                                               {"m": M, "k": "b", "v": inst("open", cnt=0)}, [{"m": M.set("b", inst("open", cnt=0))}]),
            "an entry equal to a value at another state": ([builtin("frame", "m'", "m", "k"), eq(idx("m'", "k"), "v"),
                                                            builtin("at", idx("m'", "k"), state="closed")], ["m"],
                                                           {"m": M, "k": "b", "v": inst("open", cnt=0)}, []),
            "a field only checked, over its type": ([builtin("frame", "m'", "m", "k"), builtin("at", idx("m'", "k"), state="open"),
                                                     builtin("ne", fld(idx("m'", "k"), "cnt"), "x")], ["m"], {"m": M, "k": "b", "x": 0},
                                                    [{"m": M.set("b", inst("open", cnt=1))}]),
            "two frames keep what either keeps": ([builtin("frame", "m'", "m", "k"), builtin("frame", "m'", "m", "j"),
                                                   builtin("notin", "k", "m'")], ["m"],
                                                  {"m": M.set("b", inst("open", cnt=0)), "k": "a", "j": "b"}, []),
            "a variable nothing constrains, over its type": ([], ["n"], {}, [{"n": 0}, {"n": 1}, {"n": 2}]),
            "a variable only checked, over its type": ([builtin("ne", "n'", "x")], ["n"], {"x": 1}, [{"n": 0}, {"n": 2}]),
            "a choice under ∃": ([exists("T", eq("n'", fixture.bound(0)))], ["n"], {}, [{"n": 1}, {"n": 2}]),
            "one valuation two choices under ∃ give": ([exists("T", eq("n'", lit(0)))], ["n"], {}, [{"n": 0}]),
            "two clauses that disagree": ([eq("n'", "x"), atom("$n' == $x + 1")], ["n"], {"x": 1}, []),
            "a post that reads an entry its map does not have": ([atom("$n' == ${}", fld(idx("m", "k"), "cnt"))], ["n"],
                                                                {"m": FrozenMap(), "k": "a"}, []),
            "fields of an entry @notin removes": ([builtin("frame", "m'", "m", "k"), builtin("notin", "k", "m'"),
                                                   atom("${} == 1", fld(idx("m'", "k"), "cnt"))], ["m"], {"m": M, "k": "a"}, []),
        }
        for name, (post, names, before, want) in tests.items():
            with self.subTest(name):
                # Execute
                got = self.solve(post, names, before)

                # Assert
                self.assertEqual(want, got)

    def test_what_it_cannot_enumerate_is_refused(self):
        tests = {
            "a variable nothing determines, of no type": ([], ["q"], {}, "q', so it ranges over its type q: no type 'q' in TYPES"),
            "a field nothing determines, of no type": ([builtin("frame", "m'", "m", "k"), builtin("at", idx("m'", "k"), state="dear"),
                                                        builtin("ne", fld(idx("m'", "k"), "tag"), "x")], ["m"], {"m": M, "k": "b", "x": 0},
                                                       "m'\\['b'\\].tag, so it ranges over its type tag: no type 'tag' in TYPES"),
            "a field the post does not mention, of no type": ([builtin("frame", "m'", "m", "k"), builtin("at", idx("m'", "k"), state="dear")],
                                                              ["m"], {"m": M, "k": "b"}, "m'\\['b'\\].tag, so it ranges over its type tag: no type 'tag' in TYPES"),
            "a field not of the state": ([builtin("frame", "m'", "m", "k"), builtin("at", idx("m'", "k"), state="closed"),
                                          eq(fld(idx("m'", "k"), "tag"), "x")], ["m"], {"m": M, "k": "b", "x": 0},
                                         "m'\\['b'\\].tag, but an instance at closed holds only cnt"),
            "an instance of a map of no family": ([builtin("frame", "q'", "q", "k"), builtin("at", idx("q'", "k"), state="open")],
                                                  ["q"], {"q": FrozenMap(), "k": "b"}, "q is the map of no family"),
            "part of a map, unframed": ([builtin("at", idx("m'", "k"), state="open")], ["m"], {"k": "a"}, "only in part"),
            "a frame's key left open": ([builtin("frame", "m'", "m", "k")], ["m"], {"m": M, "k": "a"}, "leaves m'\\['a'\\] unconstrained"),
            "fields of an entry without its state": ([builtin("frame", "m'", "m", "k"), atom("${} == 1", fld(idx("m'", "k"), "cnt"))],
                                                     ["m"], {"m": M, "k": "a"}, "but not its state"),
            "a variable of no state after the step": ([eq("z'", "x")], ["n"], {"x": 1}, "z is no variable"),
            "an entry equal to an instance without a field of its state": (
                [builtin("frame", "m'", "m", "k"), eq(idx("m'", "k"), "v")], ["m"], {"m": M, "k": "b", "v": inst("open")},
                "m' the entry 'b' = .*, which no instance of m is: at open it holds cnt, not no field"),
            "an entry equal to an instance with a field not of its state": (
                [builtin("frame", "m'", "m", "k"), eq(idx("m'", "k"), "v")], ["m"], {"m": M, "k": "b", "v": inst("open", cnt=0, tag=1)},
                "which no instance of m is: at open it holds cnt, not cnt, tag"),
            "an entry equal to an instance at the start state": (
                [builtin("frame", "m'", "m", "k"), eq(idx("m'", "k"), "v")], ["m"], {"m": M, "k": "b", "v": inst("none")},
                "which no instance of m is: none is the start state, which is absence"),
            "an entry equal to an instance at no state of the family": (
                [builtin("frame", "m'", "m", "k"), eq(idx("m'", "k"), "v")], ["m"], {"m": M, "k": "b", "v": inst("shut")},
                "which no instance of m is: shut is no state of its family"),
            "an entry equal to a value that is no instance": (
                [builtin("frame", "m'", "m", "k"), eq(idx("m'", "k"), "v")], ["m"], {"m": M, "k": "b", "v": 1},
                "which no instance of m is: it is no instance at all"),
            "the map equal to one holding what no instance is": (
                [eq("m'", "v")], ["m"], {"v": FrozenMap({"a": inst("closed")})}, "m' the entry 'a' = .*, which no instance of m is"),
            "the map equal to a value that is no map": ([eq("m'", "v")], ["m"], {"v": 1}, "m' the value 1, which is no map"),
            "@update with what no instance is": ([builtin("update", "m'", "m", "k", "v")], ["m"], {"m": M, "k": "b", "v": inst("open")},
                                                 "m' the entry 'b' = .*, which no instance of m is"),
        }
        for name, (post, names, before, said) in tests.items():
            with self.subTest(name):
                # Execute, Assert
                with self.assertRaisesRegex(PostError, said):
                    self.solve(post, names, before)

    def test_the_environment_decides_a_post_atom_it_declares_over_the_types(self):
        # Setup
        env = environment.over({"$_0 は $_1 以上": lambda n, x: n >= x}, {})

        # Execute
        got = self.solve([atom("$n' は $x 以上")], ["n"], {"x": 1}, env)

        # Assert
        self.assertEqual([{"n": 1}, {"n": 2}], got)

    def test_the_environment_does_not_decide_an_entry_after_the_step(self):
        # Setup
        env = environment.over({"$_0 は $_1 以上": lambda e, x: True}, {})
        post = [builtin("frame", "m'", "m", "k"), atom("${} は $x 以上", idx("m'", "k"))]

        # Execute, Assert
        with self.assertRaisesRegex(PostError, "the edge: .*m'\\['a'\\], an entry, .*no type"):
            self.solve(post, ["m"], {"m": M, "k": "a", "x": 1}, env)


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
                got = solve(ev, clause, names, before, ByName(), "the edge", FAMILIES)

                # Assert
                self.assertTrue(_same(want, got), f"want {want}, got {got}")


    def test_solve_finds_the_maps_the_post_holds_of(self):
        # Setup
        a0, a1, b0 = inst("open", cnt=0), inst("open", cnt=1), inst("closed", cnt=0)
        tests = {
            "@update": ([builtin("update", "m'", "m", "k", "v")], {"m": FrozenMap({"a": a0}), "k": "b", "v": b0}),
            "@update of an entry there": ([builtin("update", "m'", "m", "k", "v")], {"m": FrozenMap({"a": a0}), "k": "a", "v": b0}),
            "@eq of the map": ([eq("m'", "v")], {"v": FrozenMap({"a": a1, "b": b0})}),
            "@eq of the map to the one before": ([eq("m'", "m")], {"m": FrozenMap({"b": b0})}),
            "@frame with an entry equal to another entry": ([builtin("frame", "m'", "m", "k"), eq(idx("m'", "k"), idx("m", "j"))],
                                                            {"m": FrozenMap({"a": a1}), "k": "b", "j": "a"}),
            "@frame with an entry equal to a value": ([builtin("frame", "m'", "m", "k"), eq(idx("m'", "k"), "v")],
                                                      {"m": FrozenMap({"a": a0}), "k": "b", "v": b0}),
            "@frame with @at and a field fixed": ([builtin("frame", "m'", "m", "k"), builtin("at", idx("m'", "k"), state="closed"),
                                                   eq(fld(idx("m'", "k"), "cnt"), "x")], {"m": FrozenMap({"a": a0}), "k": "a", "x": 1}),
            "@frame with @at and a field over TYPES": ([builtin("frame", "m'", "m", "k"), builtin("at", idx("m'", "k"), state="open"),
                                                        builtin("ne", fld(idx("m'", "k"), "cnt"), "x")],
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
            "@frame with @at alone, its field over TYPES": ([builtin("frame", "m'", "m", "k"), builtin("at", idx("m'", "k"), state="open")],
                                                            {"m": FrozenMap({"a": a0}), "k": "b"}),
            "@frame with @at of an instance already there": ([builtin("frame", "m'", "m", "k"), builtin("at", idx("m'", "k"), state="closed")],
                                                             {"m": FrozenMap({"a": a1, "b": b0}), "k": "a"}),
            "a map nothing constrains": ([], {}),
        }
        ev = evaluator()
        for name, (post, before) in tests.items():
            with self.subTest(name):
                # Setup
                clause = formula_of(post)
                want = [{"m": m} for m in TYPES["m"] if ev.holds(clause, before, {"m": m})]

                # Execute
                got = solve(ev, clause, ("m",), before, ByName(), "the edge", FAMILIES)

                # Assert
                self.assertTrue(_same(want, got), f"want {want}, got {got}")

    def test_solve_finds_an_instance_with_exactly_the_fields_of_its_state(self):
        # Setup
        open_, shut = Rec("open", FrozenMap(cnt=1, p=0)), Rec("shut", FrozenMap())
        tests = {
            "@at alone, every field over TYPES": ([builtin("frame", "w'", "w", "k"), builtin("at", idx("w'", "k"), state="open")],
                                                  {"w": FrozenMap({"a": shut}), "k": "b"}),
            "@at with one field given, the other over TYPES": ([builtin("frame", "w'", "w", "k"), builtin("at", idx("w'", "k"), state="open"),
                                                                eq(fld(idx("w'", "k"), "cnt"), "x")], {"w": FrozenMap(), "k": "a", "x": 1}),
            "@at of a state with no field": ([builtin("frame", "w'", "w", "k"), builtin("at", idx("w'", "k"), state="shut")],
                                             {"w": FrozenMap({"a": open_}), "k": "a"}),
            "@at under ∃ of the key": ([exists("k", builtin("frame", "w'", "w", fixture.bound(0)),
                                               builtin("at", idx("w'", fixture.bound(0)), state="shut"))], {"w": FrozenMap({"a": open_})}),
        }
        ev = evaluator()
        for name, (post, before) in tests.items():
            with self.subTest(name):
                # Setup
                clause = formula_of(post)
                want = [{"w": w} for w in TYPES["w"] if ev.holds(clause, before, {"w": w})]

                # Execute
                got = solve(ev, clause, ("w",), before, ByName(), "the edge", FAMILIES)

                # Assert
                self.assertTrue(_same(want, got), f"want {want}, got {got}")


def _same(a: list[dict], b: list[dict]) -> bool:
    return len(a) == len(b) and all(any(all(same_value(x[n], y[n]) for n in x) for y in b) for x in a)


if __name__ == "__main__":
    unittest.main()
