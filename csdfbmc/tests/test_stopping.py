"""csdfbmc stopped by a signal: the walk ends where it is and says what it
found so far, so that a run cut short by timeout or ^C leaves its report."""
import json
import os
import signal
import unittest

from csdfbmc.randomwalk import RandomWalks
from csdfbmc.stopping import interrupted, on_signals
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


def signalled_after(first):
    """first, then a SIGTERM to this process while the next is being made,
    which takes far longer than a test may wait."""
    import time
    yield first
    os.kill(os.getpid(), signal.SIGTERM)
    time.sleep(30)


class InterruptedTest(unittest.TestCase):
    def test_a_signal_ends_what_is_being_made_and_what_was_made_is_kept(self):
        import time
        walk, walks = fixture.explorer(COUNTER), RandomWalks(fixture.explorer(BUMPING), max_depth=4, weights={}, seed=1)

        def items(stop):
            return list(interrupted(signalled_after(1))), stop()

        def breadth_first(stop):
            result = walk.run(lambda queue, i: signalled_after(walk.expand(queue[i])), stop=stop)
            return result.states, result.stopped_by

        def random_walks(stop):
            got = walks.run(10, take=lambda indices: signalled_after(walks.walk(indices[0])), stop=stop)
            return got.walks, got.result.stopped_by

        tests = {
            "items": (items, ([1], "SIGTERM")),
            "a breadth-first walk": (breadth_first, ([("a", (2,), ()), ("a", (1,), ())], "SIGTERM")),
            "random walks": (random_walks, (1, "SIGTERM")),
        }
        for name, (run, want) in tests.items():
            with self.subTest(name):
                # Execute
                started = time.monotonic()
                with on_signals() as stop:
                    got = run(stop)

                # Assert
                self.assertEqual(want, got)
                self.assertLess(time.monotonic() - started, 5)


class WaitedTest(unittest.TestCase):
    def test_a_worker_that_never_answers_is_left_when_a_signal_comes(self):
        import multiprocessing
        from csdfbmc import parallel

        # Setup
        class Silent:
            def get(self, timeout):
                raise multiprocessing.TimeoutError

        # Execute
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

    def test_a_walk_stopped_by_a_signal_does_not_ask_why_a_deadlock_is_stuck(self):
        # The glue may have been killed in the middle of a request, and
        # asking it takes as long as a step.
        def stopped_at_the_end(asked):
            # The signal comes after the last state or walk, asked once each.
            answers = iter([None] * asked)
            return lambda: next(answers, "SIGTERM")

        tests = {
            "breadth first": lambda: fixture.explorer(COUNTER).run(stop=stopped_at_the_end(4)),
            "random walks": lambda: RandomWalks(fixture.explorer(COUNTER), max_depth=4, weights={}, seed=1).run(
                1, stop=stopped_at_the_end(1)).result,
        }
        for name, run in tests.items():
            with self.subTest(name):
                # Execute
                result = run()

                # Assert
                self.assertEqual(("SIGTERM", [("b", (), ())], {}), (result.stopped_by, result.deadlocks, result.stuck))

    def test_random_walks_asked_to_stop_end_and_say_why(self):
        # Setup
        asked = iter([None, None, "SIGINT"])

        # Execute
        got = RandomWalks(fixture.explorer(BUMPING), max_depth=4, weights={}, seed=1).run(10, stop=lambda: next(asked))

        # Assert
        self.assertEqual((2, "SIGINT"), (got.walks, got.result.stopped_by))


