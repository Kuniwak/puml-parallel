"""csdfbmc stopped by a signal: the walk ends where it is and says what it
found so far, so that a run cut short by timeout or ^C leaves its report."""
import json
import os
import signal
import unittest
from unittest import mock

from csdfbmc.randomwalk import RandomWalks
from csdfbmc.stopping import on_signals
from csdfbmc.tests import fixture
from csdfbmc.tests.test_explore import BUMPING, COUNTER


class OnSignalsTest(unittest.TestCase):
    def test_says_the_first_signal_received_and_puts_the_handlers_back_on_leaving(self):
        tests = {"SIGTERM": [signal.SIGTERM], "SIGINT": [signal.SIGINT], "the first of two": [signal.SIGTERM, signal.SIGINT]}
        for name, sent in tests.items():
            with self.subTest(name):
                # Setup
                before = {s: signal.getsignal(s) for s in (signal.SIGINT, signal.SIGTERM)}

                # Execute
                with on_signals() as stop:
                    quiet = stop()
                    for s in sent:
                        os.kill(os.getpid(), s)
                    got = stop()

                # Assert
                self.assertEqual((None, sent[0].name), (quiet, got))
                self.assertEqual(before, {s: signal.getsignal(s) for s in before})


class WaitedTest(unittest.TestCase):
    def test_a_worker_that_never_answers_is_left_when_a_signal_comes(self):
        import multiprocessing
        from csdfbmc import parallel

        # Setup
        class Silent:
            def get(self, timeout):
                raise multiprocessing.TimeoutError

        # Execute
        with mock.patch.object(parallel, "POLL", 0.01):
            got = parallel._waited(Silent(), lambda: "SIGTERM")

        # Assert
        self.assertIsNone(got)


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
        from csdfbmc.tests.test_random_walk import command
        for args in ((), ("--random-walk", "--seed", "1"), ("--jobs", "2")):
            with self.subTest(args=args):
                # Setup
                stopped = contextlib.nullcontext(lambda: "SIGTERM")

                # Execute
                code, out, err = command(self, *args, stopping=lambda: stopped)

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
