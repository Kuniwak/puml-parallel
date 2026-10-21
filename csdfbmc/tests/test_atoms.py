"""An atom's environment function: its name is its text without its holes,
its arguments are its holes (docs/CLAUSE_DESIGN.md, docs/SYNTAX.md)."""
import unittest

from csdfbmc.atoms import function_of


class FunctionOfTest(unittest.TestCase):
    def test_reads_the_name_and_the_arguments(self):
        # Setup
        cases = [
            ("a $name hole and the space that ends it", "$在庫 が 十分", "$_ が 十分", ("在庫",)),
            ("two holes, each argument once, in order", "$a と $b と $a が 等しい", "$_ と $_ と $_ が 等しい", ("a", "b")),
            ("a hole before text that cannot continue a name", "$n>0", "$_>0", ("n",)),
            ("a ${name} hole is the same hole", "${在庫} が 十分", "$_ が 十分", ("在庫",)),
            ("$ before a digit is text", "$100 を 超える $額", "$100 を 超える $_", ("額",)),
            ("an escaped $ is text", r"\$x と $y", "\\$x と $_", ("y",)),
            ("no hole", "晴れている", "晴れている", ()),
        ]
        for name, text, want_name, want_args in cases:
            with self.subTest(name):
                # Execute
                f = function_of(text)

                # Assert
                self.assertEqual((want_name, want_args), (f.name, f.args))

    def test_the_same_text_with_other_hole_names_is_the_same_function(self):
        # Execute
        a, b = function_of("$在庫 が 十分"), function_of("$残り が 十分")

        # Assert
        self.assertEqual(a.name, b.name)

    def test_refuses_what_a_guard_cannot_read_yet(self):
        # Setup
        cases = [
            ("a value after the step", "$在庫' が 十分", "after the step"),
            ("a term with a path", "${locations[from].在庫} が 十分", "a name"),
            ("an unclosed brace", "${在庫 が 十分", "closing"),
        ]
        for name, text, message in cases:
            with self.subTest(name):
                # Execute, Assert
                with self.assertRaisesRegex(ValueError, message):
                    function_of(text)


if __name__ == "__main__":
    unittest.main()
