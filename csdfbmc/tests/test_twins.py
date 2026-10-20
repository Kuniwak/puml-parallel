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

    def test_loads_domains_and_clauses_from_a_module(self):
        twins = load_twins(self.write('''
            UNIT = 2
            DOMAINS = {"n": range(0, 4), "額": [2, 4]}
            CLAUSES = {
                "額 は最小単位の倍数": lambda 額: 額 % UNIT == 0,
            }
        '''))
        self.assertIsInstance(twins, Twins)
        self.assertEqual([2, 4], list(twins.domains["額"]))
        self.assertTrue(twins.clauses["額 は最小単位の倍数"](額=4))

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
                    load_twins(self.write("DOMAINS = {}\nCLAUSES = {}\n" + body))

    def test_refuses_a_module_without_the_two_tables(self):
        with self.assertRaises(PurityError):
            load_twins(self.write("CLAUSES = {}\n"))


if __name__ == "__main__":
    unittest.main()
