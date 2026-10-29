"""csdfbmc --progress: a line on stderr every so many seconds of how far the
walk got."""
import io
import unittest

from csdfbmc.progress import Progress
from csdfbmc.tests import fixture
from csdfbmc.tests.test_stats import CHECKS, FORGETFUL


def walk(test, *args):
    from csdfbmc.__main__ import main
    return fixture.run_main(test, main, FORGETFUL, "--evaluator-module", "csdfbmc.python_glue", *args, checks=CHECKS)


class Clock:
    def __init__(self):
        self.now = 0.0

    def __call__(self):
        return self.now


class ProgressTest(unittest.TestCase):
    def test_a_line_says_the_states_frontier_depth_rate_values_tried_and_time(self):
        # Setup
        clock, out = Clock(), io.StringIO()
        p = Progress(out, clock)
        clock.now = 30.0
        p.update(visited=120, frontier=40, depth=3, tried=900)

        # Execute
        p.report()

        # Assert
        self.assertEqual("csdfbmc: 120 states, frontier 40, depth 3, 4.0 states/s, "
                         "900 field values tried in the last 30s, 0:00:30 elapsed\n", out.getvalue())

    def test_the_rate_and_the_values_tried_are_of_the_last_interval(self):
        # Setup
        clock, out = Clock(), io.StringIO()
        p = Progress(out, clock)
        clock.now = 30.0
        p.update(visited=120, frontier=40, depth=3, tried=900)
        p.report()
        clock.now = 90.0
        p.update(visited=180, frontier=10, depth=4, tried=1000)

        # Execute
        p.report()

        # Assert
        self.assertEqual("csdfbmc: 180 states, frontier 10, depth 4, 1.0 states/s, "
                         "100 field values tried in the last 60s, 0:01:30 elapsed\n", out.getvalue().splitlines(True)[1])


class ProgressOptionTest(unittest.TestCase):
    def test_zero_says_nothing(self):
        # Execute
        code, _, err = walk(self, "--progress", "0")

        # Assert
        self.assertEqual(1, code, err)
        self.assertEqual("", err)

    def test_a_negative_interval_is_refused(self):
        # Execute
        with self.assertRaises(SystemExit) as cm:
            walk(self, "--progress", "-1")

        # Assert
        self.assertEqual(2, cm.exception.code)

    def test_the_walk_tells_it_each_state_it_expands(self):
        # Setup
        from csdfbmc.checks import Checks
        from csdfbmc.tests.fixture import explorer, load
        seen = []

        class Recorder:
            def update(self, **counts):
                seen.append(counts)

        # Execute
        explorer(load(FORGETFUL), Checks(types={"N": [0, 1, 2], "Q": [0, 1]})).run(progress=Recorder())

        # Assert
        self.assertEqual(5, len(seen))
        self.assertEqual({"visited": 5, "frontier": 0, "depth": 2, "tried": 5}, seen[-1])
