"""csdfbmc's commands run in this interpreter, as a host that starts no
process (a web page running Python in WebAssembly) runs them."""
import json
import os
import tempfile
import unittest

from csdfbmc.tests.test_explore import COUNTER


class RunTest(unittest.TestCase):
    def write(self, name, text):
        d = tempfile.TemporaryDirectory()
        self.addCleanup(d.cleanup)
        path = os.path.join(d.name, name)
        with open(path, "w", encoding="utf-8") as f:
            f.write(text)
        return path

    def test_next_answers_what_the_command_prints(self):
        # Setup
        from csdfbmc.inprocess import run
        diagram, checks = self.write("d.json", json.dumps(COUNTER)), self.write("t.py", "TYPES = {}\n")

        # Execute
        code, out, err = run(["next", diagram, checks, "--evaluator-module", "csdfbmc.python_glue", "--initial"])

        # Assert
        self.assertEqual((0, {"candidates": [{"params": {}, "values": [2]}]}, ""), (code, json.loads(out), err))

    def test_inputs_it_cannot_read_are_said_on_the_standard_error(self):
        # Setup
        from csdfbmc.inprocess import run

        # Execute
        code, out, err = run(["next", "no-such.json", "no-such.py", "--evaluator-module", "csdfbmc.python_glue", "--initial"])

        # Assert
        self.assertEqual((2, ""), (code, out))
        self.assertIn("csdfbmc next:", err)

    def test_a_command_it_does_not_know_is_refused(self):
        # Setup
        from csdfbmc.inprocess import run

        # Execute
        code, out, err = run(["walk"])

        # Assert
        self.assertEqual((2, ""), (code, out))
        self.assertIn("next", err)


if __name__ == "__main__":
    unittest.main()
