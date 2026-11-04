"""determine: the equalities of a guard that bind a field, read in the
guard's order."""
import json
import unittest

from csdfbmc.determine import Bind, determine, place
from csdfbmc.model import Builtin, Free, Index, Lit


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
                got = determine(tuple(guard), ["v", "w"], {"n"})

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


if __name__ == "__main__":
    unittest.main()
