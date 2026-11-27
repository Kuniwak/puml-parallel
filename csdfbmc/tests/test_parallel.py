"""csdfbmc --jobs: the walk with its states expanded in worker processes."""
import io
import json
import os
import unittest
from contextlib import redirect_stderr, redirect_stdout

from csdfbmc.tests import fixture
from csdfbmc.tests.fixture import atom, diagram, edge

ROOT = os.path.join(os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__)))), "examples", "bmc")


def walk(name, *args):
    from csdfbmc.__main__ import main
    path, checks = (os.path.join(ROOT, p) for p in fixture.EXAMPLES[name])
    out, err = io.StringIO(), io.StringIO()
    with redirect_stdout(out), redirect_stderr(err):
        code = main([path, checks, "--evaluator-module", f"csdfbmc.python_glue {checks}", "--json", *args])
    return code, out.getvalue(), err.getvalue()


class SameWalkTest(unittest.TestCase):
    def test_the_examples_walk_the_same_with_workers(self):
        tests = {
            "printer": (),
            "printer_retry": ("--avoid", "完了", "--within", "printing"),
            "attempts": ("--avoid", "DONE"),
            "attempts reduced": ("--symmetry", "--por", "--cone"),
            "attempts at the first finding": ("--first",),
            "attempts truncated": ("--max-states", "50"),
        }
        for name, args in tests.items():
            with self.subTest(name):
                example = name.split()[0]

                # Execute
                alone = walk(example, *args)
                workers = walk(example, "--jobs", "2", *args)

                # Assert
                self.assertEqual(alone[2], "")
                self.assertEqual(alone[0], workers[0], workers[2])
                self.assertEqual(json.loads(alone[1]), json.loads(workers[1]))

    def test_what_a_worker_cannot_expand_is_reported_as_without_workers(self):
        # Setup: a step to b leaves q to range over a type TYPES lacks.
        from csdfbmc.__main__ import main
        d = diagram({"a": ["n"], "b": ["q"]}, "a", init_post=[atom("$n' == 0")], edges=[edge("a", "b", "go")])

        # Execute
        alone = fixture.run_main(self, main, d, "--evaluator-module", "csdfbmc.python_glue")
        workers = fixture.run_main(self, main, d, "--evaluator-module", "csdfbmc.python_glue", "--jobs", "2")

        # Assert
        self.assertEqual(2, alone[0])
        self.assertIn("no type 'q' in TYPES", alone[2])
        self.assertEqual(alone, workers)

    def test_fewer_than_one_job_is_a_usage_error(self):
        from csdfbmc.__main__ import main
        err = io.StringIO()

        # Execute
        with self.assertRaises(SystemExit) as raised, redirect_stderr(err):
            main(["d.json", "t.py", "--jobs", "0"])

        # Assert
        self.assertEqual(2, raised.exception.code)
        self.assertIn("--jobs expects a positive number", err.getvalue())


class StreamTest(unittest.TestCase):
    def test_the_walk_asks_once_for_its_queue_as_it_grows(self):
        # Setup
        walk_ = fixture.example("attempts")
        expected = fixture.example("attempts").run().to_json()
        calls = []

        def expand_all(queue, start):
            calls.append(start)
            j = start
            while j < len(queue):
                yield walk_.expand(queue[j])
                j += 1

        # Execute
        actual = walk_.run(expand_all).to_json()

        # Assert
        self.assertEqual(expected, actual)
        self.assertEqual([0], calls)


# A worker that the pool's SIGTERM finds in its own exit: atexit cleanup
# swallows what a handler raises there, and the pool would wait for it.
_TERMINATED_IN_EXIT = """
import atexit, os, signal, time
from csdfbmc import parallel
signal.signal(signal.SIGTERM, parallel._ended)
atexit.register(lambda: (os.kill(os.getpid(), signal.SIGTERM), time.sleep(5)))
"""


class EndedTest(unittest.TestCase):
    def test_a_worker_ends_on_sigterm_even_in_its_own_exit(self):
        # Setup
        import subprocess
        import sys
        repo = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))

        # Execute
        done = subprocess.run([sys.executable, "-E", "-s", "-B", "-c", _TERMINATED_IN_EXIT], cwd=repo,
                              capture_output=True, text=True, timeout=30)

        # Assert
        self.assertEqual(128 + 15, done.returncode, done.stderr)
        self.assertNotIn("Exception ignored", done.stderr)
