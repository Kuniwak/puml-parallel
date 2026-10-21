"""The environment: the pure functions a clause not formalised denotes, given
by the checks module or the command line, or chosen from a seed."""
import unittest

from csdfbmc import environment
from csdfbmc.environment import given_over, nothing, over, recording, seeded
from csdfbmc.values import FrozenMap


class GivenTest(unittest.TestCase):
    def test_a_given_function_decides_its_name(self):
        # Setup
        cases = [
            ("a value is the function that is always it", {"在庫 が 十分": False}, [{"在庫": n} for n in range(3)], [False, False, False]),
            ("a function reads the names its signature asks for", {"在庫 が 十分": lambda 在庫: 在庫 > 1},
             [{"在庫": n, "額": 9} for n in range(3)], [False, False, True]),
            ("a given value wins over the seed", {"在庫 が 十分": True}, [{"n": n} for n in range(16)], [True] * 16),
        ]
        for name, given, args, want in cases:
            with self.subTest(name):
                decide = given_over(given, seeded(3))

                # Execute
                got = [decide("在庫 が 十分", a) for a in args]

                # Assert
                self.assertEqual(want, got)

    def test_a_given_function_must_give_a_truth_value(self):
        # Setup
        decide = given_over({"p": lambda n: n}, nothing)

        # Execute, Assert
        with self.assertRaisesRegex(TypeError, "'p'"):
            decide("p", {"n": 1})

    def test_nothing_decides_no_name(self):
        # Execute
        value = given_over({}, nothing)("在庫 が 十分", {})

        # Assert
        self.assertIsNone(value)


class SeededTest(unittest.TestCase):
    def test_a_seed_chooses_a_pure_function_of_the_arguments(self):
        # Setup
        args = [{"n": n, "m": FrozenMap({"k": frozenset({n, "x"})})} for n in range(32)]

        # Execute
        first = [seeded(7)("p", x) for x in args]
        again = [seeded(7)("p", x) for x in args]

        # Assert
        self.assertEqual(first, again)
        self.assertEqual({True, False}, set(first))

    def test_different_seeds_or_names_choose_different_functions(self):
        # Setup
        args = [{"n": n} for n in range(32)]
        cases = [("seeds", [(1, "p"), (2, "p")]), ("names", [(1, "p"), (1, "q")])]
        for name, pairs in cases:
            with self.subTest(name):
                # Execute
                a, b = ([seeded(s)(n, x) for x in args] for s, n in pairs)

                # Assert
                self.assertNotEqual(a, b)


class RecordingTest(unittest.TestCase):
    def test_records_each_name_decided_once_in_order(self):
        # Setup
        seen = {}
        decide = recording(given_over({"q": True, "p": False}, nothing), seen)

        # Execute
        for name in ("q", "r", "p", "q"):
            decide(name, {})

        # Assert
        self.assertEqual(["q", "p"], list(seen))


class OverTest(unittest.TestCase):
    def test_the_command_line_wins_over_the_module_and_the_seed_decides_the_rest(self):
        # Setup
        env = over({"p": True, "q": True}, {"q": False}, 5)

        # Execute
        got = [env.decide(n, {}) for n in ("p", "q")]
        rest = env.decide("r", {})

        # Assert
        self.assertEqual([True, False], got)
        self.assertEqual(seeded(5)("r", {}), rest)

    def test_without_a_seed_the_rest_is_undecided(self):
        # Execute
        value = over({}, {}, None).decide("r", {})

        # Assert
        self.assertIsNone(value)

    def test_the_report_names_the_seed_and_the_given_names(self):
        # Setup
        env = over({"q": True}, {"p": False}, 5)

        # Execute
        report = env.to_json()

        # Assert
        self.assertEqual({"seed": 5, "given": ["p", "q"]}, report)

    def test_none_decides_nothing_and_reports_nothing(self):
        # Execute
        value, report = environment.NONE.decide("p", {}), environment.NONE.to_json()

        # Assert
        self.assertIsNone(value)
        self.assertEqual({"seed": None, "given": []}, report)


if __name__ == "__main__":
    unittest.main()
