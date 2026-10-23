"""csdfbmc and csdfbmc next as commands: the inputs they read, the glue they
read the atoms through, the environment, and what they print."""
import io
import json
import os
import shlex
import sys
import unittest
from contextlib import redirect_stderr

from csdfbmc.tests import fixture
from csdfbmc.tests.fixture import atom, diagram, edge
from csdfbmc.tests.test_explore import COUNTER, OPENING

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

# COUNTER with dec's guard not formalised yet.
UNKNOWN_GUARD = dict(COUNTER, edges=[dict(COUNTER["edges"][0], guard=[atom("$n が 正")]), COUNTER["edges"][1]])
DECLARED = 'TYPES = {}\nENVIRONMENT = {"$_0 が 正": None}\n'


def walk(test, *args, d=COUNTER, checks="TYPES = {}\n"):
    from csdfbmc.__main__ import main
    return fixture.run_main(test, main, d, *args, checks=checks)


def ask_next(test, *args, d=COUNTER, checks="TYPES = {}\n"):
    from csdfbmc.next import main
    return fixture.run_main(test, main, d, "--evaluator-module", "csdfbmc.python_glue", *args, checks=checks)


class GlueTest(unittest.TestCase):
    def test_the_checker_reads_the_atoms_through_either_form_of_the_glue(self):
        # The command finds the package through PYTHONPATH (so no -E), wherever the test runs.
        command = shlex.join(["env", f"PYTHONPATH={ROOT}", sys.executable, "-s", "-B", "-m", "csdfbmc.python_glue"])
        for flag, value in (("--evaluator", command), ("--evaluator-module", "csdfbmc.python_glue")):
            with self.subTest(flag=flag):
                # Execute
                code, out, err = walk(self, flag, value)

                # Assert
                self.assertEqual(1, code, err)
                self.assertIn("deadlock at b [] after ['dec', 'dec', 'stop']", out)

    def test_what_it_cannot_start_or_read_is_a_loading_error(self):
        tests = {
            "no glue": ((), "csdfbmc reads every clause through one"),
            "a glue that cannot start": (("--evaluator-module", "no.such.glue"), "no.such.glue"),
            "an empty glue module": (("--evaluator-module", ""), "MODULE"),
        }
        for name, (args, said) in tests.items():
            with self.subTest(name):
                # Execute
                code, _, err = walk(self, *args)

                # Assert
                self.assertEqual(2, code)
                self.assertIn(said, err)

    def test_a_diagram_of_another_version_is_a_loading_error(self):
        # Execute
        code, _, err = walk(self, "--evaluator-module", "csdfbmc.python_glue", d=dict(COUNTER, version=0))

        # Assert
        self.assertEqual(2, code)
        self.assertIn("csdfbmc reads version 2", err)

    def test_the_options_of_reductions_not_back_yet_and_both_glues_are_refused(self):
        for name, args in {"both forms": ("--evaluator", "x", "--evaluator-module", "y"),
                           "--por": ("--evaluator-module", "csdfbmc.python_glue", "--por")}.items():
            with self.subTest(name):
                # Execute, Assert
                with redirect_stderr(io.StringIO()), self.assertRaises(SystemExit) as raised:
                    walk(self, *args)
                self.assertEqual(2, raised.exception.code)


class EnvironmentCommandTest(unittest.TestCase):
    def run_main(self, *args, checks=DECLARED):
        return walk(self, "--evaluator-module", "csdfbmc.python_glue", "--max-states", "10", *args, d=UNKNOWN_GUARD, checks=checks)

    def test_a_seed_is_reported_and_walks_the_same_states_again(self):
        # Execute
        _, first, _ = self.run_main("--seed", "7")
        _, again, _ = self.run_main("--seed", "7")

        # Assert
        self.assertEqual(first, again)
        self.assertIn("environment: seed 7\n", first)
        self.assertIn("decided by the environment: $_0 が 正\n", first)

    def test_env_gives_a_declared_function_its_value_over_the_checks_module(self):
        for name, checks in {"declared and left open": DECLARED, "given by the module": 'TYPES = {}\nENVIRONMENT = {"$_0 が 正": True}\n'}.items():
            with self.subTest(name):
                # Execute
                _, out, _ = self.run_main("--env", "$_0 が 正=false", checks=checks)

                # Assert
                self.assertIn("states: 1\n", out)
                self.assertIn("environment: given $_0 が 正\n", out)

    def test_what_the_environment_cannot_decide_is_a_usage_error(self):
        cases = {
            "a malformed --env": (("--env", "$_0 が 正=maybe"), DECLARED, "--env expects NAME=true or NAME=false"),
            "a function nothing decides": ((), DECLARED, "nothing decides the environment's functions '$_0 が 正'"),
            "--env of a function not declared": (("--env", "$_0 が 零=true"), DECLARED, "ENVIRONMENT does not declare"),
            "an atom not declared, which the glue cannot read": ((), "TYPES = {}\n", "SyntaxError"),
            "a function of other arguments": ((), 'TYPES = {}\nENVIRONMENT = {"$_0 が 正": lambda: True}\n', "has 1 argument"),
            "a function giving other than a truth value": ((), 'TYPES = {}\nENVIRONMENT = {"$_0 が 正": lambda n: n}\n', "it gives True or False"),
        }
        for name, (args, checks, message) in cases.items():
            with self.subTest(name):
                # Execute
                code, _, err = self.run_main(*args, checks=checks)

                # Assert
                self.assertEqual(2, code)
                self.assertIn(message, err)

    def test_the_json_report_carries_the_environment(self):
        # Execute
        _, out, _ = self.run_main("--seed", "3", "--json")

        # Assert
        self.assertEqual({"seed": 3, "given": [], "decided": ["$_0 が 正"]}, json.loads(out)["environment"])


