import os
import tempfile
import textwrap
import unittest

from csdfbmc.twins import Twins, load_twins, PurityError


class LoadTwinsTest(unittest.TestCase):
    def write(self, source):
        d = tempfile.mkdtemp()
        path = os.path.join(d, "twins.py")
        with open(path, "w", encoding="utf-8") as f:
            f.write(textwrap.dedent(source))
        return path

    def test_loads_the_domains_ghosts_and_invariants_of_a_module(self):
        # Setup
        path = self.write('''
            UNIT = 2
            DOMAINS = {"n": range(0, 4), "額": [2, 4]}
            INVARIANTS = {"額 は最小単位の倍数": lambda 額: 額 % UNIT == 0}
        ''')

        # Execute
        checks = load_twins(path)

        # Assert
        self.assertIsInstance(checks, Twins)
        self.assertEqual([2, 4], list(checks.domains["額"]))
        self.assertTrue(checks.invariants["額 は最小単位の倍数"](額=4))

    def test_refuses_a_module_that_is_not_pure(self):
        for name, body in {
            "assignment inside a function": "def f(n):\n    m = n\n    return m\n",
            "a loop": "def f(n):\n    for i in range(n):\n        pass\n    return True\n",
            "a float": "X = 0.5\n",
            "input or output": "print(1)\n",
            "randomness": "import random\n",
        }.items():
            with self.subTest(name):
                with self.assertRaises(PurityError):
                    load_twins(self.write("DOMAINS = {}\n" + body))

    def test_refuses_a_module_without_domains(self):
        with self.assertRaises(PurityError):
            load_twins(self.write("INVARIANTS = {}\n"))

    def test_refuses_the_twins_of_prose_clauses(self):
        # Setup: CLAUSES were the twins of an earlier csdfbmc; the clauses are
        # read through a glue now.
        path = self.write("DOMAINS = {}\nCLAUSES = {}\n")

        # Execute, Assert
        with self.assertRaisesRegex(PurityError, "glue"):
            load_twins(path)


if __name__ == "__main__":
    unittest.main()
