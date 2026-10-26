"""The environment: the pure functions the atoms the checks module declares
denote, given by the checks module or the command line, and never chosen."""
import unittest

from csdfbmc import environment
from csdfbmc.environment import given_over, nothing, over, recording


class GivenTest(unittest.TestCase):
    def test_a_given_function_decides_its_name(self):
        # Setup
        cases = [
            ("a value is the function that is always it", {"$_0 が 十分": False}, "$_0 が 十分", [(n,) for n in range(3)], [False, False, False]),
            ("a function takes the arguments in order", {"$_0 が $_1 以上": lambda 在庫, 額: 在庫 >= 額}, "$_0 が $_1 以上",
             [(n, 1) for n in range(3)], [False, True, True]),
            ("a name not given is left to the inner", {"$_0 が 十分": True}, "$_0 が 不足", [(0,)], [None]),
        ]
        for name, given, asked, args, want in cases:
            with self.subTest(name):
                decide = given_over(given, nothing)

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
    def test_the_command_line_wins_over_the_module_and_nothing_decides_the_rest(self):
        # Setup
        env = over({"p": True, "q": True, "r": None}, {"q": False})

        # Execute
        got = [env.decide(n, ()) for n in ("p", "q", "r")]

        # Assert
        self.assertEqual([True, False, None], got)

    def test_the_module_declares_the_functions_by_name(self):
        # Execute
        env = over({"$_0 が凍結されていない": None, "q": True}, {})

        # Assert
        self.assertEqual(frozenset({"$_0 が凍結されていない", "q"}), env.declared)

    def test_a_declared_function_nothing_gives_is_undecided(self):
        # Execute
        value = over({"r": None}, {}).decide("r", ())

        # Assert
        self.assertIsNone(value)

    def test_the_command_line_may_give_only_a_declared_function(self):
        with self.assertRaisesRegex(ValueError, "--env names 'p', which the checks module's ENVIRONMENT does not declare"):
            over({"q": None}, {"p": True})

    def test_the_report_names_the_given_names_and_the_env_values_and_no_seed(self):
        # Setup
        env = over({"q": True, "p": None, "r": None}, {"p": False})

        # Execute
        report = env.to_json()

        # Assert
        self.assertEqual({"given": ["p", "q"], "env": {"p": False}}, report)

    def test_none_declares_nothing_and_reports_nothing(self):
        # Execute
        declared, report = environment.NONE.declared, environment.NONE.to_json()

        # Assert
        self.assertEqual(frozenset(), declared)
        self.assertEqual({"given": [], "env": {}}, report)


class EnvironmentTest(unittest.TestCase):
    def test_should_require_the_values_env_gave(self):
        # Execute / Assert: an environment that does not say what --env gave
        # would be reported as if --env gave nothing.
        with self.assertRaises(TypeError):
            environment.Environment(decide=nothing, declared=frozenset(), given={})


if __name__ == "__main__":
    unittest.main()
