"""csdfbmc takes the environment from the checks module, --env and --seed,
and reports what reproduces the run."""
import json
import unittest

from csdfbmc.tests import fixture
from csdfbmc.tests.test_explore import COUNTER

# COUNTER with dec's guard not formalised yet.
UNKNOWN_GUARD = dict(COUNTER, edges=[dict(COUNTER["edges"][0], guard="@opaque"), COUNTER["edges"][1]])


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
        self.assertIn("decided by the environment: @opaque#1\n", first)

    def test_env_gives_a_function_its_value(self):
        # Execute
        _, out, _ = self.run_main("--env", "@opaque#1=false")

        # Assert
        self.assertIn("states: 1\n", out)
        self.assertIn("environment: given @opaque#1\n", out)

    def test_env_wins_over_the_checks_module(self):
        # Execute
        _, out, _ = self.run_main("--env", "@opaque#1=false", checks='TYPES = {}\nENVIRONMENT = {"@opaque#1": True}\n')

        # Assert
        self.assertIn("states: 1\n", out)

    def test_a_malformed_env_is_a_usage_error(self):
        # Execute
        code, _, err = self.run_main("--env", "@opaque#1=maybe")

        # Assert
        self.assertEqual(2, code)
        self.assertIn("--env expects NAME=true or NAME=false", err)

    def test_the_json_report_carries_the_environment(self):
        # Execute
        _, out, _ = self.run_main("--seed", "3", "--json")

        # Assert
        self.assertEqual({"seed": 3, "given": [], "decided": ["@opaque#1"]}, json.loads(out)["environment"])


if __name__ == "__main__":
    unittest.main()