class SignalledCommandTest(unittest.TestCase):
    """csdfbmc run as a command and signalled in the middle of a step, as
    timeout signals its process group and a terminal its foreground one:
    its glue (csdfbmc.tests.slow_glue) says when a step has started, and
    the step takes far longer than the test waits."""

    def started(self, *args):
        """csdfbmc on COUNTER with args, in a process group of its own, once
        it is in the middle of its first step; and its marker."""
        import subprocess
        import sys
        import tempfile
        import time
        d = tempfile.TemporaryDirectory()
        self.addCleanup(d.cleanup)
        path, checks, marker = (os.path.join(d.name, n) for n in ("d.json", "t.py", "marker"))
        with open(path, "w", encoding="utf-8") as f:
            json.dump(COUNTER, f)
        with open(checks, "w", encoding="utf-8") as f:
            f.write("TYPES = {}\n")
        root = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
        # A file, not a pipe: a glue left behind would hold a pipe open.
        err = tempfile.TemporaryFile("w+", encoding="utf-8")
        self.addCleanup(err.close)
        proc = subprocess.Popen([sys.executable, "-E", "-s", "-B", "-m", "csdfbmc", path, checks, "--progress", "0",
                                 *(a.replace("MARKER", marker) for a in args)],
                                cwd=root, stdout=subprocess.PIPE, stderr=err, text=True, start_new_session=True)
        self.addCleanup(self.killed, proc)
        deadline = time.monotonic() + 60
        while not os.path.exists(marker):
            if proc.poll() is not None or time.monotonic() > deadline:
                self.fail(f"csdfbmc never started a step: {self.killed(proc)}")
            time.sleep(0.05)
        proc.err, proc.marker = err, marker
        self.addCleanup(self.left, proc, signal.SIGKILL)
        return proc

    @staticmethod
    def left(proc, kill=0):
        """The processes that said they were in a step (slow_glue) and are
        still there, sent kill (0 sends nothing)."""
        with open(proc.marker, encoding="utf-8") as f:
            pids = {int(line) for line in f}
        alive = set()
        for pid in pids:
            try:
                os.kill(pid, kill)
                alive.add(pid)
            except ProcessLookupError:
                pass
        return alive

    @staticmethod
    def killed(proc):
        """proc and what is left of its group killed, and waited for."""
        try:
            os.killpg(proc.pid, signal.SIGKILL)
        except (ProcessLookupError, PermissionError):
            pass  # the group is gone, or holds only zombies
        proc.kill()
        return proc.communicate()[0], SignalledCommandTest.said(proc)

    @staticmethod
    def said(proc):
        proc.err.seek(0)
        return proc.err.read()

    def test_stops_within_seconds_with_what_it_found(self):
        import subprocess
        import time
        import shlex
        import sys
        module = ("--evaluator-module", "csdfbmc.tests.slow_glue MARKER")
        command = ("--evaluator", shlex.join([sys.executable, "-E", "-s", "-B", "-m", "csdfbmc.tests.slow_glue", "MARKER"]))
        tests = {
            "random walks, SIGTERM": (signal.SIGTERM, *module, "--random-walk", "--seed", "1", "--walks", "4"),
            "random walks, SIGINT": (signal.SIGINT, *module, "--random-walk", "--seed", "1", "--walks", "4"),
            "random walks, two jobs": (signal.SIGTERM, *module, "--random-walk", "--seed", "1", "--walks", "4", "--jobs", "2"),
            "breadth first": (signal.SIGTERM, *module),
            "breadth first, two jobs": (signal.SIGTERM, *module, "--jobs", "2"),
            "a glue command, random walks": (signal.SIGTERM, *command, "--random-walk", "--seed", "1", "--walks", "4"),
            "a glue command, two jobs": (signal.SIGTERM, *command, "--jobs", "2"),
        }
        for name, (sent, *args) in tests.items():
            with self.subTest(name):
                # Setup
                proc = self.started(*args)

                # Execute
                os.killpg(proc.pid, sent)
                started = time.monotonic()
                try:
                    out, err = proc.communicate(timeout=30)[0], self.said(proc)
                except subprocess.TimeoutExpired:
                    out, err = self.killed(proc)

                # Assert
                self.assertLess(time.monotonic() - started, 10, err)
                self.assertEqual(0, proc.returncode, err)
                self.assertIn(f" (stopped by {sent.name})\n", out.splitlines(True)[0])
                deadline = time.monotonic() + 5
                while self.left(proc) and time.monotonic() < deadline:
                    time.sleep(0.05)
                self.assertEqual(set(), self.left(proc), "left in the middle of a step")


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
