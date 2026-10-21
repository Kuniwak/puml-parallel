"""An atom's environment function: its name marks each hole with its
argument's position, its arguments are the distinct terms its holes read
(docs/CLAUSE_DESIGN.md, docs/SYNTAX.md). The cases are shared with the Go
side in csdf/clause/testdata/function_names.json."""
import json
import unittest
from pathlib import Path

from csdfbmc.atoms import function_of

CASES = json.loads((Path(__file__).resolve().parents[2] / "csdf" / "clause" / "testdata" / "function_names.json").read_text(encoding="utf-8"))


class FunctionOfTest(unittest.TestCase):
    def test_reads_the_name_and_the_arguments(self):
        for case in CASES["named"]:
            with self.subTest(case["case"]):
                # Setup
                want = (case["name"], tuple(case["args"]))

                # Execute
                f = function_of(case["text"])

                # Assert
                self.assertEqual(want, (f.name, f.args))

    def test_the_same_text_with_other_hole_names_is_the_same_function(self):
        # Execute
        a, b = function_of("$在庫 が 十分"), function_of("$残り が 十分")

        # Assert
        self.assertEqual(a.name, b.name)

    def test_refuses_what_a_guard_cannot_read_yet(self):
        for case in CASES["refused"]:
            with self.subTest(case["case"]):
                # Execute, Assert
                with self.assertRaisesRegex(ValueError, case["message"]):
                    function_of(case["text"])


if __name__ == "__main__":
    unittest.main()
