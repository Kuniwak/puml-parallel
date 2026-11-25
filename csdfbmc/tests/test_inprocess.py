"""csdfbmc's commands run in this interpreter, as a host that starts no
process (a web page running Python in WebAssembly) runs them."""
import json
import os
import tempfile
import unittest

from csdfbmc.inprocess import Sessions, run
from csdfbmc.next import explorer_of
from csdfbmc.tests.test_explore import COUNTER


class RunTest(unittest.TestCase):
    def setUp(self):
        d = tempfile.TemporaryDirectory()
        self.addCleanup(d.cleanup)
        self.diagram, self.checks = os.path.join(d.name, "d.json"), os.path.join(d.name, "t.py")
        self.write(self.diagram, json.dumps(COUNTER))
        self.write(self.checks, "TYPES = {}\n")
        self.opened = []

        def opener(a):
            self.opened.append(a.diagram)
            return explorer_of(a)
        self.sessions = Sessions(opener)
        self.addCleanup(self.sessions.close)

    @staticmethod
    def write(path, text):
        with open(path, "w", encoding="utf-8") as f:
            f.write(text)

    def initial(self, *extra):
        return ["next", self.diagram, self.checks, "--evaluator-module", "csdfbmc.python_glue", "--initial", *extra]

    def test_next_answers_what_the_command_prints(self):
        # Execute
        code, out, err = run(self.initial(), self.sessions)

        # Assert
        self.assertEqual((0, {"candidates": [{"params": {}, "values": [2]}]}, ""), (code, json.loads(out), err))

    def test_inputs_it_cannot_read_are_said_on_the_standard_error(self):
        # Execute
        code, out, err = run(["next", "no-such.json", "no-such.py", "--evaluator-module", "csdfbmc.python_glue", "--initial"], self.sessions)

        # Assert
        self.assertEqual((2, ""), (code, out))
        self.assertIn("csdfbmc next:", err)

    def test_a_command_it_does_not_know_is_refused(self):
        # Execute
        code, out, err = run(["walk"], self.sessions)

        # Assert
        self.assertEqual((2, ""), (code, out))
        self.assertIn("next", err)

    def test_the_inputs_are_opened_once_for_the_calls_that_name_them(self):
        # Execute
        answers = [run(self.initial(), self.sessions) for _ in range(2)]

        # Assert
        self.assertEqual(([[0, '{"candidates": [{"params": {}, "values": [2]}]}\n', ""]] * 2, 1), (answers, len(self.opened)))

    def test_inputs_named_otherwise_are_opened_again(self):
        # Setup: --env is no input the checks module declares, so the
        # second call fails, but only after opening its inputs.
        run(self.initial(), self.sessions)

        # Execute
        run(self.initial("--env", "x=true"), self.sessions)

        # Assert
        self.assertEqual(2, len(self.opened))

    def test_inputs_written_again_are_opened_again(self):
        # Setup
        run(self.initial(), self.sessions)
        self.write(self.diagram, json.dumps(COUNTER).replace("== 2", "== 3"))

        # Execute
        code, out, _ = run(self.initial(), self.sessions)

        # Assert
        self.assertEqual((0, [3], 2), (code, json.loads(out)["candidates"][0]["values"], len(self.opened)))

    def test_inputs_closed_are_opened_again(self):
        # Setup
        run(self.initial(), self.sessions)
        self.sessions.close()

        # Execute
        run(self.initial(), self.sessions)

        # Assert
        self.assertEqual(2, len(self.opened))


if __name__ == "__main__":
    unittest.main()
