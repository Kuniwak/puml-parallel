"""The environment: the pure functions a clause not formalised denotes, given
by the checks module or the command line, or chosen from a seed."""
import unittest

from csdfbmc.environment import Environment
from csdfbmc.values import FrozenMap


class EnvironmentTest(unittest.TestCase):
    def test_a_given_value_is_the_function_that_is_always_it(self):
        # Setup
        env = Environment(given={"在庫 が 十分": False})

        # Execute
        values = {env.decide("在庫 が 十分", {"在庫": n}) for n in range(3)}

        # Assert
        self.assertEqual({False}, values)

    def test_a_given_function_reads_the_names_its_signature_asks_for(self):
        # Setup
        env = Environment(given={"在庫 が 十分": lambda 在庫: 在庫 > 1})

        # Execute
        values = [env.decide("在庫 が 十分", {"在庫": n, "額": 9}) for n in range(3)]

        # Assert
        self.assertEqual([False, False, True], values)

    def test_a_name_neither_given_nor_seeded_is_undecided(self):
        # Setup
        env = Environment()

        # Execute
        value = env.decide("在庫 が 十分", {})

        # Assert
        self.assertIsNone(value)

    def test_a_seed_chooses_a_pure_function_of_the_arguments(self):
        # Setup
        a, b = Environment(seed=7), Environment(seed=7)
        args = [{"n": n, "m": FrozenMap({"k": frozenset({n, "x"})})} for n in range(32)]

        # Execute
        first = [a.decide("p", x) for x in args]
        again = [a.decide("p", x) for x in args]
        other = [b.decide("p", x) for x in args]

        # Assert
        self.assertEqual(first, again)
        self.assertEqual(first, other)
        self.assertEqual({True, False}, set(first))

    def test_different_seeds_or_names_choose_different_functions(self):
        # Setup
        args = [{"n": n} for n in range(32)]

        # Execute
        by_seed = [[Environment(seed=s).decide("p", x) for x in args] for s in (1, 2)]
        by_name = [[Environment(seed=1).decide(name, x) for x in args] for name in ("p", "q")]

        # Assert
        self.assertNotEqual(by_seed[0], by_seed[1])
        self.assertNotEqual(by_name[0], by_name[1])

    def test_a_given_value_wins_over_the_seed(self):
        # Setup
        env = Environment(given={"p": True}, seed=3)

        # Execute
        values = {env.decide("p", {"n": n}) for n in range(16)}

        # Assert
        self.assertEqual({True}, values)

    def test_a_given_function_must_give_a_truth_value(self):
        # Setup
        env = Environment(given={"p": lambda n: n})

        # Execute, Assert
        with self.assertRaisesRegex(TypeError, "'p'"):
            env.decide("p", {"n": 1})

    def test_the_report_names_the_seed_and_the_given_names(self):
        # Setup
        env = Environment(given={"q": True, "p": False}, seed=5)

        # Execute
        report = env.to_json()

        # Assert
        self.assertEqual({"seed": 5, "given": ["p", "q"]}, report)


if __name__ == "__main__":
    unittest.main()
