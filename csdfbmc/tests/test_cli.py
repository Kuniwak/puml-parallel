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
from csdfbmc.tests.fixture import atom, diagram, edge, fld, idx
from csdfbmc.tests.test_explore import COUNTER, OPENING

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

# COUNTER with dec's guard not formalised yet.
UNKNOWN_GUARD = dict(COUNTER, edges=[dict(COUNTER["edges"][0], guard=[atom("$n が 正")]), COUNTER["edges"][1]])
DECLARED = 'TYPES = {}\nENVIRONMENT = {"$_0 が 正": None}\n'


def walk(test, *args, d=COUNTER, checks="TYPES = {}\n"):
    from csdfbmc.__main__ import main
    return fixture.run_main(test, main, d, *args, checks=checks)


def ask_next(test, *args, d=COUNTER):
    from csdfbmc.next import main
    return fixture.run_main(test, main, d, "--evaluator-module", "csdfbmc.python_glue", *args)


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
        self.assertIn("csdfbmc reads version 3", err)

    def test_both_glues_are_refused(self):
        for name, args in {"both forms": ("--evaluator", "x", "--evaluator-module", "y")}.items():
            with self.subTest(name):
                # Execute, Assert
                with redirect_stderr(io.StringIO()), self.assertRaises(SystemExit) as raised:
                    walk(self, *args)
                self.assertEqual(2, raised.exception.code)


REDUCED_BY = "reduced by {}: the findings are those of the full walk; the counts and --all list the states of the reduced one\n"


class ReductionCommandTest(unittest.TestCase):
    def test_cone_reports_what_it_leaves_out(self):
        # Setup
        from csdfbmc.tests.test_cone import MEMO

        # Execute
        code, out, err = walk(self, "--evaluator-module", "csdfbmc.python_glue", "--cone", d=MEMO, checks='TYPES = {"x": [1, 2, 3]}\n')

        # Assert
        self.assertEqual(1, code, err)
        self.assertEqual("states: 4\n" + REDUCED_BY.format("--cone") + "cone of influence leaves out: memo\n", out.split("deadlock")[0])

    def test_a_reduced_walk_finds_the_shapes_of_the_full_one_and_says_its_counts_are_its_own(self):
        # Setup
        from csdfbmc.tests.test_cone import MEMO
        args = ("--evaluator-module", "csdfbmc.python_glue", "--json")
        checks = 'TYPES = {"x": [1, 2, 3]}\n'

        # Execute
        _, full, _ = walk(self, *args, d=MEMO, checks=checks)
        _, reduced, _ = walk(self, *args, "--cone", d=MEMO, checks=checks)
        _, text, _ = walk(self, *args[:2], "--cone", "--all", d=MEMO, checks=checks)

        # Assert
        def shapes(report):
            return [{k: v for k, v in g.items() if k not in ("count", "trace", "example")} for g in report["groups"]]
        self.assertEqual(shapes(json.loads(full)), shapes(json.loads(reduced)))
        self.assertEqual([], json.loads(full)["reductions"])
        self.assertEqual(["cone"], json.loads(reduced)["reductions"])
        self.assertIn(REDUCED_BY.format("--cone"), text)

    def test_cone_reports_a_value_left_out_in_json(self):
        # Setup
        from csdfbmc.tests.test_cone import MEMO

        # Execute
        code, out, err = walk(self, "--evaluator-module", "csdfbmc.python_glue", "--cone", "--json", d=MEMO, checks='TYPES = {"x": [1, 2, 3]}\n')

        # Assert
        self.assertEqual(1, code, err)
        report = json.loads(out)
        self.assertEqual(["memo"], report["left_out"])
        self.assertEqual([0, {"left_out": True}], report["deadlocks"][0]["replay"]["steps"][-2]["values"])

    def test_por_walks_fewer_states_and_reports_the_same(self):
        # Setup
        from csdfbmc.tests.test_por import ONE_WAY
        checks = 'TYPES = {"k": ["a", "b", "c"]}\n'

        # Execute
        _, full, _ = walk(self, "--evaluator-module", "csdfbmc.python_glue", d=ONE_WAY, checks=checks)
        code, reduced, err = walk(self, "--evaluator-module", "csdfbmc.python_glue", "--por", d=ONE_WAY, checks=checks)

        # Assert
        self.assertEqual(1, code, err)
        self.assertEqual("states: 27", full.splitlines()[0])
        self.assertLess(int(reduced.splitlines()[0].split()[1]), 27)
        self.assertEqual(REDUCED_BY.format("--por"), reduced.splitlines(keepends=True)[1])
        self.assertEqual(full.splitlines()[1].split(" ×")[0], reduced.splitlines()[2].split(" ×")[0])


    def test_symmetry_walks_fewer_states_and_reports_the_same(self):
        # Setup
        from csdfbmc.tests.test_por import ONE_WAY
        checks = 'TYPES = {"k": ["a", "b", "c"]}\nSYMMETRIC = ["k"]\n'

        # Execute
        code, out, err = walk(self, "--evaluator-module", "csdfbmc.python_glue", "--symmetry", d=ONE_WAY, checks=checks)

        # Assert
        self.assertEqual(1, code, err)
        self.assertEqual("states: 10", out.splitlines()[0])

    def test_symmetry_that_the_checks_refuse_is_a_loading_error(self):
        # Setup
        from csdfbmc.tests.test_por import ONE_WAY
        checks = 'TYPES = {"k": ["a", "b", "c"]}\nSYMMETRIC = ["k"]\nINVARIANTS = {"x": lambda m: "a" not in m}\n'

        # Execute
        code, _, err = walk(self, "--evaluator-module", "csdfbmc.python_glue", "--symmetry", d=ONE_WAY, checks=checks)

        # Assert
        self.assertEqual(2, code)
        self.assertIn("the constant 'a' names one symmetric ID", err)

    def test_symmetry_refused_during_the_walk_reports_no_finding(self):
        # Setup: only an atom puts an ID in last, which the walk meets.
        from csdfbmc.tests.test_por import CLOSE, OPEN, ONE_WAY
        d = dict(ONE_WAY, vars=fixture.typed_vars({"r": ["m", "last"]}), init_post=ONE_WAY["init_post"] + [atom("$last' == ''")],
                 edges=[dict(OPEN, post=OPEN["post"] + [atom("$last' == $k")]), dict(CLOSE, post=CLOSE["post"] + [fixture.eq("last'", "last")])])
        checks = 'TYPES = {"k": ["a", "b", "c"]}\nSYMMETRIC = ["k"]\n'
        args = ("--evaluator-module", "csdfbmc.python_glue", "--symmetry")
        tests = {"in full": (), "within a bound past the ID": ("--max-states", "2"), "at the first finding": ("--first",)}
        for name, more in tests.items():
            with self.subTest(name):
                # Execute
                code, out, err = walk(self, *args, *more, d=d, checks=checks)

                # Assert
                self.assertEqual((2, ""), (code, out))
                self.assertIn("last holds 'a'", err)


