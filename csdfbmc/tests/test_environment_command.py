"""csdfbmc takes the environment from the checks module, --env and --seed,
and reports what reproduces the run."""
import json
import unittest

from csdfbmc.tests import fixture
from csdfbmc.tests.test_explore import COUNTER

# COUNTER with dec's guard not formalised yet.
UNKNOWN_GUARD = dict(COUNTER, edges=[dict(COUNTER["edges"][0], guard="$n が 正"), COUNTER["edges"][1]])


class EnvironmentCommandTest(unittest.TestCase):
    def run_main(self, *args, checks="TYPES = {}\n") -> tuple[int, str, str]:
        from csdfbmc.__main__ import main
        return fixture.run_main(self, main, UNKNOWN_GUARD, "--evaluator-module", "csdfbmc.python_glue", "--max-states", "10", *args, checks=checks)

    def test_a_seed_is_reported_and_walks_the_same_states_again(self):
        # Execute
        _, first, _ = self.run_main("--seed", "7")
        _, again, _ = self.run_main("--seed", "7")

        # Assert
        self.assertEqual(first, again)
        self.assertIn("environment: seed 7\n", first)
        self.assertIn("decided by the environment: $_0 が 正\n", first)

    def test_env_gives_a_function_its_value_over_the_checks_module(self):
        # Setup
        cases = [
            ("--env gives it", "TYPES = {}\n"),
            ("--env wins over the checks module", 'TYPES = {}\nENVIRONMENT = {"$n が 正": True}\n'),
        ]
        for name, checks in cases:
            with self.subTest(name):
                # Execute
                _, out, _ = self.run_main("--env", "$n が 正=false", checks=checks)

                # Assert
                self.assertIn("states: 1\n", out)
                self.assertIn("environment: given $_0 が 正\n", out)

    def test_what_the_environment_cannot_decide_is_a_usage_error(self):
        # Setup
        cases = [
            ("a malformed --env", ("--env", "$n が 正=maybe"), "TYPES = {}\n", "--env expects NAME=true or NAME=false"),
            ("a function nothing decides", (), "TYPES = {}\n", "nothing decides the environment's functions '$_0 が 正'"),
            ("a given name of no function", ("--env", "$n が 零=true", "--seed", "1"), "TYPES = {}\n", "names no function of the diagram: '$_0 が 零'"),
            ("a function of other arguments", (), 'TYPES = {}\nENVIRONMENT = {"$n が 正": lambda: True}\n', "has 1 hole"),
            ("a function giving other than a truth value", (), 'TYPES = {}\nENVIRONMENT = {"$n が 正": lambda n: n}\n', "it gives True or False"),
        ]
        for name, args, checks, message in cases:
            with self.subTest(name):
                # Execute
                code, _, err = self.run_main(*args, checks=checks)

                # Assert
                self.assertEqual(2, code)
                self.assertIn(message, err)

    def test_opaque_is_no_option_any_more(self):
        # Execute, Assert
        with self.assertRaises(SystemExit) as raised:
            self.run_main("--opaque", "true")
        self.assertEqual(2, raised.exception.code)

    def test_the_json_report_carries_the_environment(self):
        # Execute
        _, out, _ = self.run_main("--seed", "3", "--json")

        # Assert
        self.assertEqual({"seed": 3, "given": [], "decided": ["$_0 が 正"]}, json.loads(out)["environment"])


if __name__ == "__main__":
    unittest.main()
