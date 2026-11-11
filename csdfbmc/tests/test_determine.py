"""determine: the equalities of a guard that bind a field, read in the
guard's order."""
import json
import unittest

from csdfbmc.determine import Bind, Witness, decides_early, determine, draws, part_reads, place, witness
from csdfbmc.model import App, Bound, Builtin, Exists, Field, Free, Index, Lit


def eq(x, y):
    return Builtin("eq", (x, y))


def other(*names):
    """A conjunct that reads names and binds nothing."""
    return Builtin("at", tuple(Free(n) for n in names), state="s")


def part(f, k):
    return Index(Free(f), Lit(json.dumps(k)))


V, W, N = Free("v"), Free("w"), Free("n")


class DetermineTest(unittest.TestCase):
    def test_the_equalities_that_bind_a_field_by_their_places(self):
        tests = {
            "an equality over the state": ([eq(V, N)], {0: Bind("v", None, N)}),
            "either side": ([eq(N, V)], {0: Bind("v", None, N)}),
            "after a conjunct over the state": ([other("n"), eq(V, N)], {1: Bind("v", None, N)}),
            "after a conjunct over a later field": ([other("w"), eq(V, N)], {1: Bind("v", None, N)}),
            "after a conjunct that reads the field": ([other("v"), eq(V, N)], {}),
            "a term over a field read before": ([other("w"), eq(V, W)], {1: Bind("v", None, W)}),
            "a term over the field itself": ([eq(V, Index(V, N))], {}),
            "a term over a value after the step": ([eq(V, Free("n", primed=True))], {}),
            "a term over an unknown name": ([eq(V, Free("z"))], {}),
            "two fields, the later bound to the earlier": ([eq(V, W)], {0: Bind("w", None, V)}),
            "the first equality of a field": ([eq(V, N), eq(V, W)], {0: Bind("v", None, N), 1: Bind("w", None, V)}),
            "a field bound reads as bound": ([eq(V, N), eq(W, V)], {0: Bind("v", None, N), 1: Bind("w", None, V)}),
            "parts": ([eq(part("v", "x"), N), eq(part("v", "y"), N)],
                      {0: Bind("v", "x", N), 1: Bind("v", "y", N)}),
            "parts apart by a conjunct over others": ([eq(part("v", "x"), N), other("n", "w"), eq(part("v", "y"), N)],
                                                      {0: Bind("v", "x", N), 2: Bind("v", "y", N)}),
            "a part after a conjunct that reads the field": ([eq(part("v", "x"), N), other("v"), eq(part("v", "y"), N)],
                                                             {0: Bind("v", "x", N)}),
            "a key bound once": ([eq(part("v", "x"), N), eq(part("v", "x"), N)], {0: Bind("v", "x", N)}),
            "the whole after its parts": ([eq(part("v", "x"), N), eq(V, N)], {0: Bind("v", "x", N)}),
        }
        for name, (guard, want) in tests.items():
            with self.subTest(name):
                # Execute
                got = determine(tuple(guard), ["v", "w"], {"n"}, frozenset())

                # Assert
                self.assertEqual(want, got)

    def test_a_field_ranged_is_bound_by_nothing(self):
        # Execute
        got = determine((eq(V, N),), ["v"], {"n"}, frozenset({"v"}))

        # Assert
        self.assertEqual({}, got)


class PlaceTest(unittest.TestCase):
    def test_each_conjunct_is_a_step_at_the_depth_of_what_it_reads_in_the_guard_s_order(self):
        x, y = Bind("v", "x", N), Bind("v", "y", W)
        tests = {
            "no levels, so the caller's": ([other("n")], {}, [], [(0, 0, None)], 0),
            "the depth of the field read": ([other("n"), other("w")], {}, ["v", "w"], [(0, 0, None), (2, 1, None)], 1),
            "never shallower than the step before": ([other("w"), other("v")], {}, ["v", "w"],
                                                     [(2, 0, None), (2, 1, None)], 0),
            "a binding at the last depth is asked": ([other("w"), eq(V, W), other("v")], {1: Bind("v", None, W)}, ["w"],
                                                     [(1, 0, None), (1, 1, Bind("v", None, W)), (1, 2, None)], 2),
            "a term over a field bound before": ([eq(V, N), eq(W, V)], {0: Bind("v", None, N), 1: Bind("w", None, V)}, [],
                                                 [(0, 0, Bind("v", None, N)), (0, 1, Bind("w", None, V))], 2),
            "the last part reads the parts that range": ([eq(part("v", "x"), N), eq(part("v", "y"), W)], {0: x, 1: y},
                                                         ["w", ("v", "z")], [(0, 0, x), (2, 1, y)], 2),
        }
        for name, (guard, binds, levels, want, inside) in tests.items():
            with self.subTest(name):
                # Execute
                got = place(tuple(guard), binds, levels)

                # Assert
                self.assertEqual((want, inside), got)


class PartReadsTest(unittest.TestCase):
    def test_a_part_with_a_literal_key_is_read_as_the_part_and_anything_else_as_the_whole(self):
        parts = {("r", "a"), ("r", "b")}
        tests = {
            "a field": (Field(Free("r"), "a"), {("r", "a")}),
            "an entry with a literal key": (part("r", "a"), {("r", "a")}),
            "an entry with a key not literal": (Index(Free("r"), Free("x")), {"r", "x"}),
            "a part of a part": (Field(Field(Free("r"), "a"), "c"), {("r", "a")}),
            "a key not among the parts": (Field(Free("r"), "z"), {"r"}),
            "the variable after the step": (Field(Free("r", primed=True), "a"), set()),
            "the whole variable": (Free("r"), {"r"}),
            "a part of another variable": (Field(Free("s"), "a"), {"s"}),
        }
        for name, (t, want) in tests.items():
            with self.subTest(name):
                # Execute / Assert
                self.assertEqual(want, part_reads(t, parts))


