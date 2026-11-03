"""csdfbmc --random-walk: walks from the start, each step one of the ways
out chosen at random by its event's weight, for findings deeper than a
breadth-first walk reaches."""
import json
import unittest

from csdfbmc.randomwalk import RandomWalks
from csdfbmc.tests import fixture
from csdfbmc.tests.fixture import diagram, edge
from csdfbmc.tests.test_explore import BUMPING, COUNTER, TALLY, tally_checks
from csdfbmc.tests.test_stats import CHECKS, FORGETFUL


def walks(d, checks=None, max_depth=10, weights=None, seed=1, **options):
    return RandomWalks(fixture.explorer(d, checks, **options), max_depth=max_depth, weights=weights or {}, seed=seed)


class RandomWalkTest(unittest.TestCase):
    def test_reports_a_state_with_no_way_out_as_a_deadlock_with_its_trace(self):
        # Execute
        got = walks(COUNTER).run(1)

        # Assert
        self.assertEqual([("b", (), ())], got.result.deadlocks)
        self.assertEqual(["dec", "dec", "stop"], got.result.trace_to(got.result.deadlocks[0]))

    def test_a_walk_ends_at_the_depth_given(self):
        # Execute
        got = walks(BUMPING, max_depth=5, weights={"stop": 0}).run(3)

        # Assert
        self.assertEqual([], got.result.deadlocks)
        self.assertEqual((3, 15, 5), (got.walks, got.steps, got.deepest))

    def test_counts_the_events_fired_and_says_those_never_fired(self):
        # Execute
        got = walks(BUMPING, max_depth=5, weights={"stop": 0}).run(3)

        # Assert
        self.assertEqual(15, sum(got.fired.values()))
        self.assertEqual(["stop"], got.never_fired)

    def test_a_heavier_event_is_taken_more_often(self):
        # Setup
        d = diagram({"a": []}, "a", edges=[edge("a", "a", "x"), edge("a", "a", "y")])

        # Execute
        light = walks(d, max_depth=1, weights={"y": 1}).run(400)
        heavy = walks(d, max_depth=1, weights={"y": 9}).run(400)

        # Assert
        self.assertLess(light.fired["y"], 260)
        self.assertGreater(heavy.fired["y"], 320)

    def test_a_seed_gives_the_same_walks_whatever_order_they_are_taken_in(self):
        # Setup
        rw = walks(BUMPING, max_depth=6)

        # Execute
        forward = [rw.walk(i).events() for i in range(5)]
        backward = [rw.walk(i).events() for i in reversed(range(5))]

        # Assert
        self.assertEqual(forward, list(reversed(backward)))
        self.assertNotEqual(forward, [walks(BUMPING, max_depth=6, seed=2).walk(i).events() for i in range(5)])

    def test_reports_a_broken_invariant_and_ends_the_walk_there(self):
        # Setup
        checks = tally_checks(invariants={"在庫は 4 未満": lambda 在庫: 在庫 < 4})

        # Execute
        got = walks(TALLY, checks, max_depth=30, weights={"搬出": 0}).run(1)

        # Assert
        [(name, s)] = got.result.violations
        self.assertEqual("在庫は 4 未満", name)
        self.assertGreaterEqual(s[1][0], 4)
        self.assertLessEqual(len(got.result.trace_to(s)), got.deepest)

    def test_a_finding_met_again_is_kept_once(self):
        # Execute
        got = walks(COUNTER).run(5)

        # Assert
        self.assertEqual([("b", (), ())], got.result.deadlocks)
        self.assertEqual(1, got.result.groups()[0]["count"])


def run(test, *args):
    from csdfbmc.__main__ import main
    return fixture.run_main(test, main, FORGETFUL, "--evaluator-module", "csdfbmc.python_glue", "--random-walk", *args, checks=CHECKS)


def tsv(test, text):
    import os
    import tempfile
    d = tempfile.TemporaryDirectory()
    test.addCleanup(d.cleanup)
    path = os.path.join(d.name, "weights.tsv")
    with open(path, "w", encoding="utf-8") as f:
        f.write(text)
    return path


