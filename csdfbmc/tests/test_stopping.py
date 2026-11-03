"""csdfbmc stopped by a signal: the walk ends where it is and says what it
found so far, so that a run cut short by timeout or ^C leaves its report."""
import json
import os
import signal
import unittest

from csdfbmc.randomwalk import RandomWalks
from csdfbmc.stopping import on_signals
from csdfbmc.tests import fixture
from csdfbmc.tests.test_explore import BUMPING, COUNTER


class OnSignalsTest(unittest.TestCase):
    def test_says_the_signal_received_and_puts_the_handler_back_on_leaving(self):
        # Setup
        before = signal.getsignal(signal.SIGTERM)

        # Execute
        with on_signals() as stop:
            quiet = stop()
            os.kill(os.getpid(), signal.SIGTERM)
            got = stop()

        # Assert
        self.assertEqual((None, "SIGTERM"), (quiet, got))
        self.assertIs(before, signal.getsignal(signal.SIGTERM))


class StopTest(unittest.TestCase):
    def test_a_walk_asked_to_stop_ends_and_says_why(self):
        # Execute
        result = fixture.explorer(COUNTER).run(stop=lambda: "SIGTERM")

        # Assert
        self.assertEqual("SIGTERM", result.stopped_by)
        self.assertEqual([("a", (2,), ())], result.states)
        self.assertEqual("SIGTERM", result.to_json()["stopped_by"])

    def test_a_walk_not_asked_to_stop_says_nothing_of_it(self):
        # Execute
        result = fixture.explorer(COUNTER).run(stop=lambda: None)

        # Assert
        self.assertIsNone(result.stopped_by)
        self.assertEqual([("b", (), ())], result.deadlocks)

    def test_random_walks_asked_to_stop_end_and_say_why(self):
        # Setup
        asked = iter([None, None, "SIGINT"])

        # Execute
        got = RandomWalks(fixture.explorer(BUMPING), max_depth=4, weights={}, seed=1).run(10, stop=lambda: next(asked))

        # Assert
        self.assertEqual((2, "SIGINT"), (got.walks, got.result.stopped_by))


class StopOptionTest(unittest.TestCase):
    def test_a_run_stopped_by_a_signal_says_so_and_what_it_found(self):
        import contextlib
        from unittest import mock
        from csdfbmc.tests.test_random_walk import command
        for args in ((), ("--random-walk", "--seed", "1"), ("--jobs", "2")):
            with self.subTest(args=args):
                # Setup
                stopped = contextlib.nullcontext(lambda: "SIGTERM")

                # Execute
                with mock.patch("csdfbmc.__main__.on_signals", return_value=stopped):
                    code, out, err = command(self, *args)

                # Assert
                self.assertEqual(0, code, err)
                self.assertIn(" (stopped by SIGTERM)\n", out.splitlines(True)[0])

    def test_the_json_says_whether_a_signal_stopped_the_run(self):
        from csdfbmc.tests.test_random_walk import command
        for args in ((), ("--random-walk", "--seed", "1")):
            with self.subTest(args=args):
                # Execute
                _, out, err = command(self, "--json", *args)

                # Assert
                self.assertIsNone(json.loads(out)["stopped_by"], err)