class EnvironmentCommandTest(unittest.TestCase):
    def run_main(self, *args, checks=DECLARED):
        return walk(self, "--evaluator-module", "csdfbmc.python_glue", "--max-states", "10", *args, d=UNKNOWN_GUARD, checks=checks)

    def test_seed_is_no_option(self):
        # Execute, Assert
        with redirect_stderr(io.StringIO()), self.assertRaises(SystemExit) as raised:
            self.run_main("--seed", "7")
        self.assertEqual(2, raised.exception.code)

    def test_a_given_function_is_reported_and_decided_by(self):
        # Execute
        _, out, _ = self.run_main("--env", "$_0 が 正=true")

        # Assert
        self.assertNotIn("seed", out)
        self.assertIn("decided by the environment: $_0 が 正\n", out)

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
        _, out, _ = self.run_main("--env", "$_0 が 正=true", "--json")

        # Assert
        self.assertEqual({"given": ["$_0 が 正"], "env": {"$_0 が 正": True}, "decided": ["$_0 が 正"]}, json.loads(out)["environment"])


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
        self.assertIn("q', so it ranges over its type q: no type 'q' in TYPES", err)


class SampleReportTest(unittest.TestCase):
    """Where the bound was reached is reported apart from the deadlocks, and
    is no finding for the exit status."""

    SAMPLE = 'from csdfbmc.values import sample\nTYPES = {"k": sample(["a"], beyond=["b"])}\n'

    def opening(self, *guard):
        """OPENING without STOP, OPEN guarded by guard too and opening an
        instance with a count of 0 or 1: two states where every ID is used
        up, after CLOSE."""
        e = OPENING["edges"][0]
        post = e["post"][:-1] + [atom("${} in [0, 1]", fld(idx("m'", "k"), "cnt"))]
        return dict(OPENING, edges=[dict(e, guard=e["guard"] + list(guard), post=post)] + OPENING["edges"][1:2])

    def test_the_bound_reached_is_reported_and_is_no_finding(self):
        why = " (OPEN(k=b) is enabled by a value beyond a sample)"
        closed = "r [{'a': {'at': 'closed', 'fields': {'cnt': %d}}}] after ['OPEN(k=a)', 'CLOSE(k=a)']" + why
        tests = {
            "grouped": ((), (), 0, "states: 5\nbound reached ×2 at r after CLOSE; first after ['OPEN(k=a)', 'CLOSE(k=a)']" + why + "\n"),
            "every one": (("--all",), (), 0, f"states: 5\nbound reached at {closed % 0}\nbound reached at {closed % 1}\n"),
            "a deadlock when no value beyond enables a step": ((), (atom("$k != 'b'"),), 1, "states: 5\ndeadlock ×2 at r after CLOSE; first after ['OPEN(k=a)', 'CLOSE(k=a)']\n"),
        }
        for name, (args, guard, want_code, want) in tests.items():
            with self.subTest(name):
                # Execute
                code, out, err = walk(self, "--evaluator-module", "csdfbmc.python_glue", *args, d=self.opening(*guard), checks=self.SAMPLE)

                # Assert
                self.assertEqual((want_code, want), (code, out), err)

    def test_the_bound_reached_is_reported_in_json_with_its_witness(self):
        # Execute
        code, out, err = walk(self, "--evaluator-module", "csdfbmc.python_glue", "--json", d=self.opening(), checks=self.SAMPLE)

        # Assert
        self.assertEqual(0, code, err)
        report = json.loads(out)
        self.assertEqual({"deadlocks": [], "bound_reached": [[["OPEN(k=a)", "CLOSE(k=a)"], "OPEN(k=b)"]] * 2,
                          "groups": [["bound_reached", "r", "CLOSE", 2]]},
                         {"deadlocks": report["deadlocks"],
                          "bound_reached": [[r["trace"], r["witness"]] for r in report["bound_reached"]],
                          "groups": [[g["kind"], g["at"], g["after"], g["count"]] for g in report["groups"]]})


class NextCommandTest(unittest.TestCase):
    def test_initial_prints_the_initial_values(self):
        # Execute
        code, out, _ = ask_next(self, "--initial")

        # Assert
        self.assertEqual((0, {"candidates": [{"params": {}, "values": [2]}]}), (code, json.loads(out)))

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