class RandomWalkOptionTest(unittest.TestCase):
    def test_says_how_far_the_walks_went_and_what_they_found(self):
        # Execute
        code, out, err = run(self, "--walks", "3", "--seed", "5")

        # Assert
        self.assertEqual(1, code, err)
        self.assertTrue(out.startswith("random walks: 3, steps: 6, deepest: 2, seed: 5\n"), out)
        self.assertRegex(out, r"\ndeadlock.* at a .*\['inc', 'inc'\]")
        self.assertIn("fired: inc 6\n", out)

    def test_a_seed_gives_the_same_report_with_workers_or_without(self):
        # Setup
        _, expected, _ = run(self, "--walks", "4", "--seed", "9", "--json")

        # Execute
        code, actual, err = run(self, "--walks", "4", "--seed", "9", "--json", "--jobs", "2")

        # Assert
        self.assertEqual(1, code, err)
        self.assertEqual(json.loads(expected), json.loads(actual))
        self.assertEqual({"walks": 4, "steps": 8, "deepest": 2, "seed": 9, "fired": {"inc": 8}, "never_fired": []},
                         json.loads(actual)["random_walk"])

    def test_the_functions_the_environment_decided_are_said_as_the_breadth_first_walk_says_them(self):
        from csdfbmc.tests.test_cli import DECLARED, UNKNOWN_GUARD, walk
        # Setup
        common = ("--evaluator-module", "csdfbmc.python_glue", "--env", "$_0 が 正=true", "--json")
        _, out, _ = walk(self, *common, d=UNKNOWN_GUARD, checks=DECLARED)
        expected = json.loads(out)["environment"]
        for jobs in ("1", "2"):
            with self.subTest(jobs=jobs):
                # Execute
                _, out, err = walk(self, *common, "--random-walk", "--walks", "2", "--jobs", jobs, d=UNKNOWN_GUARD, checks=DECLARED)

                # Assert
                self.assertEqual(expected, json.loads(out)["environment"], err)

    def test_a_weight_of_zero_keeps_an_event_from_firing(self):
        # Execute
        code, out, err = run(self, "--walks", "2", "--seed", "1", "--weight-tsv", tsv(self, "inc\t0\n"))

        # Assert
        self.assertEqual(0, code, err)
        self.assertIn("fired: nothing\nnever fired: inc\n", out)

    def test_a_weight_line_is_read_as_tsv(self):
        # Execute
        code, out, err = run(self, "--walks", "2", "--seed", "1", "--weight-tsv", tsv(self, '"inc"\t0\n'))

        # Assert
        self.assertEqual(0, code, err)
        self.assertIn("never fired: inc\n", out)

    def test_a_malformed_weight_line_is_refused(self):
        tests = {"an unknown event": "incc\t2\n", "no weight": "inc\n", "a negative weight": "inc\t-1\n"}
        for name, text in tests.items():
            with self.subTest(name):
                # Execute
                code, _, err = run(self, "--weight-tsv", tsv(self, text))

                # Assert
                self.assertEqual(2, code)
                self.assertIn("weight", err)

    def test_options_of_a_breadth_first_walk_are_refused(self):
        for args in (("--avoid", "inc"), ("--symmetry",), ("--max-states", "5"), ("--stats",), ("--checkpoint", "x"), ("--resume", "x")):
            with self.subTest(args=args):
                # Execute
                with self.assertRaises(SystemExit) as cm:
                    run(self, *args)

                # Assert
                self.assertEqual(2, cm.exception.code)

    def test_options_of_random_walks_need_random_walk(self):
        from csdfbmc.__main__ import main
        for args in (("--walks", "3"), ("--max-depth", "3"), ("--seed", "1"), ("--weight-tsv", "x")):
            with self.subTest(args=args):
                # Execute
                with self.assertRaises(SystemExit) as cm:
                    fixture.run_main(self, main, FORGETFUL, "--evaluator-module", "csdfbmc.python_glue", *args, checks=CHECKS)

                # Assert
                self.assertEqual(2, cm.exception.code)
