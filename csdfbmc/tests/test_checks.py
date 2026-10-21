import os
import tempfile
import textwrap
import unittest

from csdfbmc.checks import Checks, load_checks, PurityError


class TypesTest(unittest.TestCase):
    def test_a_name_ranges_over_the_type_typeof_gives_it(self):
        # Setup
        checks = Checks(types={"口座ID": ["a1", "a2"]}, typeof={"相手ID": "口座ID"})

        # Execute
        values = checks.domain("相手ID")

        # Assert
        self.assertEqual(["a1", "a2"], values)
        self.assertTrue(checks.declares("相手ID"))

    def test_a_name_typeof_does_not_name_takes_the_type_of_its_own_name(self):
        # Setup
        checks = Checks(types={"額": [1, 2]}, typeof={"相手ID": "口座ID"})

        # Execute
        values = checks.domain("額")

        # Assert
        self.assertEqual([1, 2], values)
        self.assertFalse(checks.declares("相手ID"))


class LoadChecksTest(unittest.TestCase):
    def write(self, source):
        d = tempfile.mkdtemp()
        path = os.path.join(d, "checks.py")
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
        checks = load_checks(path)

        # Assert
        self.assertIsInstance(checks, Checks)
        self.assertEqual([2, 4], list(checks.types["額"]))
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
                    load_checks(self.write("DOMAINS = {}\n" + body))

    def test_refuses_a_module_without_domains(self):
        with self.assertRaises(PurityError):
            load_checks(self.write("INVARIANTS = {}\n"))

    def test_refuses_the_clauses_an_earlier_csdfbmc_read(self):
        # Setup: the clauses are read through a glue now, so CLAUSES would
        # otherwise be silently ignored.
        path = self.write("DOMAINS = {}\nCLAUSES = {}\n")

        # Execute, Assert
        with self.assertRaisesRegex(PurityError, "glue"):
            load_checks(path)


if __name__ == "__main__":
    unittest.main()
