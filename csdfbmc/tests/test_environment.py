"""The environment: the pure functions the atoms the checks module declares
denote, given by the checks module or the command line, or chosen from a
seed."""
import unittest

from csdfbmc import environment
from csdfbmc.atoms import function_of
from csdfbmc.environment import given_over, nothing, over, recording, seeded
from csdfbmc.values import FrozenMap


class GivenTest(unittest.TestCase):
    def test_a_given_function_decides_its_name(self):
        # Setup
        cases = [
            ("a value is the function that is always it", {"$在庫 が 十分": False}, "$_0 が 十分", [(n,) for n in range(3)], [False, False, False]),
            ("a function takes the arguments in order", {"$在庫 が $額 以上": lambda 在庫, 額: 在庫 >= 額}, "$_0 が $_1 以上",
             [(n, 1) for n in range(3)], [False, True, True]),
            ("a given value wins over the seed", {"$在庫 が 十分": True}, "$_0 が 十分", [(n,) for n in range(16)], [True] * 16),
            ("any atom of the function names it", {"$残り が 十分": False}, function_of("$在庫 が 十分").name, [(1,)], [False]),
        ]
        for name, given, asked, args, want in cases:
            with self.subTest(name):
                decide = given_over(given, seeded(3))

                # Execute
                got = [decide(asked, a) for a in args]

                # Assert
                self.assertEqual(want, got)

    def test_a_given_function_must_give_a_truth_value(self):
        # Setup
        decide = given_over({"p": lambda n: n}, nothing)

        # Execute, Assert
        with self.assertRaisesRegex(ValueError, "'p'"):
            decide("p", (1,))

    def test_nothing_decides_no_name(self):
        # Execute
        value = given_over({}, nothing)("$_0 が 十分", ())

        # Assert
        self.assertIsNone(value)


class SeededTest(unittest.TestCase):
    def test_a_seed_chooses_a_pure_function_of_the_arguments(self):
        # Setup
        args = [(n, FrozenMap({"k": frozenset({n, "x"})})) for n in range(32)]

        # Execute
        first = [seeded(7)("p", x) for x in args]
        again = [seeded(7)("p", x) for x in args]

        # Assert
        self.assertEqual(first, again)
        self.assertEqual({True, False}, set(first))

    def test_different_seeds_or_names_choose_different_functions(self):
        # Setup
        args = [(n,) for n in range(32)]
        cases = [("seeds", [(1, "p"), (2, "p")]), ("names", [(1, "p"), (1, "q")])]
        for name, pairs in cases:
            with self.subTest(name):
                # Execute
                a, b = ([seeded(s)(n, x) for x in args] for s, n in pairs)

                # Assert
                self.assertNotEqual(a, b)


class CanonicalTest(unittest.TestCase):
    """The text a seed hashes, and so what it chooses, never changes."""

    def test_the_encoding_is_pinned(self):
        # Setup
        cases = [
            ("a mapping, its entries sorted", {"n": 1, "b": True}, "{str:'b':bool:True,str:'n':int:1}"),
            ("a set, its elements sorted", frozenset({2, 1}), "#{int:1,int:2}"),
            ("a tuple and a list alike", ((1, 2), [1, 2]), "((int:1,int:2),(int:1,int:2))"),
            ("a FrozenMap as a mapping", FrozenMap({"k": 1}), "{str:'k':int:1}"),
            ("a string by its repr", "口座", "str:'口座'"),
        ]
        for name, value, want in cases:
            with self.subTest(name):
                # Execute
                got = environment.canonical(value)

                # Assert
                self.assertEqual(want, got)

    def test_a_seed_chooses_what_it_always_chose(self):
        # Execute
        got = [seeded(7)("p", (n,)) for n in range(8)]

        # Assert
        self.assertEqual([False, False, True, True, True, True, True, True], got)


class RecordingTest(unittest.TestCase):
    def test_records_each_name_decided_once_in_order(self):
        # Setup
        seen = {}
        decide = recording(given_over({"q": True, "p": False}, nothing), seen)

        # Execute
        for name in ("q", "r", "p", "q"):
            decide(name, ())

        # Assert
        self.assertEqual(["q", "p"], list(seen))


class OverTest(unittest.TestCase):
    def test_the_command_line_wins_over_the_module_and_the_seed_decides_the_rest(self):
        # Setup
        env = over({"p": True, "q": True, "r": None}, {"q": False}, 5)

        # Execute
        got = [env.decide(n, ()) for n in ("p", "q", "r")]

        # Assert
        self.assertEqual([True, False, seeded(5)("r", ())], got)

    def test_the_module_declares_the_functions_by_name_whatever_atom_names_them(self):
        # Execute
        env = over({"$x が凍結されていない": None, "q": True}, {}, None)

        # Assert
        self.assertEqual(frozenset({"$_0 が凍結されていない", "q"}), env.declared)

    def test_a_declared_function_nothing_gives_is_undecided_without_a_seed(self):
        # Execute
        value = over({"r": None}, {}, None).decide("r", ())

        # Assert
        self.assertIsNone(value)

    def test_the_command_line_may_give_only_a_declared_function(self):
        with self.assertRaisesRegex(ValueError, "--env names 'p', which the checks module's ENVIRONMENT does not declare"):
            over({"q": None}, {"p": True}, None)

    def test_the_report_names_the_seed_and_the_given_names(self):
        # Setup
        env = over({"q": True, "p": None, "r": None}, {"p": False}, 5)

        # Execute
        report = env.to_json()

        # Assert
        self.assertEqual({"seed": 5, "given": ["p", "q"]}, report)

    def test_none_declares_nothing_and_reports_nothing(self):
        # Execute
        declared, report = environment.NONE.declared, environment.NONE.to_json()

        # Assert
        self.assertEqual(frozenset(), declared)
        self.assertEqual({"seed": None, "given": []}, report)

if __name__ == "__main__":
    unittest.main()
