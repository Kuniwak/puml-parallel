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


class SessionsTest(unittest.TestCase):
    def write(self, path, text):
        with open(path, "w", encoding="utf-8") as f:
            f.write(text)

    def counting(self):
        """Sessions whose opener counts how often it opens an Explorer."""
        from csdfbmc.inprocess import Sessions
        from csdfbmc.next import explorer_of
        opened = []

        def opener(a):
            opened.append(a.diagram)
            return explorer_of(a)
        return Sessions(opener), opened

    def test_the_inputs_are_opened_once_for_the_calls_that_name_them(self):
        # Setup
        from csdfbmc.inprocess import run
        d = tempfile.TemporaryDirectory()
        self.addCleanup(d.cleanup)
        diagram, checks = os.path.join(d.name, "d.json"), os.path.join(d.name, "t.py")
        self.write(diagram, json.dumps(COUNTER))
        self.write(checks, "TYPES = {}\n")
        sessions, opened = self.counting()
        self.addCleanup(sessions.close)
        args = ["next", diagram, checks, "--evaluator-module", "csdfbmc.python_glue", "--initial"]

        # Execute
        answers = [run(args, sessions) for _ in range(2)]

        # Assert
        self.assertEqual(([[0, '{"candidates": [{"params": {}, "values": [2]}]}\n', ""]] * 2, 1), (answers, len(opened)))

    def test_inputs_written_again_are_opened_again(self):
        # Setup
        from csdfbmc.inprocess import run
        d = tempfile.TemporaryDirectory()
        self.addCleanup(d.cleanup)
        diagram, checks = os.path.join(d.name, "d.json"), os.path.join(d.name, "t.py")
        self.write(diagram, json.dumps(COUNTER))
        self.write(checks, "TYPES = {}\n")
        sessions, opened = self.counting()
        self.addCleanup(sessions.close)
        args = ["next", diagram, checks, "--evaluator-module", "csdfbmc.python_glue", "--initial"]
        run(args, sessions)
        self.write(diagram, json.dumps(COUNTER).replace("== 2", "== 3"))

        # Execute
        code, out, _ = run(args, sessions)

        # Assert
        self.assertEqual((0, [3], 2), (code, json.loads(out)["candidates"][0]["values"], len(opened)))


if __name__ == "__main__":
    unittest.main()