class DecidesEarlyTest(unittest.TestCase):
    def test_a_field_s_parts_range_one_by_one_only_when_a_step_reads_one_before_the_last(self):
        levels = ["w", ("r", "a"), ("r", "b")]
        reads_a, reads_b = other_of(Field(Free("r"), "a")), other_of(Field(Free("r"), "b"))
        tests = {
            "a part before the last": ([reads_a], [(2, 0, None)], True),
            "the last part": ([reads_b], [(3, 0, None)], False),
            "a part read after the whole": ([other("r"), reads_a], [(3, 0, None), (3, 1, None)], False),
            "no step reads a part": ([other("w")], [(1, 0, None)], False),
        }
        for name, (guard, steps, want) in tests.items():
            with self.subTest(name):
                # Execute / Assert
                self.assertEqual(want, decides_early(tuple(guard), levels, steps, "r"))


class DrawsTest(unittest.TestCase):
    def test_a_level_a_leading_membership_decides_is_drawn_from_the_map(self):
        M = Free("m")
        k = Field(Free("r"), "k")
        levels = ["v", ("r", "k"), ("r", "a")]
        tests = {
            "a field": ([Builtin("in", (V, M))], {0: M}),
            "a part read as a field": ([Builtin("in", (k, M))], {1: M}),
            "a part read as an entry": ([Builtin("in", (part("r", "k"), M))], {1: M}),
            "after a conjunct over earlier levels": ([other("v"), Builtin("in", (k, M))], {1: M}),
            "the first membership of a level": ([Builtin("in", (V, M)), Builtin("in", (V, N))], {0: M}),
            "a map over a field": ([Builtin("in", (k, Free("v")))], {}),
            "a map after the step": ([Builtin("in", (V, Free("m", primed=True)))], {}),
            "not a level": ([Builtin("in", (N, M))], {}),
            "a part of a part": ([Builtin("in", (Field(k, "z"), M))], {}),
            "a non-membership": ([Builtin("notin", (V, M))], {}),
            "after a conjunct reading the level": ([other_of(k), Builtin("in", (k, M))], {}),
            "after a conjunct reading the whole record": ([other("r"), Builtin("in", (k, M))], {}),
            "after a conjunct reading a later level": ([other_of(Field(Free("r"), "a")), Builtin("in", (k, M))], {}),
        }
        for name, (guard, want) in tests.items():
            with self.subTest(name):
                # Execute / Assert
                self.assertEqual(want, draws(tuple(guard), levels, {"m", "n"}))


class WitnessTest(unittest.TestCase):
    def test_the_levels_an_entry_of_a_map_decides_are_drawn_from_its_entries(self):
        M, J = Free("m"), Bound(0)
        k, a, b = Field(Free("r"), "k"), Field(Free("r"), "a"), Field(Free("r"), "b")
        idk, x = Field(Index(M, J), "id"), Field(Index(M, J), "x")
        levels = ["v", ("r", "k"), ("r", "a"), ("r", "b")]

        def some(*body):
            return Exists("K", (Builtin("in", (J, M)),) + body)
        tests = {
            "parts by entries": ([some(eq(k, idk), eq(a, x))], Witness(0, (1, 2), M, "K", (idk, x))),
            "either side": ([some(eq(idk, k))], Witness(0, (1,), M, "K", (idk,))),
            "a field": ([some(eq(V, idk))], Witness(0, (0,), M, "K", (idk,))),
            "after a conjunct over earlier levels": ([other("v"), some(eq(k, idk))], Witness(1, (1,), M, "K", (idk,))),
            "the levels in the guard's order": ([some(eq(a, x), eq(k, idk))], Witness(0, (2, 1), M, "K", (x, idk))),
            "levels apart": ([some(eq(k, idk), eq(b, x))], None),
            "a level twice": ([some(eq(k, idk), eq(k, x))], None),
            "no membership first": ([Exists("K", (eq(k, idk), Builtin("in", (J, M))))], None),
            "a membership of another bound name": ([Exists("K", (Builtin("in", (Bound(1), M)), eq(k, idk)))], None),
            "a map over a field": ([Exists("K", (Builtin("in", (J, V)), eq(k, idk)))], None),
            "a term over a field": ([some(eq(k, Index(V, J)))], None),
            "a term applying a function": ([some(eq(k, App("f", (J,))))], None),
            "a term after the step": ([some(eq(k, Index(Free("m", primed=True), J)))], None),
            "a conjunct not an equality": ([some(eq(k, idk), other("n"))], None),
            "no equality": ([some()], None),
            "after a conjunct reading a level it binds": ([other_of(k), some(eq(k, idk))], None),
            "after a conjunct reading a later level": ([other_of(b), some(eq(k, idk))], None),
        }
        for name, (guard, want) in tests.items():
            with self.subTest(name):
                # Execute / Assert
                self.assertEqual(want, witness(tuple(guard), levels, {"m", "n"}))


def other_of(*terms):
    """A conjunct that reads terms and binds nothing."""
    return Builtin("at", terms, state="s")


if __name__ == "__main__":
    unittest.main()
