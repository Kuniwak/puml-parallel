"""csdfbmc --checkpoint and --resume: a walk written out as it goes, and
taken up again where it was."""
import json
import os
import tempfile
import unittest

from csdfbmc import checkpoint
from csdfbmc.tests import fixture
from csdfbmc.tests.fixture import example
from csdfbmc.tests.test_stats import CHECKS, FORGETFUL


def walk(test, *args, diagram=FORGETFUL):
    from csdfbmc.__main__ import main
    return fixture.run_main(test, main, diagram, "--evaluator-module", "csdfbmc.python_glue", *args, checks=CHECKS)


class ResumeTest(unittest.TestCase):
    def test_a_walk_resumed_after_any_state_ends_as_the_whole_walk(self):
        for name, options in (("printer_retry", {}), ("attempts", {"por": True, "symmetry": True, "avoid": "DONE"})):
            with self.subTest(name=name):
                # Setup
                whole = example(name, **options)
                expected = whole.run().to_json()
                saved = []
                saving = example(name, **options)
                saving.run(checkpoint=lambda w: saved.append(checkpoint.dumps(w, saving.d.edges)))

                for k in (0, len(saved) // 2, len(saved) - 1):
                    # Execute
                    again = example(name, **options)
                    actual = again.run(resume=checkpoint.loads(saved[k], again.d.edges)).to_json()

                    # Assert
                    self.assertEqual(expected, actual, k)


class CheckpointOptionTest(unittest.TestCase):
    def setUp(self):
        d = tempfile.TemporaryDirectory()
        self.addCleanup(d.cleanup)
        self.path = os.path.join(d.name, "walk.ckpt")

    def test_a_walk_resumed_from_its_checkpoint_says_what_it_said(self):
        # Setup
        _, expected, _ = walk(self, "--json", "--stats", "--checkpoint", self.path)

        # Execute
        code, actual, err = walk(self, "--json", "--stats", "--resume", self.path)

        # Assert
        self.assertEqual(1, code, err)
        self.assertEqual(json.loads(expected), json.loads(actual))

    def test_a_checkpoint_of_other_inputs_is_refused(self):
        # Setup
        walk(self, "--checkpoint", self.path)
        other = {**FORGETFUL, "name": "Other"}

        # Execute
        code, _, err = walk(self, "--resume", self.path, diagram=other)

        # Assert
        self.assertEqual(2, code)
        self.assertIn("was written for other inputs", err)
