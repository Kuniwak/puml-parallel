"""csdfbmc.serve runs a glue a module's evaluator(args) gives as a command,
so that a glue written as a module answers over standard input and output."""
import io
import json
import unittest

from csdfbmc.serve import main


class ServeTest(unittest.TestCase):
    def test_a_module_glue_answers_over_lines(self):
        # Setup
        lines = [{"op": "hello", "protocol": 3},
                 {"op": "lint", "atom": {"text": ["", " > LIMIT"], "holes": [0], "primed": [False]}, "role": "guard"}]
        stdin = io.StringIO("".join(json.dumps(r) + "\n" for r in lines))
        stdout = io.StringIO()

        # Execute
        main(["csdfbmc.python_glue"], stdin, stdout)

        # Assert
        answers = [json.loads(line) for line in stdout.getvalue().splitlines()]
        self.assertEqual("python", answers[0]["language"])
        self.assertFalse(answers[1]["ok"])
        self.assertIn("LIMIT", answers[1]["why"])

    def test_no_module_is_a_usage_error(self):
        # Execute, Assert
        with self.assertRaises(SystemExit):
            main([], io.StringIO(), io.StringIO())


if __name__ == "__main__":
    unittest.main()