class ReportTest(unittest.TestCase):
    def test_a_finding_is_reported_with_its_shortest_trace(self):
        # Setup
        d = dict(COUNTER, init_post=[atom("$n' in (0, 1)")])

        # Execute
        code, out, _ = walk(self, "--evaluator-module", "csdfbmc.python_glue", d=d)

        # Assert
        self.assertEqual(1, code)
        self.assertEqual("states: 3\ndeadlock at b [] after ['stop']\n", out)

    def test_a_post_it_cannot_enumerate_is_a_loading_error(self):
        # Setup
        d = diagram({"a": ["q"]}, "a", edges=[edge("a", "a", "go")])

        # Execute
        code, _, err = walk(self, "--evaluator-module", "csdfbmc.python_glue", d=d)

        # Assert
        self.assertEqual(2, code)
        self.assertIn("q has none in TYPES", err)


class SampleReportTest(unittest.TestCase):
    """Where the bound was reached is reported apart from the deadlocks, and
    is no finding for the exit status."""

    SAMPLE = 'from csdfbmc.values import sample\nTYPES = {"k": sample(["a"])}\n'

    def opening(self, *guard):
        e = OPENING["edges"][0]
        return dict(OPENING, edges=[dict(e, guard=e["guard"] + list(guard))] + OPENING["edges"][1:2])

    def test_the_bound_reached_is_reported_and_is_no_finding(self):
        tests = {
            "the text": ((), "states: 3\nbound reached at r [{'a': {'at': 'closed', 'fields': {'bal': 0}}}] after ['OPEN(k=a)', 'CLOSE(k=a)']"
                             " (a sample type has more values)\n"),
            "every finding": (("--all",), "states: 3\nbound reached at r [{'a': {'at': 'closed', 'fields': {'bal': 0}}}] after ['OPEN(k=a)', 'CLOSE(k=a)']"
                                         " (a sample type has more values)\n"),
        }
        for name, (args, want) in tests.items():
            with self.subTest(name):
                # Execute
                code, out, err = walk(self, "--evaluator-module", "csdfbmc.python_glue", *args, d=self.opening(), checks=self.SAMPLE)

                # Assert
                self.assertEqual((0, want), (code, out), err)

    def test_the_json_report_carries_the_bound_reached(self):
        # Execute
        code, out, _ = walk(self, "--evaluator-module", "csdfbmc.python_glue", "--json", d=self.opening(), checks=self.SAMPLE)

        # Assert
        report = json.loads(out)
        self.assertEqual(0, code)
        self.assertEqual(([], ["OPEN(k=a)", "CLOSE(k=a)"]), (report["deadlocks"], report["bound_reached"][0]["trace"]))
        self.assertEqual([("bound_reached", "r", "CLOSE", 1)], [(g["kind"], g["at"], g["after"], g["count"]) for g in report["groups"]])

    def test_a_sample_type_the_diagram_orders_is_a_loading_error(self):
        # Execute
        code, _, err = walk(self, "--evaluator-module", "csdfbmc.python_glue", d=self.opening(atom("$k > 'a'")), checks=self.SAMPLE)

        # Assert
        self.assertEqual(2, code)
        self.assertIn("argument 0 is ordered", err)


class NextCommandTest(unittest.TestCase):
    def test_initial_prints_the_initial_values(self):
        # Execute
        code, out, _ = ask_next(self, "--initial")

        # Assert
        self.assertEqual((0, {"candidates": [{"params": {}, "values": [2]}]}), (code, json.loads(out)))

    def test_a_sample_type_the_diagram_orders_is_answered_since_next_tries_no_value_beyond_it(self):
        # Setup
        e = OPENING["edges"][0]
        d = dict(OPENING, edges=[dict(e, guard=e["guard"] + [atom("$k > 'a'")])])

        # Execute
        code, out, err = ask_next(self, "--initial", d=d, checks=SampleReportTest.SAMPLE)

        # Assert
        self.assertEqual((0, {"candidates": [{"params": {}, "values": [{}]}]}), (code, json.loads(out)), err)

    def test_edge_prints_the_values_after_the_edge_from_a_runtime_state(self):
        # Setup
        state = json.dumps({"state_id": "a", "state_name": "counting", "values": [{"name": "n", "value": 1}]})

        for edge_index, want in (("0", [{"params": {}, "values": [0]}]), ("1", [])):
            with self.subTest(edge=edge_index):
                # Execute
                code, out, _ = ask_next(self, "--state", state, "--edge", edge_index)

                # Assert
                self.assertEqual((0, {"candidates": want}), (code, json.loads(out)))


if __name__ == "__main__":
    unittest.main()
